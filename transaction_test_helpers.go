package testutil

import (
	"context"
	"fmt"
	"reflect"
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

// TransactionTestHelper provides utilities for testing transactions
type TransactionTestHelper struct {
	tc *DBTestContext
}

// NewTransactionTestHelper creates a new transaction test helper
func NewTransactionTestHelper(tc *DBTestContext) *TransactionTestHelper {
	return &TransactionTestHelper{tc: tc}
}

// ExecuteInTransaction executes a function within a transaction and automatically handles
// commit or rollback based on the result. It also ensures proper table resolution for
// registered models, automatically resolving table names for models with custom TableName()
// methods used within the transaction.
//
// This function fixes the common "Table not set" error in GORM when using models with
// TableName() methods inside transactions by automatically registering appropriate
// callbacks to resolve table names. It works with both value and pointer receiver
// TableName() methods.
//
// Example:
//
//	testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
//	    // This will now correctly resolve the table name for MyModel
//	    return tx.Create(&MyModel{Name: "test"}).Error
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
// registered models via RegisterModels(). The wrapper ensures that each registered model
// is properly associated with its table name within transaction operations.
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
//
// This fixes issues with GORM internal table resolution that occur during transactions,
// even when models correctly implement the TableName() method.
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

	// Try to remove existing callbacks to avoid duplication
	th.removeExistingCallbacks(txWrapper)

	// Register callbacks for all operations that might need table resolution

	// Query operations (Find, First, Take, Last, Count, etc.)
	txWrapper.Callback().Query().Before("gorm:query").Register("testutil:ensure_query_table", func(db *gorm.DB) {
		// This is triggered before a SELECT query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Create operations (Save, Create)
	txWrapper.Callback().Create().Before("gorm:create").Register("testutil:ensure_create_table", func(db *gorm.DB) {
		// This is triggered before an INSERT query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Update operations (Update, Updates, Save with existing record)
	txWrapper.Callback().Update().Before("gorm:update").Register("testutil:ensure_update_table", func(db *gorm.DB) {
		// This is triggered before an UPDATE query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Delete operations (Delete, DeletedAt for soft delete)
	txWrapper.Callback().Delete().Before("gorm:delete").Register("testutil:ensure_delete_table", func(db *gorm.DB) {
		// This is triggered before a DELETE query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Raw SQL operations
	txWrapper.Callback().Raw().Before("gorm:raw").Register("testutil:ensure_raw_table", func(db *gorm.DB) {
		// This is triggered before a raw SQL query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	return txWrapper
}

// removeExistingCallbacks attempts to remove existing callbacks to avoid duplication warnings.
// This function is important when multiple transactions are created, as GORM will otherwise
// produce warning messages about duplicate callbacks.
//
// It removes all our custom callbacks from all processor types (Query, Create, Update, Delete, Raw).
// This ensures a clean slate before registering new callbacks, preventing the "duplicated callback"
// warnings that can occur when using multiple transaction helpers or nested transactions.
func (th *TransactionTestHelper) removeExistingCallbacks(db *gorm.DB) {
	callbackNames := []string{
		"testutil:ensure_query_table",
		"testutil:ensure_create_table",
		"testutil:ensure_update_table",
		"testutil:ensure_delete_table",
		"testutil:ensure_raw_table",
	}

	// Remove each callback from each processor
	for _, name := range callbackNames {
		db.Callback().Query().Remove(name)
		db.Callback().Create().Remove(name)
		db.Callback().Update().Remove(name)
		db.Callback().Delete().Remove(name)
		db.Callback().Raw().Remove(name)
	}
}

// tryGetTableName attempts to get a table name by calling a TableName() method on a model.
// It uses reflection to dynamically find and invoke the TableName() method if it exists.
//
// This function is a key part of the table resolution fix, handling:
// - Both value receiver and pointer receiver TableName() methods
// - Nil pointer models (by creating a new instance)
// - Direct struct values (by creating a pointer to the value)
//
// It tries multiple approaches to obtain the table name:
// 1. First attempts to call TableName() on the pointer type (for pointer receiver methods)
// 2. Then attempts to call TableName() on the value type (for value receiver methods)
//
// This comprehensive approach ensures that regardless of how the TableName() method
// is implemented (pointer or value receiver), the correct table name will be resolved.
func (th *TransactionTestHelper) tryGetTableName(model interface{}) string {
	if model == nil {
		return ""
	}

	modelValue := reflect.ValueOf(model)
	modelType := modelValue.Type()

	// Handle pointer types
	if modelValue.Kind() == reflect.Ptr {
		if modelValue.IsNil() {
			// Create a new instance for nil pointers
			modelValue = reflect.New(modelType.Elem())
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
//
// For each case, it attempts to determine the correct table name using:
// 1. The model's TableName() method (if available)
// 2. The registered table name from RegisterModel()
// 3. As a last resort, creating a new instance of the model type to get its TableName
//
// This solution works regardless of how the GORM operation is invoked, fixing
// various edge cases that could lead to "Table not set" errors.
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
	}

	// If all else fails and we have a model but no table, try to use the model's type to infer table name
	if db.Statement.Model != nil {
		modelType := reflect.TypeOf(db.Statement.Model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}

		if modelType.Kind() == reflect.Struct {
			// Try to create a new instance and check its TableName
			newInstance := reflect.New(modelType).Interface()
			tableName := th.tryGetTableName(newInstance)
			if tableName != "" {
				db.Statement.Table = tableName
			}
		}
	}
}

// setTableFromModel sets the table name in the DB statement based on a model.
// This function is a critical part of the fix for the "Table not set" error in GORM transactions.
//
// It attempts to get the table name using two approaches:
//  1. First tries to get the table name directly from the model's TableName() method
//     using our enhanced tryGetTableName helper, which works with both value and pointer receivers
//  2. If that fails, it checks the model type against all registered models to find a match
//
// When a match is found in registered models, it still prioritizes getting the table name
// from the model's TableName() method over using the registered table name, which ensures
// that any runtime customization of table names is respected.
//
// This approach ensures proper table resolution in all cases, fixing the issues with
// GORM's internal table resolution during transactions.
func (th *TransactionTestHelper) setTableFromModel(db *gorm.DB, model interface{}) {
	if model == nil {
		return
	}

	// First, try to get the table name using our improved tryGetTableName method
	tableName := th.tryGetTableName(model)
	if tableName != "" {
		db.Statement.Table = tableName
		return
	}

	// Get the model type to check against registered models
	modelType := reflect.TypeOf(model)
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
