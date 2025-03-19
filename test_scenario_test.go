package testutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockTestRunner is a helper to capture test results
type MockTestRunner struct {
	t          *testing.T
	failCalled bool
	messages   []string
}

// Fail marks the test as failed
func (m *MockTestRunner) Fail() {
	m.failCalled = true
}

// Log records a log message
func (m *MockTestRunner) Log(args ...interface{}) {
	for _, arg := range args {
		m.messages = append(m.messages, arg.(string))
	}
}

// Logf records a formatted log message
func (m *MockTestRunner) Logf(format string, args ...interface{}) {
	m.messages = append(m.messages, format)
}

// Run runs a subtest
func (m *MockTestRunner) Run(name string, f func(t *testing.T)) bool {
	f(m.t)
	return !m.failCalled
}

func TestRunTestScenario_Success(t *testing.T) {
	// Create a test scenario
	scenario := TestScenario{
		Name:        "Successful Scenario",
		Description: "A test scenario that succeeds",
		SetupMock: func(tc *DBTestContext) {
			// No setup required for this test
		},
		Steps: []TestStep{
			{
				Name: "Do something",
				ExecuteTest: func(ctx interface{}) (interface{}, error) {
					return "success", nil
				},
				ExpectResult: "success",
				ExpectError:  false,
			},
			{
				Name: "Do something else",
				ExecuteTest: func(ctx interface{}) (interface{}, error) {
					return 42, nil
				},
				ExpectResult: 42,
				ExpectError:  false,
			},
		},
		TearDown: func(tc *DBTestContext) {
			// No teardown required for this test
		},
	}

	// Run the scenario
	RunTestScenario(t, scenario)
}

func TestRunTestScenario_WithError(t *testing.T) {
	// Create a test scenario with an error
	scenario := TestScenario{
		Name:        "Error Scenario",
		Description: "A test scenario with an expected error",
		Steps: []TestStep{
			{
				Name: "Do something that succeeds",
				ExecuteTest: func(ctx interface{}) (interface{}, error) {
					return "success", nil
				},
				ExpectResult: "success",
				ExpectError:  false,
			},
			{
				Name: "Do something that fails",
				ExecuteTest: func(ctx interface{}) (interface{}, error) {
					return nil, errors.New("expected error")
				},
				ExpectError:   true,
				ErrorContains: "expected error",
			},
		},
	}

	// Run the scenario
	RunTestScenario(t, scenario)
}

func TestRunTestScenario_WithSetupAndTeardown(t *testing.T) {
	// Track whether setup and teardown were called
	setupCalled := false
	teardownCalled := false

	// Create a test scenario with setup and teardown
	scenario := TestScenario{
		Name: "Setup and Teardown Scenario",
		SetupMock: func(tc *DBTestContext) {
			setupCalled = true
		},
		Steps: []TestStep{
			{
				Name: "Single step",
				ExecuteTest: func(ctx interface{}) (interface{}, error) {
					return "success", nil
				},
				ExpectResult: "success",
			},
		},
		TearDown: func(tc *DBTestContext) {
			teardownCalled = true
		},
	}

	// Run the scenario
	RunTestScenario(t, scenario)

	// Verify setup and teardown were called
	assert.True(t, setupCalled, "Setup function should have been called")
	assert.True(t, teardownCalled, "Teardown function should have been called")
}

func TestRunTestScenario_WithTablePrefix(t *testing.T) {
	// Create a test scenario with a table prefix
	prefix := "test_"
	prefixCaptured := ""

	scenario := TestScenario{
		Name: "Table Prefix Scenario",
		Steps: []TestStep{
			{
				Name: "Check table prefix",
				ExecuteTest: func(ctx interface{}) (interface{}, error) {
					tc, ok := ctx.(*DBTestContext)
					if !ok {
						return nil, errors.New("context is not a DBTestContext")
					}
					prefixCaptured = tc.config.TablePrefix
					return prefixCaptured, nil
				},
				ExpectResult: prefix,
			},
		},
	}

	// Run the scenario with a table prefix option
	RunTestScenario(t, scenario, WithTablePrefix(prefix))

	// Verify the prefix was set correctly
	assert.Equal(t, prefix, prefixCaptured)
}
