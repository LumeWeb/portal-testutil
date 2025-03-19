package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDBTestContext(t *testing.T) {
	// Create a new test context
	tc := NewDBTestContext(t)

	// Verify the context was created successfully
	assert.NotNil(t, tc)
	assert.NotNil(t, tc.TestContext)
	assert.NotNil(t, tc.mock)
	assert.NotNil(t, tc.config)

	// Verify default configuration
	assert.Equal(t, "", tc.config.TablePrefix)
}

func TestWithTablePrefix(t *testing.T) {
	// Create a test context with table prefix
	prefix := "test_"
	tc := NewDBTestContext(t, WithTablePrefix(prefix))

	// Verify the prefix was set correctly
	assert.Equal(t, prefix, tc.config.TablePrefix)
}

func TestRegisterService(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Create a mock service from our test file
	svc := &MockService{}

	// Mock the ID method
	svc.mockObj.On("ID").Return("test-service")

	// Register the mock service
	tc.RegisterService("test-service", svc)

	// Verification is implicit (no easy way to check registered services)
	// If the method doesn't panic, we consider it successful
}

func TestVerifyExpectations(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Add a simple expectation
	tc.mock.ExpectBegin()

	// Satisfy the expectation
	tc.DB().Begin()

	// Verify expectations
	tc.VerifyExpectations()

	// No assertions needed, as VerifyExpectations would fail the test
	// if expectations weren't met
}

func TestForTable(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Create an expectations builder for a table
	builder := tc.ForTable("users")

	// Verify the builder was created
	assert.NotNil(t, builder)
	assert.Equal(t, "users", builder.table)
}

func TestRaw(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Get the raw SQL mock
	raw := tc.Raw()

	// Verify it's the same mock
	assert.Equal(t, tc.mock, raw)
}

func TestDB(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Get the DB connection
	db := tc.DB()

	// Verify it's not nil
	assert.NotNil(t, db)
}
