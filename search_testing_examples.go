package testutil

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/queryutil"
)

// ExampleSearchTestHelper demonstrates how to use the SearchTestHelper
func ExampleSearchTestHelper() {
	// This is just an example and won't actually run

	// In your test:
	// t := &testing.T{}
	// tc := NewDBTestContext(t)
	// defer tc.Teardown()

	// Create a search test helper
	// searchHelper := NewSearchTestHelper(tc)

	// Create a row builder for search results
	// builder := NewRowBuilder("id", "name", "email")
	// builder.AddRow(1, "John Doe", "john@example.com")
	// builder.AddRow(2, "Jane Smith", "jane@example.com")
	// rows := builder.Build()

	// Set up expectations for a basic search
	// searchHelper.ExpectSearch("users", "john", 1, rows)

	// Set up expectations for a search with filters
	// filters := []queryutil.Filter{
	//     {Field: "role", Operator: queryutil.OperatorEquals, Value: "admin"},
	// }
	// searchHelper.ExpectSearchWithFilters("users", "john", filters, 1, rows)

	// Set up expectations for a search error
	// searchHelper.ExpectSearchError("users", "john", errors.New("database error"))

	// Set up expectations for a text search
	// searchFields := []string{"name", "email"}
	// searchHelper.ExpectTextSearch("users", searchFields, "john", 1, rows)

	// Set up expectations for a global search
	// tables := []string{"users", "posts"}
	// counts := []int64{1, 2}
	// rowsList := []*sqlmock.Rows{rows, rows}
	// searchHelper.ExpectGlobalSearch(tables, "john", counts, rowsList)

	// Set up expectations for a search with pagination
	// pagination := queryutil.Pagination{Start: 0, End: 10, PageSize: 10}
	// searchHelper.ExpectSearchWithPagination("users", "john", pagination, 1, rows)

	// Set up expectations for a search with sorting
	// sorts := []queryutil.Sort{
	//     {Field: "name", Order: queryutil.OrderAsc},
	// }
	// searchHelper.ExpectSearchWithSorting("users", "john", sorts, 1, rows)
}

// TestSearchTestHelper demonstrates how to use the SearchTestHelper in a real test
func TestSearchTestHelper(t *testing.T) {
	// Skip this test as it's just an example
	t.Skip("This is just an example test")

	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create a search test helper
	searchHelper := NewSearchTestHelper(tc)

	// Create a row builder for search results
	builder := NewRowBuilder("id", "name", "email")
	builder.AddRow(1, "John Doe", "john@example.com")
	rows := builder.Build()

	// Set up expectations for a basic search
	searchHelper.ExpectSearch("users", "john", 1, rows)

	// Example service function that would use search
	searchUsers := func(query string) ([]map[string]interface{}, int64, error) {
		// In a real service, this would query the database
		// For this example, we'll just return mock data
		return []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
		}, 1, nil
	}

	// Test the service function
	users, total, err := searchUsers("john")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(t, "John Doe", users[0]["name"])

	// Test with error
	searchHelper.ExpectSearchError("users", "error", errors.New("database error"))

	searchWithError := func(query string) ([]map[string]interface{}, int64, error) {
		if query == "error" {
			return nil, 0, errors.New("database error")
		}
		return nil, 0, nil
	}

	_, _, err = searchWithError("error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")

	// Test with pagination
	pagination := queryutil.Pagination{Start: 0, End: 10, PageSize: 10}
	searchHelper.ExpectSearchWithPagination("users", "john", pagination, 1, rows)

	searchWithPagination := func(query string, pagination queryutil.Pagination) ([]map[string]interface{}, int64, error) {
		// In a real service, this would query the database with pagination
		return []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
		}, 1, nil
	}

	users, total, err = searchWithPagination("john", pagination)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)

	// Test with sorting
	sorts := []queryutil.Sort{
		{Field: "name", Order: queryutil.OrderAsc},
	}
	searchHelper.ExpectSearchWithSorting("users", "john", sorts, 1, rows)

	searchWithSorting := func(query string, sorts []queryutil.Sort) ([]map[string]interface{}, int64, error) {
		// In a real service, this would query the database with sorting
		return []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
		}, 1, nil
	}

	users, total, err = searchWithSorting("john", sorts)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)

	// Test with text search
	searchFields := []string{"name", "email"}
	searchHelper.ExpectTextSearch("users", searchFields, "john", 1, rows)

	textSearch := func(fields []string, term string) ([]map[string]interface{}, int64, error) {
		// In a real service, this would query the database with a text search
		return []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
		}, 1, nil
	}

	users, total, err = textSearch(searchFields, "john")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)

	// Test with global search
	tables := []string{"users", "posts"}
	counts := []int64{1, 2}

	postRows := sqlmock.NewRows([]string{"id", "title", "content"}).
		AddRow(1, "Test Post", "This is a test post").
		AddRow(2, "Another Post", "This is another test post")

	rowsList := []*sqlmock.Rows{rows, postRows}
	searchHelper.ExpectGlobalSearch(tables, "test", counts, rowsList)

	globalSearch := func(term string) map[string]interface{} {
		// In a real service, this would query multiple tables
		return map[string]interface{}{
			"users": []map[string]interface{}{
				{"id": 1, "name": "John Doe", "email": "john@example.com"},
			},
			"posts": []map[string]interface{}{
				{"id": 1, "title": "Test Post", "content": "This is a test post"},
				{"id": 2, "title": "Another Post", "content": "This is another test post"},
			},
		}
	}

	results := globalSearch("test")
	assert.NotNil(t, results)
	assert.Len(t, results["users"].([]map[string]interface{}), 1)
	assert.Len(t, results["posts"].([]map[string]interface{}), 2)
}
