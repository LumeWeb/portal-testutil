package testutil

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.lumeweb.com/portal/core"
)

// ConcurrentTestCase represents a test case for concurrent operations
type ConcurrentTestCase struct {
	Name           string
	SetupMock      func(*DBTestContext, int)
	ExecuteService func(core.Context, int, chan error)
	NumOperations  int
	ExpectErrors   bool
	mu             sync.Mutex    // Mutex to protect concurrent operations
	timeout        time.Duration // Timeout for the test case
}

// ConcurrentTestHelper provides utilities for testing concurrent operations
type ConcurrentTestHelper struct {
	tc *DBTestContext
}

// NewConcurrentTestHelper creates a new concurrent test helper
func NewConcurrentTestHelper(tc *DBTestContext) *ConcurrentTestHelper {
	return &ConcurrentTestHelper{tc: tc}
}

// RunWithConcurrency runs a function concurrently with the specified number of goroutines
func (ch *ConcurrentTestHelper) RunWithConcurrency(numRoutines int, fn func(int) error) []error {
	// Default timeout
	timeout := 5 * time.Second

	// Create a wait group to wait for all goroutines
	var wg sync.WaitGroup

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create error collection channel
	errorChan := make(chan error, numRoutines)

	// Run the function in multiple goroutines
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		goroutineID := i
		go func() {
			defer wg.Done()

			// Create a channel for this goroutine's result
			resultChan := make(chan error, 1)

			// Run the function in a goroutine
			go func() {
				resultChan <- fn(goroutineID)
			}()

			// Wait for either completion or timeout
			select {
			case err := <-resultChan:
				if err != nil {
					errorChan <- fmt.Errorf("goroutine %d: %w", goroutineID, err)
				}
			case <-ctx.Done():
				errorChan <- fmt.Errorf("goroutine %d: timed out", goroutineID)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	return errors
}

// RunWithConcurrencyAndTimeout runs a function concurrently with a specific timeout
func (ch *ConcurrentTestHelper) RunWithConcurrencyAndTimeout(numRoutines int, timeout time.Duration, fn func(int) error) []error {
	// Create a wait group to wait for all goroutines
	var wg sync.WaitGroup

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create error collection channel
	errorChan := make(chan error, numRoutines)

	// Run the function in multiple goroutines
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		goroutineID := i
		go func() {
			defer wg.Done()

			// Create a channel for this goroutine's result
			resultChan := make(chan error, 1)

			// Run the function in a goroutine
			go func() {
				resultChan <- fn(goroutineID)
			}()

			// Wait for either completion or timeout
			select {
			case err := <-resultChan:
				if err != nil {
					errorChan <- fmt.Errorf("goroutine %d: %w", goroutineID, err)
				}
			case <-ctx.Done():
				errorChan <- fmt.Errorf("goroutine %d: timed out", goroutineID)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	return errors
}

// RunParallelQueries runs database queries in parallel and collects the results
func (ch *ConcurrentTestHelper) RunParallelQueries(numQueries int, queryFn func(int, *DBTestContext) error) []error {
	return ch.RunWithConcurrency(numQueries, func(id int) error {
		return queryFn(id, ch.tc)
	})
}

// Concurrent returns a concurrent test helper for the test context
func (tc *DBTestContext) Concurrent() *ConcurrentTestHelper {
	return NewConcurrentTestHelper(tc)
}

// RunConcurrentTests runs a series of concurrent operation tests
func RunConcurrentTests(t *testing.T, testCases []ConcurrentTestCase, opts ...func(*Config)) {
	// Skip in short mode to avoid long tests
	if testing.Short() {
		t.Skip("Skipping concurrent tests in short mode")
		return
	}

	// Run each concurrent test - use index to avoid copying mutexes
	for i := range testCases {
		tc := &testCases[i] // Use a pointer to avoid copying the mutex
		t.Run(tc.Name, func(t *testing.T) {
			// Use a specific timeout for these tests
			timeout := 5 * time.Second
			if tc.timeout > 0 {
				timeout = tc.timeout
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Create a test context for this test with provided options
			testCtx := NewDBTestContext(t, opts...)
			defer testCtx.Teardown()

			// Create a wait group to wait for all goroutines
			var wg sync.WaitGroup

			// Pre-test setup if provided
			if tc.SetupMock != nil {
				for i := 0; i < tc.NumOperations; i++ {
					tc.SetupMock(testCtx, i)
				}
			}

			// Create error collection channel
			errorChan := make(chan error, tc.NumOperations)

			// Run test in N concurrent goroutines
			numRoutines := tc.NumOperations
			for i := 0; i < numRoutines; i++ {
				wg.Add(1)
				goroutineID := i
				go func() {
					defer wg.Done()
					localErrChan := make(chan error, 1)

					// Run with context to support timeout
					go func() {
						tc.ExecuteService(testCtx, goroutineID, localErrChan)
					}()

					// Wait for either completion or timeout
					select {
					case err := <-localErrChan:
						if err != nil {
							errorChan <- fmt.Errorf("goroutine %d: %w", goroutineID, err)
						}
					case <-ctx.Done():
						errorChan <- fmt.Errorf("goroutine %d: timed out", goroutineID)
					}
				}()
			}

			// Wait for all goroutines to complete
			wg.Wait()
			close(errorChan)

			// Check for errors
			var errors []error
			for err := range errorChan {
				errors = append(errors, err)
			}

			if len(errors) > 0 && !tc.ExpectErrors {
				for _, err := range errors {
					t.Errorf("Concurrent operation error: %v", err)
				}
			}

			// Verify mock expectations
			testCtx.VerifyExpectations()
		})
	}
}

// WithTimeout adds a timeout to the concurrent test case
func (tc *ConcurrentTestCase) WithTimeout(timeout time.Duration) *ConcurrentTestCase {
	tc.timeout = timeout
	return tc
}

// WithConcurrency sets the number of concurrent operations
func (tc *ConcurrentTestCase) WithConcurrency(n int) *ConcurrentTestCase {
	tc.NumOperations = n
	return tc
}

// WithErrorHandler adds a custom error handler to the concurrent test case
func (tc *ConcurrentTestCase) WithErrorHandler(handler func(int, error)) *ConcurrentTestCase {
	oldExecuteService := tc.ExecuteService
	tc.ExecuteService = func(ctx core.Context, id int, errChan chan error) {
		localErrChan := make(chan error, 1)
		oldExecuteService(ctx, id, localErrChan)

		if len(localErrChan) > 0 {
			err := <-localErrChan
			handler(id, err)
			errChan <- err
		}
	}
	return tc
}
