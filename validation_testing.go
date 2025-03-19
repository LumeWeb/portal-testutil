package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ValidationTester provides utilities for testing validation logic in a structured way.
//
// It offers methods to test various aspects of validation including:
// - Required fields
// - String length constraints (min/max)
// - Email format validation
// - URL format validation
// - Numeric range validation
// - Enum value validation
// - Date format validation
// - JSON format validation
//
// Each validation method takes a validator function that is specific to the validation
// being tested, making it easy to test individual validation rules in isolation.
type ValidationTester struct {
	t *testing.T // The testing.T instance for making assertions
}

// NewValidationTester creates a new validation tester with the given testing.T instance.
//
// Example:
//
//	// Create directly
//	validator := testutil.NewValidationTester(t)
//
//	// Or through a test context
//	validator := testCtx.Validation()
func NewValidationTester(t *testing.T) *ValidationTester {
	return &ValidationTester{t: t}
}

// AssertValidationError asserts that the given error is a validation error
// containing the expected message
func (v *ValidationTester) AssertValidationError(err error, expectedMsg string) {
	assert.Error(v.t, err, "Expected a validation error")
	assert.Contains(v.t, err.Error(), expectedMsg, "Error message should contain expected text")
}

// AssertNoValidationError asserts that no validation error occurred
func (v *ValidationTester) AssertNoValidationError(err error) {
	assert.NoError(v.t, err, "Expected no validation error")
}

// AssertFieldValidation tests validation for a specific field
func (v *ValidationTester) AssertFieldValidation(validator func(interface{}) error,
	field string, validValue, invalidValue interface{}) {

	// Test with valid value
	err := validator(validValue)
	assert.NoError(v.t, err, "Validation should pass with valid value")

	// Test with invalid value
	err = validator(invalidValue)
	assert.Error(v.t, err, "Validation should fail with invalid value")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")
}

// AssertRequiredField tests that a field is required
func (v *ValidationTester) AssertRequiredField(validator func(interface{}) error, field string) {
	err := validator(nil)
	assert.Error(v.t, err, "Validation should fail when required field is nil")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")
	assert.Contains(v.t, err.Error(), "required", "Error should indicate field is required")
}

// AssertMinLength tests that a string field has a minimum length
func (v *ValidationTester) AssertMinLength(validator func(string) error, field string, minLength int) {
	// Test with a string that's too short
	tooShort := ""
	for i := 0; i < minLength-1; i++ {
		tooShort += "a"
	}

	err := validator(tooShort)
	assert.Error(v.t, err, "Validation should fail when string is too short")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")

	// Test with a string of exactly the minimum length
	exactLength := ""
	for i := 0; i < minLength; i++ {
		exactLength += "a"
	}

	err = validator(exactLength)
	assert.NoError(v.t, err, "Validation should pass when string is exactly the minimum length")

	// Test with a longer string
	longer := exactLength + "aaa"
	err = validator(longer)
	assert.NoError(v.t, err, "Validation should pass when string is longer than minimum")
}

// AssertMaxLength tests that a string field has a maximum length
func (v *ValidationTester) AssertMaxLength(validator func(string) error, field string, maxLength int) {
	// Test with a string that's too long
	tooLong := ""
	for i := 0; i < maxLength+1; i++ {
		tooLong += "a"
	}

	err := validator(tooLong)
	assert.Error(v.t, err, "Validation should fail when string is too long")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")

	// Test with a string of exactly the maximum length
	exactLength := ""
	for i := 0; i < maxLength; i++ {
		exactLength += "a"
	}

	err = validator(exactLength)
	assert.NoError(v.t, err, "Validation should pass when string is exactly the maximum length")

	// Test with a shorter string
	shorter := exactLength[:maxLength/2]
	err = validator(shorter)
	assert.NoError(v.t, err, "Validation should pass when string is shorter than maximum")
}

// AssertEmailFormat tests that a string field has a valid email format
func (v *ValidationTester) AssertEmailFormat(validator func(string) error, field string) {
	// Test with invalid email formats
	invalidEmails := []string{
		"",
		"notanemail",
		"missing@tld",
		"@missingname.com",
		"spaces in@email.com",
		"missing.domain@",
	}

	for _, email := range invalidEmails {
		err := validator(email)
		assert.Error(v.t, err, "Validation should fail with invalid email: %s", email)
		assert.Contains(v.t, err.Error(), field, "Error should mention the field name")
	}

	// Test with valid email formats
	validEmails := []string{
		"test@example.com",
		"user.name@example.co.uk",
		"user+tag@example.org",
		"123@numbers.com",
	}

	for _, email := range validEmails {
		err := validator(email)
		assert.NoError(v.t, err, "Validation should pass with valid email: %s", email)
	}
}

// AssertNumericRange tests that a numeric field is within a specified range
func (v *ValidationTester) AssertNumericRange(validator func(int) error, field string, min, max int) {
	// Test with value below minimum
	err := validator(min - 1)
	assert.Error(v.t, err, "Validation should fail when value is below minimum")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")

	// Test with value above maximum
	err = validator(max + 1)
	assert.Error(v.t, err, "Validation should fail when value is above maximum")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")

	// Test with minimum value
	err = validator(min)
	assert.NoError(v.t, err, "Validation should pass when value is at minimum")

	// Test with maximum value
	err = validator(max)
	assert.NoError(v.t, err, "Validation should pass when value is at maximum")

	// Test with value in the middle of the range
	middle := (min + max) / 2
	err = validator(middle)
	assert.NoError(v.t, err, "Validation should pass when value is in the middle of the range")
}

// AssertEnumValue tests that a string field contains one of the allowed values
func (v *ValidationTester) AssertEnumValue(validator func(string) error, field string, allowedValues []string) {
	// Test with invalid value
	err := validator("invalid_value")
	assert.Error(v.t, err, "Validation should fail with invalid enum value")
	assert.Contains(v.t, err.Error(), field, "Error should mention the field name")

	// Test with each allowed value
	for _, value := range allowedValues {
		err = validator(value)
		assert.NoError(v.t, err, "Validation should pass with allowed value: %s", value)
	}
}

// AssertURLFormat tests that a string field has a valid URL format
func (v *ValidationTester) AssertURLFormat(validator func(string) error, field string) {
	// Test with invalid URL formats
	invalidURLs := []string{
		"",
		"notaurl",
		"http://",
		"ftp://invalid",
		"www.example.com",            // Missing scheme
		"http:/example.com",          // Missing slash
		"http://example.com:invalid", // Invalid port
	}

	for _, url := range invalidURLs {
		err := validator(url)
		assert.Error(v.t, err, "Validation should fail with invalid URL: %s", url)
		assert.Contains(v.t, err.Error(), field, "Error should mention the field name")
	}

	// Test with valid URL formats
	validURLs := []string{
		"http://example.com",
		"https://example.com",
		"http://www.example.com/path",
		"https://example.com:8080/path?query=value",
		"http://example.com/path#fragment",
	}

	for _, url := range validURLs {
		err := validator(url)
		assert.NoError(v.t, err, "Validation should pass with valid URL: %s", url)
	}
}

// AssertJSONFormat tests that a string field has a valid JSON format
func (v *ValidationTester) AssertJSONFormat(validator func(string) error, field string) {
	// Test with invalid JSON formats
	invalidJSON := []string{
		"",
		"{",
		"}",
		"{\"key\": value}", // Missing quotes around value
		"[1, 2,]",          // Trailing comma
		"{key: \"value\"}", // Missing quotes around key
	}

	for _, json := range invalidJSON {
		err := validator(json)
		assert.Error(v.t, err, "Validation should fail with invalid JSON: %s", json)
		assert.Contains(v.t, err.Error(), field, "Error should mention the field name")
	}

	// Test with valid JSON formats
	validJSON := []string{
		"{}",
		"[]",
		"{\"key\": \"value\"}",
		"{\"key\": 123}",
		"{\"key\": true}",
		"{\"key\": null}",
		"[1, 2, 3]",
		"{\"key\": [1, 2, 3]}",
		"{\"key\": {\"nested\": \"value\"}}",
	}

	for _, json := range validJSON {
		err := validator(json)
		assert.NoError(v.t, err, "Validation should pass with valid JSON: %s", json)
	}
}

// AssertDateFormat tests that a string field has a valid date format
func (v *ValidationTester) AssertDateFormat(validator func(string) error, field string, format string) {
	// Test with invalid date formats
	invalidDates := []string{
		"",
		"notadate",
		"2023/13/01", // Invalid month
		"2023/01/32", // Invalid day
		"01/01/2023", // Wrong format if expecting YYYY-MM-DD
	}

	for _, date := range invalidDates {
		err := validator(date)
		assert.Error(v.t, err, "Validation should fail with invalid date: %s", date)
		assert.Contains(v.t, err.Error(), field, "Error should mention the field name")
	}

	// Test with valid date formats based on the expected format
	var validDates []string
	switch format {
	case "YYYY-MM-DD":
		validDates = []string{
			"2023-01-01",
			"2023-12-31",
			"2000-02-29", // Leap year
		}
	case "MM/DD/YYYY":
		validDates = []string{
			"01/01/2023",
			"12/31/2023",
			"02/29/2000", // Leap year
		}
	default:
		// Add some ISO format dates as default
		validDates = []string{
			"2023-01-01",
			"2023-01-01T00:00:00Z",
			"2023-01-01T00:00:00+00:00",
		}
	}

	for _, date := range validDates {
		err := validator(date)
		assert.NoError(v.t, err, "Validation should pass with valid date: %s", date)
	}
}
