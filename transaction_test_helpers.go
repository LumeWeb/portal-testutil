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

	// Register callbacks for all operations that might need table resolution

	// Query operations (Find, First, Take, Last, Count, etc.)
	txWrapper.Callback().Query().Before("gorm:query").Register("testutil:ensure_query_table", func(db *gorm.DB) {
		// This is triggered before a SELECT query is executed
		if db.Statement.Model != nil && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Create operations (Save, Create)
	txWrapper.Callback().Create().Before("gorm:create").Register("testutil:ensure_create_table", func(db *gorm.DB) {
		// This is triggered before an INSERT query is executed
		if db.Statement.Model != nil && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Update operations (Update, Updates, Save with existing record)
	txWrapper.Callback().Update().Before("gorm:update").Register("testutil:ensure_update_table", func(db *gorm.DB) {
		// This is triggered before an UPDATE query is executed
		if db.Statement.Model != nil && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Delete operations (Delete, DeletedAt for soft delete)
	txWrapper.Callback().Delete().Before("gorm:delete").Register("testutil:ensure_delete_table", func(db *gorm.DB) {
		// This is triggered before a DELETE query is executed
		if db.Statement.Model != nil && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	// Raw SQL operations
	txWrapper.Callback().Raw().Before("gorm:raw").Register("testutil:ensure_raw_table", func(db *gorm.DB) {
		// This is triggered before a raw SQL query is executed
		if db.Statement.Model != nil && db.Statement.Table == "" {
			th.ensureTableSet(db)
		}
	})

	return txWrapper
}

// tryGetTableName attempts to get a table name by calling a TableName() method on a model.
// It uses reflection to dynamically find and invoke the TableName() method if it exists.
func (th *TransactionTestHelper) tryGetTableName(model interface{}) string {
	if model == nil {
		return ""
	}

	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Ptr && !modelValue.IsNil() {
		// Try direct TableName method call
		tableNameMethod := modelValue.MethodByName("TableName")
		if tableNameMethod.IsValid() {
			results := tableNameMethod.Call(nil)
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
	if db.Statement.ReflectValue.IsValid() && db.Statement.ReflectValue.Kind() == reflect.Struct {
		modelValue := db.Statement.ReflectValue.Interface()
		th.setTableFromModel(db, modelValue)
		if db.Statement.Table != "" {
			return
		}
	}
}

// setTableFromModel sets the table name in the DB statement based on a model
func (th *TransactionTestHelper) setTableFromModel(db *gorm.DB, model interface{}) {
	if model == nil {
		return
	}

	// First, check if the model has a TableName method we can call directly
	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() == reflect.Ptr && !modelVal.IsNil() {
		// Try direct TableName method call
		tableNameMethod := modelVal.MethodByName("TableName")
		if tableNameMethod.IsValid() {
			results := tableNameMethod.Call(nil)
			if len(results) > 0 && results[0].Kind() == reflect.String {
				db.Statement.Table = results[0].String()
				return
			}
		}
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
	for tableName, registeredModel := range th.tc.registeredModels {
		registeredType := reflect.TypeOf(registeredModel)
		if registeredType.Kind() == reflect.Ptr {
			registeredType = registeredType.Elem()
		}

		// If we found a matching model type, set the table name
		if modelType == registeredType {
			// For models with TableName method, use the method's return value
			if tableNameMethod := reflect.ValueOf(registeredModel).MethodByName("TableName"); tableNameMethod.IsValid() {
				results := tableNameMethod.Call(nil)
				if len(results) > 0 && results[0].Kind() == reflect.String {
					db.Statement.Table = results[0].String()
					return
				}
			}

			// Otherwise use the table name from registration
			db.Statement.Table = tableName
			return
		}
	}
}

// WithRollbackOnly sets up a transaction that will be rolled back regardless of the result.
// Like ExecuteInTransaction, it ensures proper table resolution for registered models.
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
// Like ExecuteInTransaction, it ensures proper table resolution for registered models.
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
