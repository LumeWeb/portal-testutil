package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

// TestCase represents a simple model for testing filter arguments
type FilterTestCase struct {
	gorm.Model
	ReferenceNumber string
	Type            string
	Status          string
}

// TestFilterArgsBugFix demonstrates the fix for the issue with portal-testutil
// and filter arguments. We've added a WithArgs method to the CountExpectationBuilder
// and FindExpectationBuilder that allows specifying exact argument matching.
func TestFilterArgsBugFix(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register model
	RegisterModelWithRelationships[FilterTestCase](tc)

	// Set up test data
	now := time.Now()
	filteredCase := FilterTestCase{
		Model:           gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		ReferenceNumber: "CASE-123",
		Type:            "spam",
		Status:          "new",
	}

	// The fix allows specifying exact arguments to match
	tc.ForTable("filter_test_cases").
		ExpectCount().
		Where("type = ?").
		WithArgs("spam"). // This is the key fix - we can now explicitly specify args
		ReturnCount(1)

	// Also works with Find operations
	tc.ForTable("filter_test_cases").
		ExpectFind().
		Where("type = ?").
		WithArgs("spam"). // And here too
		ReturnModels([]FilterTestCase{filteredCase})

	// Execute a query that has WHERE parameters
	db := tc.DB()
	filters := []queryutil.Filter{
		{
			Field:    "type",
			Operator: queryutil.OperatorEquals,
			Value:    "spam",
		},
	}

	// Apply filters to query
	var count int64
	query := db.Table("filter_test_cases")
	query = queryutil.ApplyFilters(query, filters, nil)

	// This now works because we've explicitly specified the expected argument
	err := query.Count(&count).Error
	assert.NoError(t, err, "Count query should match the expectation")
	assert.Equal(t, int64(1), count)

	// This also works for the same reason
	var cases []FilterTestCase
	err = query.Find(&cases).Error
	assert.NoError(t, err, "Find query should match the expectation")
	assert.Equal(t, 1, len(cases))
}

// TestFilterArgsWithDifferentOperators verifies our WithArgs method works with different
// filter operators in queryutil.ApplyFilters
func TestFilterArgsWithDifferentOperators(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register model
	RegisterModelWithRelationships[FilterTestCase](tc)

	// Set up test data
	now := time.Now()
	testCase := FilterTestCase{
		Model:           gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		ReferenceNumber: "CASE-123",
		Type:            "spam",
		Status:          "active",
	}

	// Test each operator type

	// 1. OperatorEquals
	tc.ForTable("filter_test_cases").
		ExpectFind().
		Where("type = ?").
		WithArgs("spam").
		ReturnModels([]FilterTestCase{testCase})

	// 2. OperatorNotEquals
	tc.ForTable("filter_test_cases").
		ExpectFind().
		Where("type <> ?").
		WithArgs("ham").
		ReturnModels([]FilterTestCase{testCase})

	// 3. OperatorGTE
	tc.ForTable("filter_test_cases").
		ExpectFind().
		Where("reference_number >= ?").
		WithArgs("CASE-100").
		ReturnModels([]FilterTestCase{testCase})

	// 4. OperatorLTE
	tc.ForTable("filter_test_cases").
		ExpectFind().
		Where("reference_number <= ?").
		WithArgs("CASE-200").
		ReturnModels([]FilterTestCase{testCase})

	// 5. OperatorContains (uses LIKE)
	tc.ForTable("filter_test_cases").
		ExpectFind().
		Where("reference_number LIKE ?").
		WithArgs("%123%").
		ReturnModels([]FilterTestCase{testCase})

	// Apply each filter and verify
	db := tc.DB()

	// Test OperatorEquals
	query := db.Table("filter_test_cases")
	query = queryutil.ApplyFilters(query, []queryutil.Filter{{
		Field:    "type",
		Operator: queryutil.OperatorEquals,
		Value:    "spam",
	}}, nil)
	var cases []FilterTestCase
	err := query.Find(&cases).Error
	assert.NoError(t, err, "OperatorEquals: Find query should match")
	assert.Equal(t, 1, len(cases))

	// Test OperatorNotEquals
	query = db.Table("filter_test_cases")
	query = queryutil.ApplyFilters(query, []queryutil.Filter{{
		Field:    "type",
		Operator: queryutil.OperatorNotEquals,
		Value:    "ham",
	}}, nil)
	cases = nil
	err = query.Find(&cases).Error
	assert.NoError(t, err, "OperatorNotEquals: Find query should match")
	assert.Equal(t, 1, len(cases))

	// Test OperatorGTE
	query = db.Table("filter_test_cases")
	query = queryutil.ApplyFilters(query, []queryutil.Filter{{
		Field:    "reference_number",
		Operator: queryutil.OperatorGTE,
		Value:    "CASE-100",
	}}, nil)
	cases = nil
	err = query.Find(&cases).Error
	assert.NoError(t, err, "OperatorGTE: Find query should match")
	assert.Equal(t, 1, len(cases))

	// Test OperatorLTE
	query = db.Table("filter_test_cases")
	query = queryutil.ApplyFilters(query, []queryutil.Filter{{
		Field:    "reference_number",
		Operator: queryutil.OperatorLTE,
		Value:    "CASE-200",
	}}, nil)
	cases = nil
	err = query.Find(&cases).Error
	assert.NoError(t, err, "OperatorLTE: Find query should match")
	assert.Equal(t, 1, len(cases))

	// Test OperatorContains
	query = db.Table("filter_test_cases")
	query = queryutil.ApplyFilters(query, []queryutil.Filter{{
		Field:    "reference_number",
		Operator: queryutil.OperatorContains,
		Value:    "123",
	}}, nil)
	cases = nil
	err = query.Find(&cases).Error
	assert.NoError(t, err, "OperatorContains: Find query should match")
	assert.Equal(t, 1, len(cases))
}

// The bug was that portal-testutil v0.2.11's high-level ExpectCount() and ExpectFind() abstractions
// didn't provide a way to specify SQL query arguments. So when GORM generated a query with a WHERE
// clause and parameters, the expectation didn't match what was actually executed.
//
// The solution is to add a WithArgs() method to the expectation builders that allows specifying
// exact arguments to match, which is particularly useful when testing with queryutil.Filter
// or any other code that generates SQL with placeholders and arguments.
