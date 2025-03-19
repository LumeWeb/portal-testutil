package testutil

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ExampleValidationTester demonstrates how to use the ValidationTester
func ExampleValidationTester() {
	// This is just an example and won't actually run

	// In your test:
	// t := &testing.T{}
	// validator := NewValidationTester(t)

	// Test a simple validation function
	// validator.AssertValidationError(validateUser(invalidUser), "username is required")

	// Test field validation
	// validator.AssertFieldValidation(validateUsername, "username", "validuser", "")

	// Test required field
	// validator.AssertRequiredField(validateUsername, "username")

	// Test string length
	// validator.AssertMinLength(validatePassword, "password", 8)
	// validator.AssertMaxLength(validateUsername, "username", 50)

	// Test email format
	// validator.AssertEmailFormat(validateEmail, "email")

	// Test numeric range
	// validator.AssertNumericRange(validateAge, "age", 18, 120)

	// Test enum values
	// validator.AssertEnumValue(validateRole, "role", []string{"admin", "user", "guest"})
}

// TestValidationTester demonstrates how to use the ValidationTester in a real test
func TestValidationTester(t *testing.T) {
	// Skip this test as it's just an example
	t.Skip("This is just an example test")

	// Create a validation tester
	validator := NewValidationTester(t)

	// Test a simple validation function
	validateUser := func(user interface{}) error {
		if user == nil {
			return errors.New("user is required")
		}
		return nil
	}

	validator.AssertValidationError(validateUser(nil), "user is required")
	validator.AssertNoValidationError(validateUser("valid user"))

	// Test field validation
	validateUsername := func(username interface{}) error {
		if username == nil || username.(string) == "" {
			return errors.New("username is required")
		}
		return nil
	}

	validator.AssertFieldValidation(validateUsername, "username", "validuser", "")

	// Test required field
	validator.AssertRequiredField(validateUsername, "username")

	// Test string length
	validatePassword := func(password string) error {
		if len(password) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}
		return nil
	}

	validator.AssertMinLength(validatePassword, "password", 8)

	validateDisplayName := func(name string) error {
		if len(name) > 50 {
			return fmt.Errorf("display name must be at most 50 characters")
		}
		return nil
	}

	validator.AssertMaxLength(validateDisplayName, "display name", 50)

	// Test email format
	validateEmail := func(email string) error {
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(email) {
			return fmt.Errorf("email has invalid format")
		}
		return nil
	}

	validator.AssertEmailFormat(validateEmail, "email")

	// Test numeric range
	validateAge := func(age int) error {
		if age < 18 {
			return fmt.Errorf("age must be at least 18")
		}
		if age > 120 {
			return fmt.Errorf("age must be at most 120")
		}
		return nil
	}

	validator.AssertNumericRange(validateAge, "age", 18, 120)

	// Test enum values
	validateRole := func(role string) error {
		allowedRoles := []string{"admin", "user", "guest"}
		for _, r := range allowedRoles {
			if role == r {
				return nil
			}
		}
		return fmt.Errorf("role must be one of: %s", strings.Join(allowedRoles, ", "))
	}

	validator.AssertEnumValue(validateRole, "role", []string{"admin", "user", "guest"})

	// Verify that assertions work
	assert.NotPanics(t, func() {
		validator.AssertValidationError(errors.New("validation error"), "validation")
	})

	assert.Panics(t, func() {
		validator.AssertValidationError(nil, "should fail")
	})
}
