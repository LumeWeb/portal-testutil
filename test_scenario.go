package testutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScenario represents a multi-step test scenario
type TestScenario struct {
	Name        string
	Description string
	SetupMock   func(*DBTestContext)
	Steps       []TestStep
	TearDown    func(*DBTestContext)
}

// TestStep represents a single step in a test scenario
type TestStep struct {
	Name          string
	ExecuteTest   func(interface{}) (interface{}, error) // Changed to interface{} to avoid core.Context dependency
	ExpectResult  interface{}
	ExpectError   bool
	ErrorContains string
}

// RunTestScenario runs a multi-step test scenario
func RunTestScenario(t *testing.T, scenario TestScenario, opts ...func(*Config)) {
	t.Run(scenario.Name, func(t *testing.T) {
		if scenario.Description != "" {
			t.Logf("Description: %s", scenario.Description)
		}

		// Create a test context with the provided options
		testCtx := NewDBTestContext(t, opts...)
		defer func() {
			// Run teardown if provided
			if scenario.TearDown != nil {
				scenario.TearDown(testCtx)
			}
			testCtx.Teardown()
		}()

		// Setup the mocks
		if scenario.SetupMock != nil {
			scenario.SetupMock(testCtx)
		}

		// Run each step
		for i, step := range scenario.Steps {
			stepName := fmt.Sprintf("Step%d %s", i+1, step.Name)
			t.Run(stepName, func(t *testing.T) {
				// Execute the test
				result, err := step.ExecuteTest(testCtx)

				// Verify expectations
				if step.ExpectError {
					assert.Error(t, err)
					if step.ErrorContains != "" {
						assert.Contains(t, err.Error(), step.ErrorContains)
					}
				} else {
					assert.NoError(t, err)
					if step.ExpectResult != nil {
						assert.Equal(t, step.ExpectResult, result)
					}
				}
			})
		}

		// Verify all expectations were met
		testCtx.VerifyExpectations()
	})
}
