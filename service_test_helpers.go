package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

// ServiceTestCase represents a test case for a service
type ServiceTestCase struct {
	Name          string
	SetupMock     func(*DBTestContext)
	ExecuteTest   func(core.Context) (interface{}, error)
	ExpectResult  interface{}
	ExpectError   bool
	ErrorContains string
	Pagination    queryutil.Pagination // Optional pagination for the test case
}

// ServiceTestOptions contains options for running service tests
type ServiceTestOptions struct {
	Config     *Config              // Configuration options for the test context
	Pagination queryutil.Pagination // Default pagination for all tests
}

// RunServiceTests runs a set of service test cases
func RunServiceTests(t *testing.T, testCases []ServiceTestCase, opts ...func(*ServiceTestOptions)) {
	// Apply default options
	options := &ServiceTestOptions{
		Config:     DefaultConfig(),
		Pagination: NewPaginationHelper().DefaultPagination(),
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(options)
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Create a test context with the configured options
			var configOpts []func(*Config)
			if options.Config.TablePrefix != "" {
				configOpts = append(configOpts, WithTablePrefix(options.Config.TablePrefix))
			}

			testCtx := NewDBTestContext(t, configOpts...)
			defer testCtx.Teardown()

			// Configure mocks
			if tc.SetupMock != nil {
				tc.SetupMock(testCtx)
			}

			// Execute the test
			result, err := tc.ExecuteTest(testCtx)

			// Verify expectations
			if tc.ExpectError {
				assert.Error(t, err)
				if tc.ErrorContains != "" {
					assert.Contains(t, err.Error(), tc.ErrorContains)
				}
			} else {
				assert.NoError(t, err)
				if tc.ExpectResult != nil {
					assert.Equal(t, tc.ExpectResult, result)
				}
			}

			// Verify SQL expectations
			testCtx.VerifyExpectations()
		})
	}
}

// WithTestTablePrefix sets the table prefix for all tests
func WithTestTablePrefix(prefix string) func(*ServiceTestOptions) {
	return func(o *ServiceTestOptions) {
		o.Config.TablePrefix = prefix
	}
}

// WithTestPagination sets the default pagination for all tests
func WithTestPagination(page, pageSize int) func(*ServiceTestOptions) {
	return func(o *ServiceTestOptions) {
		paginator := NewPaginationHelper()
		o.Pagination = paginator.CreatePagination(page, pageSize)
	}
}

// WithPagination adds pagination to the service test case
func (tc *ServiceTestCase) WithPagination(page, pageSize int) *ServiceTestCase {
	paginator := NewPaginationHelper()
	tc.Pagination = paginator.CreatePagination(page, pageSize)
	return tc
}
