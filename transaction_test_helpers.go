package testutil

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

// Note: We use the pluralizer instance declared in db_test_context.go
// This package provides enhanced transaction support for GORM with proper table resolution

// TransactionTestCase represents a test case for transaction testing
type TransactionTestCase struct {
	Name           string
	SetupMock      func(*DBTestContext)
	ExecuteService func(core.Context) error
	ExpectError    bool
	ErrorContains  string
	Pagination     queryutil.Pagination // Optional pagination for the test case
}

// TransactionTestHelper provides utilities for testing transactions with GORM.
// It includes specialized functionality for resolving table names in transactions
// and handling GORM callbacks without generating warning messages.
//
// Each TransactionTestHelper instance gets a unique session ID to prevent
// callback name conflicts when using multiple helpers, which eliminates
// the "duplicated callback" warnings that would otherwise appear.
type TransactionTestHelper struct {
	tc                  *DBTestContext  // The test context this helper works with
	registeredCallbacks map[string]bool // Tracks which callbacks have been registered and their active state
	sessionID           string          // Unique ID for this transaction helper instance
}

// NewTransactionTestHelper creates a new transaction test helper with unique session ID.
//
// The helper is assigned a unique session ID based on a timestamp, which ensures
// that multiple helpers can be used in the same test without causing callback naming
// conflicts in GORM. The helper also tracks which callbacks it has registered,
// allowing it to manage them properly without generating warning messages.
//
// When working with complex models that have both relationships AND lifecycle hooks,
// you should register them using RegisterModelWithRelationships to prevent "Table not set" errors:
//
//	// Register models to ensure proper table resolution
//	RegisterModelWithRelationships[MyComplexModel](testCtx)
//
// Example:
//
//	// Create a transaction helper
//	txHelper := NewTransactionTestHelper(testCtx)
//
//	// Use it to execute transaction with proper table name resolution
//	txHelper.ExecuteInTransaction(func(tx *gorm.DB) error {
//	    return tx.Create(&MyModel{}).Error // Table name is properly resolved
//	})
func NewTransactionTestHelper(tc *DBTestContext) *TransactionTestHelper {
	// Generate a unique session ID for this instance using a timestamp
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())

	return &TransactionTestHelper{
		tc:                  tc,
		registeredCallbacks: make(map[string]bool),
		sessionID:           sessionID,
	}
}

// ExecuteInTransaction executes a function within a transaction and automatically handles
// commit or rollback based on the result. It also ensures proper table resolution for
// registered models, automatically resolving table names for models with custom TableName()
// methods used within the transaction.
//
// This function fixes the common "Table not set" error in GORM when using models with
// TableName() methods inside transactions by automatically registering appropriate
// callbacks to resolve table names. It works with:
//
// - Both value and pointer receiver TableName() methods
// - Simple models without relationships
// - Complex models with relationships
// - Models that have been transformed internally by GORM
//
// The enhanced table name resolution ensures that even in complex scenarios where
// GORM might internally transform a model (such as with relationship loading), the
// correct table name is still resolved.
//
// For complex models with BOTH relationships AND lifecycle hooks (BeforeCreate, BeforeUpdate, etc.),
// you should use RegisterModelWithRelationships before using this function:
//
//	// Register models to prevent "Table not set" errors
//	RegisterModelWithRelationships[MyComplexModel](testCtx)
//
// "Table not set" errors typically occur when:
// 1. A model has relationship fields (struct types or slices of structs)
// 2. The model also has lifecycle hooks that call other methods
// 3. The model has a more complex structure overall
//
// Example:
//
//	testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
//	    // Creates a simple model with correct table name resolution
//	    err := tx.Create(&SimpleModel{Name: "test"}).Error
//	    if err != nil {
//	        return err
//	    }
//
//	    // Also works with complex models that have relationships
//	    return tx.Create(&ComplexModel{
//	        RelatedID: 1,
//	        Name: "test",
//	    }).Error
//	})
//
// createWithCorrectTable is a helper function that ensures the table is correctly set
// for Create operations - this is our automatic fix for the "Table not set" bug
func (th *TransactionTestHelper) createWithCorrectTable(tx *gorm.DB, value interface{}) *gorm.DB {
	var tableName string

	// Strategy 1: Try to get tableName from the model with direct interface call
	if tabler, ok := value.(interface{ TableName() string }); ok {
		tableName = tabler.TableName()
		return tx.Table(tableName).Create(value)
	}

	// Strategy 2: Try reflection for pointer receiver methods
	modelValue := reflect.ValueOf(value)
	if modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
		if method := modelValue.MethodByName("TableName"); method.IsValid() {
			results := method.Call(nil)
			if len(results) > 0 && results[0].Kind() == reflect.String {
				tableName = results[0].String()
				return tx.Table(tableName).Create(value)
			}
		}
	}

	// Strategy 3: Look in registered models
	modelType := reflect.TypeOf(value)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Thread safety when accessing registered models
	th.tc.mu.Lock()
	defer th.tc.mu.Unlock()

	// Use snake case for name matching
	modelName := toSnakeCase(modelType.Name())
	pluralName := pluralizer.Plural(modelName)

	// First check pluralized name (GORM convention)
	if _, ok := th.tc.registeredModels[pluralName]; ok {
		return tx.Table(pluralName).Create(value)
	}

	// Then check singular name
	if _, ok := th.tc.registeredModels[modelName]; ok {
		return tx.Table(modelName).Create(value)
	}

	// If only one model is registered, it's likely what we need
	if len(th.tc.registeredModels) == 1 {
		for tableName := range th.tc.registeredModels {
			return tx.Table(tableName).Create(value)
		}
	}

	// Last resort: use pluralized name
	if len(modelName) > 0 {
		return tx.Table(pluralName).Create(value)
	}

	// If we get here, proceed with normal Create
	return tx.Create(value)
}

// addCreateInterceptor adds a callback specifically to intercept Create operations
// This provides table name resolution for all create operations in transactions
func addCreateInterceptor(db *gorm.DB, th *TransactionTestHelper) {
	// Register a specific callback for Create that happens right before the operation
	callbackName := fmt.Sprintf("testutil:create_intercept:%s", th.sessionID)

	// Register callback before the GORM create operation
	db.Callback().Create().Before("gorm:create").Register(callbackName, func(db *gorm.DB) {
		// Only proceed if the table isn't set yet
		if db.Statement.Table != "" {
			return
		}

		// If a value is being created, try to get its table name
		if db.Statement.ReflectValue.IsValid() {
			value := db.Statement.ReflectValue.Interface()

			// Strategy 1: Try direct interface method call
			if tabler, ok := value.(interface{ TableName() string }); ok {
				tableName := tabler.TableName()
				db.Statement.Table = tableName
				return
			}

			// Strategy 2: Try reflection for pointer receiver methods
			modelValue := reflect.ValueOf(value)
			if modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
				if method := modelValue.MethodByName("TableName"); method.IsValid() {
					results := method.Call(nil)
					if len(results) > 0 && results[0].Kind() == reflect.String {
						db.Statement.Table = results[0].String()
						return
					}
				}
			}

			// Strategy 3: Look in registered models by type name
			modelType := reflect.TypeOf(value)
			if modelType.Kind() == reflect.Ptr {
				modelType = modelType.Elem()
			}

			// Create snake case name for lookup
			modelName := toSnakeCase(modelType.Name())

			// Use mutex for thread safety when accessing registered models
			th.tc.mu.Lock()
			defer th.tc.mu.Unlock()

			// Try pluralized version first (GORM convention)
			pluralName := pluralizer.Plural(modelName)
			if _, ok := th.tc.registeredModels[pluralName]; ok {
				db.Statement.Table = pluralName
				return
			}

			// Then try exact match
			if _, ok := th.tc.registeredModels[modelName]; ok {
				db.Statement.Table = modelName
				return
			}

			// Last resort: If only one model is registered, use that table
			if len(th.tc.registeredModels) == 1 {
				for tableName := range th.tc.registeredModels {
					db.Statement.Table = tableName
					return
				}
			}

			// Final fallback: use pluralized snake case name
			db.Statement.Table = pluralName
		}
	})

	th.registeredCallbacks[callbackName] = true
}

// SafeTransactionDB ensures proper table resolution in transactions
// This automatic wrapper handles the "Table not set" bug that occurs with
// cross-package relationships in GORM
type SafeTransactionDB struct {
	*gorm.DB
	th *TransactionTestHelper
}

// Create intercepts Create operations to ensure table is set
func (s *SafeTransactionDB) Create(value interface{}) *gorm.DB {
	// Try to get tableName from model or value
	var tableName string

	// Strategy 1: Try direct interface method call
	if tabler, ok := value.(interface{ TableName() string }); ok {
		tableName = tabler.TableName()
	} else {
		// Strategy 2: Try reflection for pointer receiver methods
		modelValue := reflect.ValueOf(value)
		if modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
			if method := modelValue.MethodByName("TableName"); method.IsValid() {
				results := method.Call(nil)
				if len(results) > 0 && results[0].Kind() == reflect.String {
					tableName = results[0].String()
				}
			}
		}

		// Strategy 3: If still no table name, try registered models
		if tableName == "" {
			// Get the model type for lookup
			modelType := reflect.TypeOf(value)
			if modelType.Kind() == reflect.Ptr {
				modelType = modelType.Elem()
			}

			// Use mutex for thread safety
			s.th.tc.mu.Lock()
			defer s.th.tc.mu.Unlock()

			// Try to find by type name first
			modelName := toSnakeCase(modelType.Name())
			pluralName := pluralizer.Plural(modelName)

			// First check pluralized name (GORM convention)
			if _, ok := s.th.tc.registeredModels[pluralName]; ok {
				tableName = pluralName
			} else if _, ok := s.th.tc.registeredModels[modelName]; ok {
				// Then check exact match
				tableName = modelName
			} else if len(s.th.tc.registeredModels) == 1 {
				// If only one model is registered, it's likely what we want
				for name := range s.th.tc.registeredModels {
					tableName = name
					break
				}
			} else {
				// Last resort: use pluralized snake case
				tableName = pluralName
			}
		}
	}

	// If we found a table name, explicitly set it
	if tableName != "" {
		return s.DB.Table(tableName).Create(value)
	}

	// Otherwise, use default behavior
	return s.DB.Create(value)
}

func (th *TransactionTestHelper) ExecuteInTransaction(fn func(*gorm.DB) error) error {
	// Start by expecting a transaction
	th.tc.mock.ExpectBegin()

	// Get transaction from the DB
	tx := th.tc.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Create a transaction wrapper that correctly handles model table resolution
	txWrapper := th.wrapTransactionWithTableInfo(tx)

	// Register the table resolver callback to fix "Table not set" errors
	// This callback runs before the main GORM create operation to ensure
	// the table name is properly set, preventing the "Table not set" error
	tableResolverCallback := fmt.Sprintf("testutil:table_resolver:%s", th.sessionID)
	txWrapper.Callback().Create().Before("gorm:create").Register(tableResolverCallback, func(db *gorm.DB) {
		// Only proceed if the table isn't set yet
		if db.Statement.Table == "" && db.Statement.ReflectValue.IsValid() {
			value := db.Statement.ReflectValue.Interface()

			// Try direct interface method call first (most reliable)
			// This works for models that implement the TableName method
			if tabler, ok := value.(interface{ TableName() string }); ok {
				tableName := tabler.TableName()
				db.Statement.Table = tableName
				return
			}

			// Use reflection for models with pointer receivers
			// Some models implement TableName only on the pointer receiver
			modelValue := reflect.ValueOf(value)
			if modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
				if method := modelValue.MethodByName("TableName"); method.IsValid() {
					results := method.Call(nil)
					if len(results) > 0 && results[0].Kind() == reflect.String {
						db.Statement.Table = results[0].String()
						return
					}
				}

				// For models, try looking up in registered models
				modelType := modelValue.Type().Elem()

				// For registered models, find by type name
				modelName := toSnakeCase(modelType.Name())
				pluralName := pluralizer.Plural(modelName)

				// Check registered models with mutex protection
				th.tc.mu.Lock()
				defer th.tc.mu.Unlock()

				// First try with plural name
				if _, exists := th.tc.registeredModels[pluralName]; exists {
					db.Statement.Table = pluralName
					return
				}

				// Then try singular name
				if _, exists := th.tc.registeredModels[modelName]; exists {
					db.Statement.Table = modelName
					return
				}

				// If only one model is registered, it's likely what we want
				if len(th.tc.registeredModels) == 1 {
					for tableName := range th.tc.registeredModels {
						db.Statement.Table = tableName
						return
					}
				}

				// Last resort: use name-based inference
				db.Statement.Table = pluralName
			}
		}
	})
	th.registeredCallbacks[tableResolverCallback] = true

	// Execute with our table-resolving wrapper
	err := fn(txWrapper)

	// Handle commit or rollback based on error
	if err != nil {
		th.tc.mock.ExpectRollback()
		tx.Rollback()
		return err
	}

	// Expect commit and commit the transaction
	th.tc.mock.ExpectCommit()
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// wrapTransactionWithTableInfo creates a transaction wrapper that ensures proper table resolution.
// This solves the "Table not set" error that can occur in transaction operations when using
// registered models via RegisterModels() or RegisterModelWithRelationships(). The wrapper
// ensures that each registered model is properly associated with its table name within
// transaction operations.
//
// This function:
// 1. Creates a new GORM session for the transaction
// 2. Removes any existing callbacks with the same names to prevent duplication warnings
// 3. Registers callbacks for all GORM operations (Query, Create, Update, Delete, Raw)
// 4. Each callback ensures the table name is properly set before the operation executes
//
// The callbacks handle various model types including:
// - Models with TableName() methods using value receivers
// - Models with TableName() methods using pointer receivers
// - Nil pointer models
// - Slices of models
// - Direct struct values
// - Complex models with relationships
// - Models that have been transformed to map internally by GORM
//
// The enhanced table name resolution includes:
// - Looking up models by type name when direct type comparison fails
// - Handling GORM's internal transformation of models to maps
// - Extracting table name information from GORM struct tags
// - Matching models by type name for complex scenarios
//
// This fixes issues with GORM internal table resolution that occur during transactions,
// even with complex models that have relationships or when GORM performs internal
// transformations of the models.
func (th *TransactionTestHelper) wrapTransactionWithTableInfo(tx *gorm.DB) *gorm.DB {
	// No need to wrap if there are no registered models
	if len(th.tc.registeredModels) == 0 {
		return tx
	}

	// Create a new session for our wrapped transaction
	txWrapper := tx.Session(&gorm.Session{})

	// It's important to understand how GORM processes operations with models:
	// 1. The Model() call only sets up the Statement.Model field but doesn't set the table name
	// 2. The table name is actually resolved right before executing a query
	// 3. We need to hook into all operations that might execute queries

	// Clean up any callbacks we previously registered
	th.removeExistingCallbacks(txWrapper)

	// Register callbacks for all operations that might need table resolution - with unique session ID to prevent duplication
	callbackBaseNames := []string{
		"ensure_query_table",
		"ensure_create_table",
		"ensure_update_table",
		"ensure_delete_table",
		"ensure_raw_table",
	}

	// Create full callback names with session ID to ensure uniqueness
	callbackNames := make([]string, len(callbackBaseNames))
	for i, baseName := range callbackBaseNames {
		callbackNames[i] = fmt.Sprintf("testutil:%s:%s", baseName, th.sessionID)
	}

	// Register Query operation callback
	callbackName := callbackNames[0]
	txWrapper.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before a SELECT query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})
	th.registeredCallbacks[callbackName] = true

	// Register Create operation callback
	callbackName = callbackNames[1]
	txWrapper.Callback().Create().Before("gorm:create").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before an INSERT query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Add a final callback to ensure table name is set for all Create operations
	createCallbackName := fmt.Sprintf("testutil:create_table_resolver:%s", th.sessionID)
	txWrapper.Callback().Create().Before("gorm:create").Register(createCallbackName, func(db *gorm.DB) {
		// If we have a value to create but no table set, provide one last attempt
		if db.Statement.Table == "" && db.Statement.ReflectValue.IsValid() {
			// Try to find the correct table name
			value := db.Statement.ReflectValue.Interface()

			// First try to get table directly from the model
			if tabler, ok := value.(interface{ TableName() string }); ok {
				db.Statement.Table = tabler.TableName()
				return
			}

			// Otherwise use type-based resolution
			modelType := reflect.TypeOf(value)
			if modelType.Kind() == reflect.Ptr {
				modelType = modelType.Elem()
			}

			// Use thread-safe access to registered models
			th.tc.mu.Lock()
			defer th.tc.mu.Unlock()

			// Try pluralized name first (GORM convention)
			snakeCase := toSnakeCase(modelType.Name())
			pluralName := pluralizer.Plural(snakeCase)

			if _, exists := th.tc.registeredModels[pluralName]; exists {
				db.Statement.Table = pluralName
			} else if _, exists := th.tc.registeredModels[snakeCase]; exists {
				// Try exact name match
				db.Statement.Table = snakeCase
			} else {
				// If no match found in registered models, use pluralized name as fallback
				db.Statement.Table = pluralName
			}
		}
	})
	th.registeredCallbacks[callbackName] = true

	// Register Update operation callback
	callbackName = callbackNames[2]
	txWrapper.Callback().Update().Before("gorm:update").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before an UPDATE query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})
	th.registeredCallbacks[callbackName] = true

	// Register Delete operation callback
	callbackName = callbackNames[3]
	txWrapper.Callback().Delete().Before("gorm:delete").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before a DELETE query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})
	th.registeredCallbacks[callbackName] = true

	// Register Raw operation callback
	callbackName = callbackNames[4]
	txWrapper.Callback().Raw().Before("gorm:raw").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before a raw SQL query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})
	th.registeredCallbacks[callbackName] = true

	return txWrapper
}

// removeExistingCallbacks removes only the callbacks that this specific TransactionTestHelper
// instance has previously registered, using its internal tracking system.
//
// Each transaction helper tracks exactly which callbacks it has registered in its
// registeredCallbacks map. This enables precise callback management where each helper
// only removes the callbacks it personally created. The approach has several benefits:
//
//  1. Prevents "removing callback" warning messages in logs by only attempting to remove
//     callbacks that actually exist
//  2. Allows multiple transaction helpers to coexist without interfering with each other
//  3. Keeps the GORM callback registry clean by cleaning up after each transaction
//
// This is part of the solution to eliminate the warning messages that were previously
// generated when multiple transactions were used in the same test or when a single
// transaction helper was reused multiple times.
func (th *TransactionTestHelper) removeExistingCallbacks(db *gorm.DB) {
	// Only remove callbacks we've actually registered
	for name, registered := range th.registeredCallbacks {
		if !registered {
			continue
		}

		// Remove the callback from the appropriate processors
		if strings.Contains(name, "query_table") {
			db.Callback().Query().Remove(name)
		} else if strings.Contains(name, "create_table") {
			db.Callback().Create().Remove(name)
		} else if strings.Contains(name, "update_table") {
			db.Callback().Update().Remove(name)
		} else if strings.Contains(name, "delete_table") {
			db.Callback().Delete().Remove(name)
		} else if strings.Contains(name, "raw_table") {
			db.Callback().Raw().Remove(name)
		}

		// Mark as no longer registered
		th.registeredCallbacks[name] = false
	}
}

// tryGetTableName attempts to get a table name by calling a TableName() method on a model.
// It uses reflection to dynamically find and invoke the TableName() method if it exists.
//
// This function is a key part of the table resolution fix, handling:
// - Both value receiver and pointer receiver TableName() methods
// - Nil pointer models (by creating a new instance)
// - Direct struct values (by creating a pointer to the value)
// - Models with relationships (by examining model fields)
// - Models with both relationships AND lifecycle hooks
// - Complex validation logic with map operations
//
// It tries multiple approaches to obtain the table name:
// 1. First attempts to call TableName() on the pointer type (for pointer receiver methods)
// 2. Then attempts to call TableName() on the value type (for value receiver methods)
// 3. For complex models with relationships, it inspects GORM struct tags for table information
// 4. For models with lifecycle hooks, it creates a clean instance to avoid hook interference
// 5. As a fallback, it tries to extract the table name from the type name itself
//
// The enhanced implementation adds special handling for complex models with relationships
// and lifecycle hooks that might use map operations in validation methods, which can
// interfere with GORM's internal table name resolution.
//
// This comprehensive approach ensures that regardless of how the TableName() method
// is implemented (pointer or value receiver) or how complex the model structure is,
// the correct table name will be resolved.
func (th *TransactionTestHelper) tryGetTableName(model interface{}) string {
	if model == nil {
		return ""
	}

	modelValue := reflect.ValueOf(model)
	modelType := modelValue.Type()

	// Create a clean type detector
	isCleanInstance := false

	// First, check if we can infer table name from type name
	// This will be our fallback if other methods fail
	typeName := ""
	if modelType.Kind() == reflect.Ptr {
		typeName = modelType.Elem().Name()
	} else {
		typeName = modelType.Name()
	}

	// Store inferred table name for fallback
	inferredTableName := ""
	if typeName != "" {
		// Convert camel case to snake case
		var sb strings.Builder
		for i, r := range typeName {
			if i > 0 && unicode.IsUpper(r) {
				sb.WriteRune('_')
			}
			sb.WriteRune(unicode.ToLower(r))
		}
		snakeCaseName := sb.String()

		// Pluralize using proper pluralizer
		inferredTableName = pluralizer.Plural(snakeCaseName)
	}

	// Handle pointer types
	if modelValue.Kind() == reflect.Ptr {
		if modelValue.IsNil() {
			// Create a new instance for nil pointers
			modelValue = reflect.New(modelType.Elem())
			isCleanInstance = true
		}
	} else {
		// For non-pointer types, create a pointer to use for method calls
		// since TableName might be defined on the pointer receiver
		newValue := reflect.New(modelType)
		newValue.Elem().Set(modelValue)
		modelValue = newValue
	}

	// Try direct TableName method call on the pointer
	tableNameMethod := modelValue.MethodByName("TableName")
	if tableNameMethod.IsValid() {
		results := tableNameMethod.Call(nil)
		if len(results) > 0 && results[0].Kind() == reflect.String {
			return results[0].String()
		}
	}

	// If we had to create a pointer to a struct value, also try the value directly
	if modelValue.Kind() == reflect.Ptr && modelType.Kind() != reflect.Ptr {
		valueMethod := modelValue.Elem().MethodByName("TableName")
		if valueMethod.IsValid() {
			results := valueMethod.Call(nil)
			if len(results) > 0 && results[0].Kind() == reflect.String {
				return results[0].String()
			}
		}
	}

	// For complex models with relationships, try to find the table name
	// by examining the model's fields for GORM struct tag hints
	if modelValue.Kind() == reflect.Ptr && modelValue.Elem().Kind() == reflect.Struct {
		structVal := modelValue.Elem()
		structType := structVal.Type()

		// Look for GORM model embedding or table name hints in struct tags
		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)

			// Check for GORM struct tags that might have table information
			tag := field.Tag.Get("gorm")
			if strings.Contains(tag, "table:") {
				parts := strings.Split(tag, ";")
				for _, part := range parts {
					if strings.HasPrefix(part, "table:") {
						return strings.TrimPrefix(part, "table:")
					}
				}
			}
		}
	}

	// Check if the model has lifecycle hooks (BeforeCreate, BeforeUpdate, etc.)
	// If it does and we're not already working with a clean instance, create one
	// This prevents hooks from interfering with table name resolution
	if !isCleanInstance && modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
		hasLifecycleHooks := false
		hasComplexValidation := false
		hookMethods := []string{"BeforeCreate", "BeforeUpdate", "BeforeSave", "Validate"}

		for _, hookName := range hookMethods {
			method := modelValue.MethodByName(hookName)
			if method.IsValid() {
				hasLifecycleHooks = true

				// Check if it's the Validate method specifically
				if hookName == "Validate" {
					hasComplexValidation = true
				}

				// If it's a BeforeCreate or BeforeUpdate that calls Validate, note this special case
				if hookName == "BeforeCreate" || hookName == "BeforeUpdate" {
					// We can't inspect the method contents directly, but we can look for a
					// Validate method which would indicate a more complex validation pattern
					validateMethod := modelValue.MethodByName("Validate")
					if validateMethod.IsValid() {
						hasComplexValidation = true
					}
				}
			}
		}

		// Enhanced handling for models with complex validation
		if hasLifecycleHooks {
			// For models with both hooks and complex validation, the regular clean instance
			// approach sometimes fails. We'll try multiple approaches.

			// Approach 1: Create a completely new clean instance
			cleanType := modelType
			if cleanType.Kind() == reflect.Ptr {
				cleanType = cleanType.Elem()
			}
			cleanInstance := reflect.New(cleanType).Interface()

			// Try to get the table name from the clean instance
			tableName := th.tryGetTableName(cleanInstance)
			if tableName != "" {
				return tableName
			}

			// Approach 2: If we have a complex validation scenario, try to look for direct table registration
			if hasComplexValidation {
				// Lock for reading registered models
				th.tc.mu.Lock()
				defer th.tc.mu.Unlock()

				// Try to find a direct match in the registered models
				for registeredTableName, registeredModel := range th.tc.registeredModels {
					regModelType := reflect.TypeOf(registeredModel)
					if regModelType.Kind() == reflect.Ptr {
						regModelType = regModelType.Elem()
					}

					targetType := modelType
					if targetType.Kind() == reflect.Ptr {
						targetType = targetType.Elem()
					}

					// Check if the types match
					if regModelType == targetType {
						return registeredTableName
					}

					// Check if the type names match
					if regModelType.Name() == targetType.Name() {
						return registeredTableName
					}
				}

				// If we still don't have a table name, use our inferred one as a last resort
				if inferredTableName != "" {
					return inferredTableName
				}
			}
		}
	}

	// Try finding a matching type in registeredModels
	th.tc.mu.Lock()
	for registeredTableName, registeredModel := range th.tc.registeredModels {
		regModelType := reflect.TypeOf(registeredModel)
		targetType := modelType

		if regModelType.Kind() == reflect.Ptr {
			regModelType = regModelType.Elem()
		}
		if targetType.Kind() == reflect.Ptr {
			targetType = targetType.Elem()
		}

		// Try direct type match
		if regModelType == targetType {
			th.tc.mu.Unlock()
			return registeredTableName
		}

		// Try name match
		if regModelType.Name() == targetType.Name() {
			th.tc.mu.Unlock()
			return registeredTableName
		}
	}
	th.tc.mu.Unlock()

	// As a last resort, return the inferred table name
	return inferredTableName
}

// ensureTableSet ensures that the table is set for the current DB operation.
// It checks if the model matches any registered model and sets the appropriate table name if needed.
// This is a critical part of the transaction table resolution functionality that fixes the
// "Table not set" error in GORM transactions.
//
// The "Table not set" bug occurs in GORM when models have both cross-package relationships
// AND lifecycle hooks (like BeforeCreate, BeforeSave) especially when those hooks use map
// operations. During transaction initialization, GORM loses track of the table name when
// cloning the statement, resulting in errors during Create operations.
//
// This function uses a multi-strategy approach to resolve table names:
// 1. Strategy 1: Use reflection to get table name from ReflectValue
// 2. Strategy 2: If only one model is registered, use that
// 3. Strategy 3: Special handling for map values (common in GORM)
// 4. Strategy 4: Try using Statement.Model if available
// 5. Strategy 5: Try using Statement.Dest if available
//
// The function handles multiple ways that models can be passed to GORM:
// 1. Via the Statement.Model field (set by Model())
// 2. Via the Statement.ReflectValue field (which can contain):
//   - A pointer to a struct
//   - A direct struct value
//   - A slice of structs or pointers
//   - A map (for Create operations with map values)
//
// 3. Via the Statement.Dest field when ReflectValue is a map
//
// For each case, it attempts to determine the correct table name using:
// 1. The model's TableName() method (if available)
// 2. The registered table name from RegisterModel() or RegisterModelWithRelationships()
// 3. As a last resort, creating a new instance of the model type to get its TableName
//
// Enhanced features to handle complex models:
// - Special detection for models with both relationships AND lifecycle hooks
// - Clean instance creation to avoid hook side effects
// - Type name matching for complex models that have been transformed by GORM
// - Multiple fallback strategies for resolving the table name
//
// This solution works regardless of how the GORM operation is invoked, fixing
// various edge cases that could lead to "Table not set" errors, including:
// - Using RegisterModelWithRelationships with custom TableName models
// - Map values in Create operations
// - Transactions where model info is lost during processing
// - Models with both relationships AND hooks that use map operations
func (th *TransactionTestHelper) ensureTableSet(db *gorm.DB) {
	// If a table is already set, no need to do anything
	if db.Statement.Table != "" {
		return
	}

	// Strategy 1: Use reflection to get table name from ReflectValue
	if db.Statement.ReflectValue.IsValid() {
		value := db.Statement.ReflectValue.Interface()

		// Try direct interface method call
		if tabler, ok := value.(interface{ TableName() string }); ok {
			db.Statement.Table = tabler.TableName()
			return
		}

		// Try pointer receiver method with reflection
		modelValue := reflect.ValueOf(value)
		if modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
			if method := modelValue.MethodByName("TableName"); method.IsValid() {
				results := method.Call(nil)
				if len(results) > 0 && results[0].Kind() == reflect.String {
					db.Statement.Table = results[0].String()
					return
				}
			}

			// For cross-package relationships, use type name
			modelType := modelValue.Type().Elem()
			modelName := toSnakeCase(modelType.Name())

			// Thread safety for accessing registeredModels
			th.tc.mu.Lock()
			defer th.tc.mu.Unlock()

			// Try registered models with pluralized name
			pluralName := pluralizer.Plural(modelName)
			if _, ok := th.tc.registeredModels[pluralName]; ok {
				db.Statement.Table = pluralName
				return
			}

			// Try singular name
			if _, ok := th.tc.registeredModels[modelName]; ok {
				db.Statement.Table = modelName
				return
			}
		}
	}

	// Strategy 2: If only one model is registered, use that
	// Need to lock since we're accessing registeredModels
	th.tc.mu.Lock()
	if len(th.tc.registeredModels) == 1 {
		for tableName := range th.tc.registeredModels {
			db.Statement.Table = tableName
			th.tc.mu.Unlock()
			return
		}
	}
	th.tc.mu.Unlock()

	// Strategy 3: Special handling for map values (common in GORM)
	if db.Statement.ReflectValue.Kind() == reflect.Map {
		// When working with map values, GORM often loses table info
		th.tc.mu.Lock()
		if len(th.tc.registeredModels) > 0 {
			// Just use the first registered model as a best guess
			for tableName := range th.tc.registeredModels {
				db.Statement.Table = tableName
				th.tc.mu.Unlock()
				return
			}
		}
		th.tc.mu.Unlock()
	}

	// Strategy 4: Try using Statement.Model if available
	if db.Statement.Model != nil {
		th.setTableFromModel(db, db.Statement.Model)
		if db.Statement.Table != "" {
			return
		}
	}

	// Strategy 5: Try using Statement.Dest if available
	if db.Statement.Dest != nil {
		// Try direct interface method call
		if tabler, ok := db.Statement.Dest.(interface{ TableName() string }); ok {
			db.Statement.Table = tabler.TableName()
			return
		}

		// Try reflection
		destValue := reflect.ValueOf(db.Statement.Dest)
		if destValue.Kind() == reflect.Ptr && !destValue.IsNil() {
			if method := destValue.MethodByName("TableName"); method.IsValid() {
				results := method.Call(nil)
				if len(results) > 0 && results[0].Kind() == reflect.String {
					db.Statement.Table = results[0].String()
					return
				}
			}

			// Try lookup by type name
			destType := destValue.Elem().Type()
			destTypeName := toSnakeCase(destType.Name())

			// Thread safety
			th.tc.mu.Lock()
			defer th.tc.mu.Unlock()

			// Try plural name
			pluralName := pluralizer.Plural(destTypeName)
			if _, ok := th.tc.registeredModels[pluralName]; ok {
				db.Statement.Table = pluralName
				return
			}

			// Try singular name
			if _, ok := th.tc.registeredModels[destTypeName]; ok {
				db.Statement.Table = destTypeName
				return
			}
		}

		// Then try standard table resolution from the model
		th.setTableFromModel(db, db.Statement.Dest)
		if db.Statement.Table != "" {
			return
		}

		// NEW: If we're dealing with a complex validation model, do additional resolution
		// This handles models with both hooks and relationships that use validation
		destType := reflect.TypeOf(db.Statement.Dest)
		if destType != nil && destType.Kind() == reflect.Ptr {
			destValue := reflect.ValueOf(db.Statement.Dest)

			// Check if this is a validation model by looking for Validate method
			validateMethod := destValue.MethodByName("Validate")
			if validateMethod.IsValid() {
				// Has validation - check if it also has relationships by examining fields
				hasRelationships := false
				if destType.Elem().Kind() == reflect.Struct {
					for i := 0; i < destType.Elem().NumField(); i++ {
						field := destType.Elem().Field(i)
						fieldType := field.Type

						if fieldType.Kind() == reflect.Ptr {
							fieldType = fieldType.Elem()
						}

						// Check for struct fields or slices of structs (relationships)
						if fieldType.Kind() == reflect.Struct ||
							(fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct) {
							// Skip GORM Model embedding
							if field.Anonymous && fieldType.Name() == "Model" {
								continue
							}

							// Check for relationship tags
							tag := field.Tag.Get("gorm")
							if strings.Contains(tag, "foreignKey") ||
								strings.Contains(tag, "references") ||
								strings.Contains(tag, "many2many") {
								hasRelationships = true
								break
							}

							// Non-time, non-primitive struct field is likely a relationship
							if !field.Anonymous &&
								fieldType.Name() != "Time" &&
								fieldType.Name() != "NullTime" &&
								!strings.HasPrefix(fieldType.PkgPath(), "time") &&
								fieldType.Kind() == reflect.Struct {
								hasRelationships = true
								break
							}
						}
					}
				}

				// For models with both validation and relationships, do enhanced resolution
				if hasRelationships {
					// Special case handled - first try the registered model lookup
					th.tc.mu.Lock()
					for tableName, model := range th.tc.registeredModels {
						modelType := reflect.TypeOf(model)

						// Try exact type match first
						if modelType == destType ||
							(modelType.Kind() == reflect.Ptr && destType.Kind() == reflect.Ptr &&
								modelType.Elem() == destType.Elem()) {
							// Found direct match - use registered table name
							db.Statement.Table = tableName
							th.tc.mu.Unlock()
							return
						}

						// Try name match next
						if modelType.Kind() == reflect.Ptr {
							modelType = modelType.Elem()
						}
						destElemType := destType.Elem()

						if destElemType.Name() == modelType.Name() {
							// Name match - use registered table name
							db.Statement.Table = tableName
							th.tc.mu.Unlock()
							return
						}
					}
					th.tc.mu.Unlock()

					// Last resort: get table name from type
					typeName := destType.Elem().Name()
					if typeName != "" {
						// Use proper pluralizer for table name
						tableName := pluralizer.Plural(toSnakeCase(typeName))
						db.Statement.Table = tableName
						return
					}
				}
			}
		}
	}

	// If we couldn't set from Statement.Dest, try the ReflectValue if available
	if db.Statement.ReflectValue.IsValid() {
		// For pointer to struct
		if db.Statement.ReflectValue.Kind() == reflect.Ptr &&
			!db.Statement.ReflectValue.IsNil() &&
			db.Statement.ReflectValue.Elem().Kind() == reflect.Struct {
			modelValue := db.Statement.ReflectValue.Interface()
			th.setTableFromModel(db, modelValue)
			if db.Statement.Table != "" {
				return
			}
		}
		// For direct struct value
		if db.Statement.ReflectValue.Kind() == reflect.Struct {
			modelValue := db.Statement.ReflectValue.Interface()
			th.setTableFromModel(db, modelValue)
			if db.Statement.Table != "" {
				return
			}
		}
		// For slice of struct or pointer to struct
		if db.Statement.ReflectValue.Kind() == reflect.Slice && db.Statement.ReflectValue.Len() > 0 {
			// Try to get first element
			firstElem := db.Statement.ReflectValue.Index(0)
			if firstElem.IsValid() {
				if firstElem.Kind() == reflect.Ptr && !firstElem.IsNil() {
					modelValue := firstElem.Interface()
					th.setTableFromModel(db, modelValue)
					if db.Statement.Table != "" {
						return
					}
				} else if firstElem.Kind() == reflect.Struct {
					modelValue := firstElem.Interface()
					th.setTableFromModel(db, modelValue)
					if db.Statement.Table != "" {
						return
					}
				}
			}
		}
		// For map values, which GORM uses internally sometimes
		if db.Statement.ReflectValue.Kind() == reflect.Map {
			// Check if there's a "TableName" field in the map
			// This happens during some GORM transaction operations
			if db.Statement.Dest != nil {
				th.setTableFromModel(db, db.Statement.Dest)
				if db.Statement.Table != "" {
					return
				}
			}

			// If we still don't have a table name, check if this is a Create operation
			// where ReflectValue is used as the value map but the model/table is lost
			if db.Statement.Dest != nil {
				destType := reflect.TypeOf(db.Statement.Dest)
				if destType.Kind() == reflect.Ptr {
					destType = destType.Elem()
				}

				if destType.Kind() == reflect.Struct {
					// Try to get the table name from the destination type
					tableName := th.tryGetTableName(db.Statement.Dest)
					if tableName != "" {
						db.Statement.Table = tableName
						return
					}
				}
			}

			// Enhanced handling for complex models with relationships
			// For complex models with relationships, the map transformation may have lost the original model type
			// Try to find the model type by examining registered models that might match
			if db.Statement.Table == "" && db.Statement.Dest != nil {
				// Lock for reading registered models
				th.tc.mu.Lock()
				defer th.tc.mu.Unlock()

				// Get the type name of the destination
				destType := reflect.TypeOf(db.Statement.Dest)
				if destType.Kind() == reflect.Ptr {
					destType = destType.Elem()
				}
				destTypeName := destType.String()

				// Look for models with matching type name in the registry
				for registeredTableName, registeredModel := range th.tc.registeredModels {
					registeredType := reflect.TypeOf(registeredModel)
					if registeredType.Kind() == reflect.Ptr {
						registeredType = registeredType.Elem()
					}

					// If we find a matching type name, use its table name
					if registeredType.String() == destTypeName {
						tableNameFromModel := th.tryGetTableName(registeredModel)
						if tableNameFromModel != "" {
							db.Statement.Table = tableNameFromModel
							return
						}

						// Or use the table name from registration
						db.Statement.Table = registeredTableName
						return
					}
				}
			}
		}
	}

	// If all else fails and we have a model but no table, try to use the model's type to infer table name
	if db.Statement.Model != nil {
		modelType := reflect.TypeOf(db.Statement.Model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}

		if modelType.Kind() == reflect.Struct {
			// Create a new clean instance to ensure no hook side effects interfere
			newInstance := reflect.New(modelType).Interface()
			tableName := th.tryGetTableName(newInstance)
			if tableName != "" {
				db.Statement.Table = tableName
				return
			}

			// As a final fallback, check if this model type is registered with a custom table name
			th.tc.mu.Lock()
			defer th.tc.mu.Unlock()

			for tableName, model := range th.tc.registeredModels {
				regModelType := reflect.TypeOf(model)
				if regModelType.Kind() == reflect.Ptr {
					regModelType = regModelType.Elem()
				}

				if modelType == regModelType {
					db.Statement.Table = tableName
					return
				}
			}
		}
	}
}

// setTableFromModel sets the table name in the DB statement based on a model.
// This function is a critical part of the fix for the "Table not set" error in GORM transactions.
//
// It attempts to get the table name using multiple approaches:
//  1. First tries to get the table name directly from the model's TableName() method
//     using our enhanced tryGetTableName helper, which works with both value and pointer receivers
//  2. If that fails, it checks the model type against all registered models to find a match
//  3. If direct type comparison fails, it tries matching by type name for complex models
//  4. For models with both relationships AND hooks, special handling ensures proper table resolution
//
// Enhanced features:
// - Detection of models with both relationships and lifecycle hooks
// - Creates clean instances of models to avoid hook side effects during table name resolution
// - Matches by type name when direct type comparison fails, for models transformed by GORM
// - Additional relationship tag detection to identify relationship fields
// - More precise type matching for complex scenarios
//
// When a match is found in registered models, it still prioritizes getting the table name
// from the model's TableName() method over using the registered table name, which ensures
// that any runtime customization of table names is respected.
//
// This approach ensures proper table resolution in all cases, fixing the issues with
// GORM's internal table resolution during transactions, even for complex models with both
// relationships and lifecycle hooks.
func (th *TransactionTestHelper) setTableFromModel(db *gorm.DB, model interface{}) {
	if model == nil {
		return
	}

	// Check if this is a model with both hooks and relationships
	// They need special handling due to their complex nature
	modelType := reflect.TypeOf(model)
	modelValue := reflect.ValueOf(model)

	hasHooks := false
	hasRelationships := false

	// Only analyze if this is a struct or pointer to struct
	if modelType.Kind() == reflect.Ptr && !modelValue.IsNil() {
		elemType := modelType.Elem()
		if elemType.Kind() == reflect.Struct {
			// Check for hooks
			hookMethods := []string{"BeforeCreate", "BeforeUpdate", "BeforeSave", "Validate"}
			for _, hookName := range hookMethods {
				method := modelValue.MethodByName(hookName)
				if method.IsValid() {
					hasHooks = true
					break
				}
			}

			// Check for relationship fields
			for i := 0; i < elemType.NumField(); i++ {
				field := elemType.Field(i)

				// Check field type - could be a struct or slice of structs for relationships
				fieldType := field.Type
				if fieldType.Kind() == reflect.Ptr {
					fieldType = fieldType.Elem()
				}

				if fieldType.Kind() == reflect.Struct ||
					(fieldType.Kind() == reflect.Slice &&
						fieldType.Elem().Kind() == reflect.Struct) {

					// Skip standard GORM Model embedding
					if field.Anonymous && fieldType.Name() == "Model" {
						continue
					}

					// First, look for GORM struct tags that indicate relationships
					tag := field.Tag.Get("gorm")
					if strings.Contains(tag, "foreignKey") ||
						strings.Contains(tag, "references") ||
						strings.Contains(tag, "many2many") {
						hasRelationships = true
						break
					}

					// Even without explicit GORM tags, a non-primitive struct field
					// that isn't an embedded type is likely a relationship
					if !field.Anonymous &&
						fieldType.Name() != "Time" && // Skip time.Time fields
						fieldType.Name() != "NullTime" && // Skip sql.NullTime fields
						!strings.HasPrefix(fieldType.PkgPath(), "time") && // Skip other time-related fields
						fieldType.Kind() == reflect.Struct {
						hasRelationships = true
						break
					}

					// Check for slices that could be has-many relationships
					if fieldType.Kind() == reflect.Slice &&
						fieldType.Elem().Kind() == reflect.Struct {
						hasRelationships = true
						break
					}
				}
			}
		}
	}

	// For models with both hooks and relationships, we need to be extra careful
	if hasHooks && hasRelationships {
		// Try to find the model type in our registrations first (most reliable)
		modelTypeElem := modelType
		if modelTypeElem.Kind() == reflect.Ptr {
			modelTypeElem = modelTypeElem.Elem()
		}

		th.tc.mu.Lock()
		defer th.tc.mu.Unlock()

		// Try to find a direct registration match first
		for registeredTableName, registeredModel := range th.tc.registeredModels {
			registeredType := reflect.TypeOf(registeredModel)
			if registeredType.Kind() == reflect.Ptr {
				registeredType = registeredType.Elem()
			}

			// If types match exactly, use the registered table name
			if modelTypeElem == registeredType {
				// Try to get the table name from a clean instance to avoid hook interference
				cleanModel := reflect.New(registeredType).Interface()
				tableNameFromModel := th.tryGetTableName(cleanModel)
				if tableNameFromModel != "" {
					db.Statement.Table = tableNameFromModel
					return
				}

				// Otherwise use the table name from registration
				db.Statement.Table = registeredTableName
				return
			}
		}

		// If no direct match found, create a clean instance and try to get the table name
		if modelType.Kind() == reflect.Ptr {
			cleanModel := reflect.New(modelType.Elem()).Interface()
			tableName := th.tryGetTableName(cleanModel)
			if tableName != "" {
				db.Statement.Table = tableName
				return
			}
		}

		// Continue with standard processing
	}

	// First, try to get the table name using our improved tryGetTableName method
	tableName := th.tryGetTableName(model)
	if tableName != "" {
		db.Statement.Table = tableName
		return
	}

	// Get the model type to check against registered models
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return
	}

	// Lock for reading registered models
	th.tc.mu.Lock()
	defer th.tc.mu.Unlock()

	// Check if we can find a matching model registration
	for registeredTableName, registeredModel := range th.tc.registeredModels {
		registeredType := reflect.TypeOf(registeredModel)
		if registeredType.Kind() == reflect.Ptr {
			registeredType = registeredType.Elem()
		}

		// If we found a matching model type, set the table name
		if modelType == registeredType {
			// First try to get the table name from the registered model
			tableNameFromModel := th.tryGetTableName(registeredModel)
			if tableNameFromModel != "" {
				db.Statement.Table = tableNameFromModel
				return
			}

			// Otherwise use the table name from registration
			db.Statement.Table = registeredTableName
			return
		}

		// Check if the type names match, which can help with models with relationships
		// This is especially helpful when GORM has processed the model internally
		if modelType.String() == registeredType.String() {
			// Try to get the table name from the registered model
			tableNameFromModel := th.tryGetTableName(registeredModel)
			if tableNameFromModel != "" {
				db.Statement.Table = tableNameFromModel
				return
			}

			// Otherwise use the table name from registration
			db.Statement.Table = registeredTableName
			return
		}
	}
}

// WithRollbackOnly sets up a transaction that will be rolled back regardless of the result.
// Like ExecuteInTransaction, it ensures proper table resolution for registered models with
// all the same table name resolution fixes to prevent "Table not set" errors.
//
// This is useful for testing code that should operate within a transaction without
// actually committing changes, such as read-only operations or testing error conditions.
//
// Example:
//
//	testCtx.Transaction().WithRollbackOnly(func(tx *gorm.DB) error {
//	    // Run operations that you want to test but don't want committed
//	    return tx.First(&user, 1).Error
//	})
func (th *TransactionTestHelper) WithRollbackOnly(fn func(*gorm.DB) error) error {
	// Start by expecting a transaction
	th.tc.mock.ExpectBegin()

	// Get transaction from the DB
	tx := th.tc.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Create a transaction wrapper that correctly handles model table resolution
	txWrapper := th.wrapTransactionWithTableInfo(tx)

	// Execute the function
	err := fn(txWrapper)

	// Always rollback
	th.tc.mock.ExpectRollback()
	tx.Rollback()

	return err
}

// WithCommitOnly sets up a transaction that will be committed regardless of the result.
// Like ExecuteInTransaction, it ensures proper table resolution for registered models with
// all the same table name resolution fixes to prevent "Table not set" errors.
//
// This is useful for testing code where you want to ensure the transaction is always
// committed, regardless of whether the function returns an error or not. This can be
// helpful for operations that are expected to partially fail but still commit their work.
//
// Example:
//
//	testCtx.Transaction().WithCommitOnly(func(tx *gorm.DB) error {
//	    // Guaranteed to commit even if this returns an error
//	    return tx.Create(&model).Error
//	})
func (th *TransactionTestHelper) WithCommitOnly(fn func(*gorm.DB) error) error {
	// Start by expecting a transaction
	th.tc.mock.ExpectBegin()

	// Get transaction from the DB
	tx := th.tc.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Create a transaction wrapper that correctly handles model table resolution
	txWrapper := th.wrapTransactionWithTableInfo(tx)

	// Execute the function
	err := fn(txWrapper)

	// Always commit
	th.tc.mock.ExpectCommit()
	if commitErr := tx.Commit().Error; commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	return err
}

// RunTransactionTests runs a set of transaction test cases
func RunTransactionTests(t *testing.T, testCases []TransactionTestCase, opts ...func(*Config)) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Setup test context with provided options
			testCtx := NewDBTestContext(t, opts...)
			defer testCtx.Teardown()

			// Setup mock expectations
			if tc.SetupMock != nil {
				tc.SetupMock(testCtx)
			}

			// Execute service operation
			err := tc.ExecuteService(testCtx)

			// Verify expectations
			if tc.ExpectError {
				assert.Error(t, err)
				if tc.ErrorContains != "" {
					assert.Contains(t, err.Error(), tc.ErrorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			testCtx.VerifyExpectations()
		})
	}
}

// Transaction returns a transaction test helper for the test context
func (tc *DBTestContext) Transaction() *TransactionTestHelper {
	helper := NewTransactionTestHelper(tc)

	// Add special handling for all model types including complex validation models
	// This callback runs early in the GORM Create process to ensure table resolution
	helper.tc.DB().Callback().Create().Before("gorm:create").Register(
		fmt.Sprintf("testutil:ensure_complex_table_fix:%s", helper.sessionID),
		func(db *gorm.DB) {
			// Only process if table is not already set
			if db.Statement.Table != "" {
				return
			}

			// Try direct table resolution for any model
			if db.Statement.Dest != nil {
				destVal := reflect.ValueOf(db.Statement.Dest)
				if destVal.Kind() == reflect.Ptr && !destVal.IsNil() {
					// Direct method call for TableName is most reliable
					if tableNameMethod := destVal.MethodByName("TableName"); tableNameMethod.IsValid() {
						results := tableNameMethod.Call(nil)
						if len(results) > 0 && results[0].Kind() == reflect.String {
							tableName := results[0].String()
							if tableName != "" {
								db.Statement.Table = tableName
								return
							}
						}
					}

					// Check for Validate method - indicator of complex validation
					validateMethod := destVal.MethodByName("Validate")
					if validateMethod.IsValid() {
						// Check for relationships
						destType := reflect.TypeOf(db.Statement.Dest)
						hasRelationships := false

						if destType.Elem().Kind() == reflect.Struct {
							for i := 0; i < destType.Elem().NumField(); i++ {
								field := destType.Elem().Field(i)

								// Skip embedded GORM Model
								if field.Anonymous && field.Type.Name() == "Model" {
									continue
								}

								// Check field type for relationships
								fieldType := field.Type
								if fieldType.Kind() == reflect.Ptr {
									fieldType = fieldType.Elem()
								}

								// Struct fields (not time.Time) or slices of structs are likely relationships
								if (fieldType.Kind() == reflect.Struct &&
									fieldType.Name() != "Time" &&
									fieldType.Name() != "NullTime" &&
									!strings.HasPrefix(fieldType.PkgPath(), "time")) ||
									(fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct) {

									// Check for relationship tags
									tag := field.Tag.Get("gorm")
									if tag != "" && (strings.Contains(tag, "foreignKey") ||
										strings.Contains(tag, "references") ||
										strings.Contains(tag, "many2many")) {
										hasRelationships = true
										break
									}

									// Non-anonymous struct field likely indicates relationship
									if !field.Anonymous {
										hasRelationships = true
										break
									}
								}
							}
						}

						// For models with both validation and relationships, use comprehensive resolution
						if hasRelationships {
							// Try to find in registered models first
							found := false
							tc.mu.Lock()
							for tableName, model := range tc.registeredModels {
								modelType := reflect.TypeOf(model)

								// Try direct type match
								if modelType == destType ||
									(modelType.Kind() == reflect.Ptr && destType.Kind() == reflect.Ptr &&
										modelType.Elem() == destType.Elem()) {
									db.Statement.Table = tableName
									found = true
									break
								}

								// Try name match
								if modelType.Kind() == reflect.Ptr {
									modelType = modelType.Elem()
								}

								if modelType.Name() == destType.Elem().Name() {
									db.Statement.Table = tableName
									found = true
									break
								}

								// Try package path + name match
								if modelType.PkgPath() == destType.Elem().PkgPath() &&
									modelType.Name() == destType.Elem().Name() {
									db.Statement.Table = tableName
									found = true
									break
								}
							}
							tc.mu.Unlock()

							if found {
								return
							}

							// Last resort: infer from type name using proper pluralization
							typeName := destType.Elem().Name()
							if typeName != "" {
								snakeCase := toSnakeCase(typeName)
								db.Statement.Table = pluralizer.Plural(snakeCase)
								return
							}
						}
					}
				}
			}
		},
	)

	// Also register for Query operations to ensure consistent resolution
	helper.tc.DB().Callback().Query().Before("gorm:query").Register(
		fmt.Sprintf("testutil:ensure_query_table_fix:%s", helper.sessionID),
		func(db *gorm.DB) {
			// Reuse the same logic as Create - call our improved table resolution
			if db.Statement.Table == "" {
				helper.ensureTableSet(db)
			}
		},
	)

	return helper
}

// WithRollback adds a rollback expectation to the transaction test case
func (tc *TransactionTestCase) WithRollback() *TransactionTestCase {
	oldSetupMock := tc.SetupMock
	tc.SetupMock = func(dtc *DBTestContext) {
		if oldSetupMock != nil {
			oldSetupMock(dtc)
		}
		dtc.mock.ExpectRollback()
	}
	return tc
}

// WithCommit adds a commit expectation to the transaction test case
func (tc *TransactionTestCase) WithCommit() *TransactionTestCase {
	oldSetupMock := tc.SetupMock
	tc.SetupMock = func(dtc *DBTestContext) {
		if oldSetupMock != nil {
			oldSetupMock(dtc)
		}
		dtc.mock.ExpectCommit()
	}
	return tc
}

// WithPagination adds pagination to the transaction test case
func (tc *TransactionTestCase) WithPagination(page, pageSize int) *TransactionTestCase {
	paginator := NewPaginationHelper()
	tc.Pagination = paginator.CreatePagination(page, pageSize)
	return tc
}

// WithTimeout adds a timeout to the transaction test case
func (tc *TransactionTestCase) WithTimeout(timeout time.Duration) *TransactionTestCase {
	oldExecuteService := tc.ExecuteService
	tc.ExecuteService = func(ctx core.Context) error {
		// Create a context with timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Create a channel to receive the result
		resultChan := make(chan error, 1)

		// Execute the service in a goroutine
		go func() {
			resultChan <- oldExecuteService(ctx)
		}()

		// Wait for either the result or timeout
		select {
		case result := <-resultChan:
			return result
		case <-timeoutCtx.Done():
			return fmt.Errorf("operation timed out after %v", timeout)
		}
	}
	return tc
}

// WithRetry adds retry logic to the transaction test case
func (tc *TransactionTestCase) WithRetry(maxRetries int, retryDelay time.Duration) *TransactionTestCase {
	oldExecuteService := tc.ExecuteService
	tc.ExecuteService = func(ctx core.Context) error {
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			err := oldExecuteService(ctx)
			if err == nil {
				return nil
			}
			lastErr = err
			time.Sleep(retryDelay)
		}
		return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
	}
	return tc
}

// WithBeforeHook adds a hook to run before the service execution
func (tc *TransactionTestCase) WithBeforeHook(hook func(core.Context)) *TransactionTestCase {
	oldExecuteService := tc.ExecuteService
	tc.ExecuteService = func(ctx core.Context) error {
		hook(ctx)
		return oldExecuteService(ctx)
	}
	return tc
}

// WithAfterHook adds a hook to run after the service execution
func (tc *TransactionTestCase) WithAfterHook(hook func(core.Context, error)) *TransactionTestCase {
	oldExecuteService := tc.ExecuteService
	tc.ExecuteService = func(ctx core.Context) error {
		err := oldExecuteService(ctx)
		hook(ctx, err)
		return err
	}
	return tc
}

// getRegisteredTableName tries to find the correct table name for a transaction
// when the normal GORM table resolution has failed. This is particularly important
// for handling edge cases with models that have both cross-package relationships
// and validation hooks that use map operations.
//
// This function is a more aggressive approach to finding the correct table name
// compared to the standard GORM mechanisms, and is designed to handle the specific
// edge case where GORM loses table information during transactions.
//
// The function tries several fallback strategies:
// 1. Using SQL statement parsing if available
// 2. Checking for models with hooks and relationships
// 3. Using the first registered model as a last resort
//
// This more aggressive approach is only used when all other table resolution strategies
// have failed and is specifically targeted at the edge case with models that have both
// cross-package relationships and hooks that use map operations.
func (th *TransactionTestHelper) getRegisteredTableName(db *gorm.DB) string {
	// If we have a table in the statement context, use it
	if db.Statement.Table != "" {
		return db.Statement.Table
	}

	// If we don't have many registered models, there's a good chance one of them is what we want
	if len(th.tc.registeredModels) == 1 {
		// With only one registered model, it's likely the one we're working with
		for tableName := range th.tc.registeredModels {
			return tableName
		}
	}

	// Try to extract schema from actual SQL prepared by GORM if available
	if db.Statement.SQL.String() != "" {
		// Extract table name pattern from SQL like "INSERT INTO `table_name`"
		sqlText := db.Statement.SQL.String()
		if strings.Contains(sqlText, "INSERT INTO") {
			// Extract the table name from the SQL
			parts := strings.Split(sqlText, "`")
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	// If we have a Dest field that is a slice or struct, we can use that to determine the table
	if db.Statement.Dest != nil {
		if tableNameGetter, ok := db.Statement.Dest.(interface{ TableName() string }); ok {
			tableName := tableNameGetter.TableName()
			return tableName
		}
	}

	// Check all registered models for a match based on special characteristics
	th.tc.mu.Lock()
	defer th.tc.mu.Unlock()

	// If we're doing a Create operation, see which model has validation hooks
	// Models with BeforeCreate/BeforeUpdate hooks that call Validate() are
	// the most likely to trigger this bug
	for tableName, model := range th.tc.registeredModels {
		modelType := reflect.TypeOf(model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}

		// Special case for models with hooks and relationships
		if th.modelHasHooksAndRelationships(modelType) {
			return tableName
		}
	}

	// FALLBACK: Just get the first registered model's table
	// In test environments, this is often the right one
	for tableName := range th.tc.registeredModels {
		return tableName // Just return the first one as a fallback
	}

	// If nothing worked, return empty string
	return ""
}

// modelHasHooksAndRelationships is a helper that checks if a model type has both
// hooks (BeforeCreate/BeforeUpdate) and relationships to other models.
// These models are the most likely to trigger the "Table not set" bug.
//
// The function specifically looks for:
// 1. Lifecycle hooks like BeforeCreate, BeforeUpdate, and Validate
// 2. Relationship fields identified by:
//   - GORM relationship tags (foreignKey, references, many2many)
//   - Cross-package struct fields (fields from different packages)
//
// When both hooks and relationships are present, GORM is most likely to lose
// track of table names during transaction initialization, especially when those
// hooks perform operations on maps or complex data structures.
func (th *TransactionTestHelper) modelHasHooksAndRelationships(modelType reflect.Type) bool {
	if modelType.Kind() != reflect.Struct {
		return false
	}

	// Check for hooks
	hasHooks := false
	ptrType := reflect.PtrTo(modelType)
	_, hasBeforeCreate := ptrType.MethodByName("BeforeCreate")
	_, hasBeforeUpdate := ptrType.MethodByName("BeforeUpdate")
	_, hasValidate := ptrType.MethodByName("Validate")

	if hasBeforeCreate || hasBeforeUpdate {
		hasHooks = true
	} else if hasValidate {
		hasHooks = true
	}

	// Check for relationships (struct fields that aren't primitive types)
	hasRelationships := false
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		fieldType := field.Type

		// Skip embedded fields
		if field.Anonymous {
			continue
		}

		// Check if this is a relationship field
		if fieldType.Kind() == reflect.Struct ||
			(fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct) ||
			(fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct) ||
			(fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Ptr && fieldType.Elem().Elem().Kind() == reflect.Struct) {
			// Look for gorm tags that indicate relationships
			if tag := field.Tag.Get("gorm"); tag != "" {
				if strings.Contains(tag, "foreignKey") ||
					strings.Contains(tag, "references") ||
					strings.Contains(tag, "many2many") {
					hasRelationships = true
					break
				}
			}

			// If the field has a different package than the struct, it's likely a cross-package relationship
			if fieldType.Kind() == reflect.Struct && fieldType.PkgPath() != modelType.PkgPath() {
				hasRelationships = true
				break
			}

			if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct &&
				fieldType.Elem().PkgPath() != modelType.PkgPath() {
				hasRelationships = true
				break
			}
		}
	}

	return hasHooks && hasRelationships
}
