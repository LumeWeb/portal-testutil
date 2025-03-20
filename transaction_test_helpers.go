package testutil

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

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

	// Execute the function with our wrapped transaction
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
//
// It tries multiple approaches to obtain the table name:
// 1. First attempts to call TableName() on the pointer type (for pointer receiver methods)
// 2. Then attempts to call TableName() on the value type (for value receiver methods)
// 3. For complex models with relationships, it inspects GORM struct tags for table information
// 4. For models with lifecycle hooks, it creates a clean instance to avoid hook interference
//
// The enhanced implementation adds special handling for complex models with relationships
// by looking for GORM struct tags that contain table information, which helps resolve
// the table name even when the model has been transformed internally by GORM.
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
		hookMethods := []string{"BeforeCreate", "BeforeUpdate", "BeforeSave", "Validate"}

		for _, hookName := range hookMethods {
			method := modelValue.MethodByName(hookName)
			if method.IsValid() {
				hasLifecycleHooks = true
				break
			}
		}

		if hasLifecycleHooks {
			// Create a clean instance of the model to get the table name
			// without triggering hooks or side effects
			cleanType := modelType
			if cleanType.Kind() == reflect.Ptr {
				cleanType = cleanType.Elem()
			}
			cleanInstance := reflect.New(cleanType).Interface()

			// Recursively try with clean instance, but make sure we don't create
			// an infinite loop
			return th.tryGetTableName(cleanInstance)
		}
	}

	return ""
}

// ensureTableSet ensures that the table is set for the current DB operation.
// It checks if the model matches any registered model and sets the appropriate table name if needed.
// This is a critical part of the transaction table resolution functionality that fixes the
// "Table not set" error in GORM transactions.
//
// This function handles multiple ways that models can be passed to GORM:
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
// - Models with both relationships AND hooks
func (th *TransactionTestHelper) ensureTableSet(db *gorm.DB) {
	// If a table is already set, no need to do anything
	if db.Statement.Table != "" {
		return
	}

	// First try using the Statement.Model if available
	if db.Statement.Model != nil {
		th.setTableFromModel(db, db.Statement.Model)
		if db.Statement.Table != "" {
			return
		}
	}

	// If we couldn't set from Statement.Model, try the ReflectValue if available
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

	// Check the Dest field if it's available
	if db.Statement.Dest != nil && db.Statement.Table == "" {
		th.setTableFromModel(db, db.Statement.Dest)
		if db.Statement.Table != "" {
			return
		}

		// Special handling for models with relationships AND hooks
		// Check if Dest is a model with hooks and provide extra processing
		destType := reflect.TypeOf(db.Statement.Dest)
		if destType != nil && destType.Kind() == reflect.Ptr {
			destValue := reflect.ValueOf(db.Statement.Dest)
			hasHooks := false
			hookMethods := []string{"BeforeCreate", "BeforeUpdate", "BeforeSave", "Validate"}

			// Check if it has hooks
			for _, hookName := range hookMethods {
				method := destValue.MethodByName(hookName)
				if method.IsValid() {
					hasHooks = true
					break
				}
			}

			// Check if it has relationships by looking for fields with struct or slice types
			// that have gorm tags with foreignKey, references, many2many, etc.
			hasRelationships := false
			if destType.Elem().Kind() == reflect.Struct {
				for i := 0; i < destType.Elem().NumField(); i++ {
					field := destType.Elem().Field(i)

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

			// If it has both hooks and relationships, do more thorough processing
			if hasHooks && hasRelationships {
				// Try to find this model or its type in the registration map
				th.tc.mu.Lock()
				defer th.tc.mu.Unlock()

				for tableName, model := range th.tc.registeredModels {
					modelType := reflect.TypeOf(model)
					if modelType == destType ||
						(modelType.Kind() == reflect.Ptr && destType.Kind() == reflect.Ptr &&
							modelType.Elem() == destType.Elem()) {
						// Found a direct match, use the registered table name
						db.Statement.Table = tableName
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
	return NewTransactionTestHelper(tc)
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
