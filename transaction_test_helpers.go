package testutil

import (
	"context"
	"fmt"
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
// commit or rollback based on the result
func (th *TransactionTestHelper) ExecuteInTransaction(fn func(*gorm.DB) error) error {
	// Start by expecting a transaction
	th.tc.mock.ExpectBegin()

	// Get transaction from the DB
	tx := th.tc.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Execute the function
	err := fn(tx)

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

// WithRollbackOnly sets up a transaction that will be rolled back regardless of the result
func (th *TransactionTestHelper) WithRollbackOnly(fn func(*gorm.DB) error) error {
	// Start by expecting a transaction
	th.tc.mock.ExpectBegin()

	// Get transaction from the DB
	tx := th.tc.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Execute the function
	err := fn(tx)

	// Always rollback
	th.tc.mock.ExpectRollback()
	tx.Rollback()

	return err
}

// WithCommitOnly sets up a transaction that will be committed regardless of the result
func (th *TransactionTestHelper) WithCommitOnly(fn func(*gorm.DB) error) error {
	// Start by expecting a transaction
	th.tc.mock.ExpectBegin()

	// Get transaction from the DB
	tx := th.tc.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Execute the function
	err := fn(tx)

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
