package testutil

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/queryutil"
)

func TestSearchTestHelper_ExpectSearchWithQueryUtil(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")
	rowBuilder.AddRow(2, "Bob", "bob@example.com")

	// Create test filters
	filters := []queryutil.Filter{
		{
			Field:    "name",
			Operator: queryutil.OperatorContains,
			Value:    "a",
		},
	}

	// Create test sorts
	sorts := []queryutil.Sort{
		{
			Field: "name",
			Order: queryutil.OrderAsc,
		},
	}

	// Create test pagination
	pagination := queryutil.Pagination{
		Start:    0,
		PageSize: 10,
	}

	// Set up search expectations
	helper.ExpectSearchWithQueryUtil("users", filters, sorts, pagination, 2, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearch(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")
	rowBuilder.AddRow(2, "Bob", "bob@example.com")

	// Set up search expectations
	helper.ExpectSearch("users", "a", 2, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchWithFilters(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")

	// Create test filters
	filters := []queryutil.Filter{
		{
			Field:    "name",
			Operator: queryutil.OperatorEquals,
			Value:    "Alice",
		},
	}

	// Set up search expectations
	helper.ExpectSearchWithFilters("users", "a", filters, 1, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchError(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Set up search error expectation
	expectedErr := errors.New("database error")
	helper.ExpectSearchError("users", "a", expectedErr)

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectTextSearch(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")

	// Set up text search expectations
	helper.ExpectTextSearch("users", []string{"name", "email"}, "al", 1, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectGlobalSearch(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows for multiple tables
	usersRowBuilder := NewRowBuilder("id", "name")
	usersRowBuilder.AddRow(1, "Alice")

	postsRowBuilder := NewRowBuilder("id", "title")
	postsRowBuilder.AddRow(1, "My Post")

	// Set up global search expectations
	tables := []string{"users", "posts"}
	counts := []int64{1, 1}
	rowsList := []*sqlmock.Rows{usersRowBuilder.Build(), postsRowBuilder.Build()}

	helper.ExpectGlobalSearch(tables, "al", counts, rowsList)

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchWithPagination(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")
	rowBuilder.AddRow(2, "Bob", "bob@example.com")

	// Create test pagination
	pagination := queryutil.Pagination{
		Start:    0,
		PageSize: 10,
	}

	// Set up search expectations with pagination
	helper.ExpectSearchWithPagination("users", "a", pagination, 2, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchWithSorting(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")
	rowBuilder.AddRow(2, "Bob", "bob@example.com")

	// Create test sorts
	sorts := []queryutil.Sort{
		{
			Field: "name",
			Order: queryutil.OrderAsc,
		},
	}

	// Set up search expectations with sorting
	helper.ExpectSearchWithSorting("users", "a", sorts, 2, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchWithGlobalSearch(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")

	// Create global search config
	searchConfig := &queryutil.GlobalSearchConfig{
		SearchableColumns: []string{"name", "email"},
	}

	// Set up search expectations with global search config
	helper.ExpectSearchWithGlobalSearch("users", "al", searchConfig, 1, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchCount(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Create test filters
	filters := []queryutil.Filter{
		{
			Field:    "name",
			Operator: queryutil.OperatorContains,
			Value:    "a",
		},
	}

	// Set up search count expectation
	helper.ExpectSearchCount("users", filters, 2)

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_ExpectSearchRows(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Build test rows
	rowBuilder := NewRowBuilder("id", "name", "email")
	rowBuilder.AddRow(1, "Alice", "alice@example.com")
	rowBuilder.AddRow(2, "Bob", "bob@example.com")

	// Create test filters
	filters := []queryutil.Filter{
		{
			Field:    "name",
			Operator: queryutil.OperatorContains,
			Value:    "a",
		},
	}

	// Create test sorts
	sorts := []queryutil.Sort{
		{
			Field: "name",
			Order: queryutil.OrderAsc,
		},
	}

	// Create test pagination
	pagination := queryutil.Pagination{
		Start:    0,
		PageSize: 10,
	}

	// Set up search rows expectation
	helper.ExpectSearchRows("users", filters, sorts, pagination, rowBuilder.Build())

	// Verify expectations
	ctx.VerifyExpectations()
}

func TestSearchTestHelper_BuildWhereClauseFromFilters(t *testing.T) {
	// Create a test context
	ctx := NewDBTestContext(t)

	// Create a search helper
	helper := ctx.Search()

	// Test with no filters
	whereClause := helper.buildWhereClauseFromFilters(nil)
	assert.Equal(t, "1=1", whereClause)

	// Test with filters
	filters := []queryutil.Filter{
		{
			Field:    "name",
			Operator: queryutil.OperatorEquals,
			Value:    "Alice",
		},
		{
			Field:    "age",
			Operator: queryutil.OperatorGTE,
			Value:    18,
		},
	}

	whereClause = helper.buildWhereClauseFromFilters(filters)
	assert.Equal(t, "name = 'Alice' AND age >= 18", whereClause)
}
