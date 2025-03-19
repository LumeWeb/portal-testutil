package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/queryutil"
)

// ExamplePaginationHelper demonstrates how to use the PaginationHelper
func ExamplePaginationHelper() {
	// This is just an example and won't run in regular tests

	// Create a pagination helper
	_ = NewPaginationHelper()

	// Example methods:
	// paginator.DefaultPagination()
	// paginator.CreatePagination(2, 10)
	// paginator.FirstPage(10)
	// paginator.NextPage(firstPage)
	// paginator.PreviousPage(nextPage)
	// paginator.CalculateTotalPages(95, 10)
	// paginator.IsLastPage(page2, 15)
	// paginator.GetPageNumber(page2)
}

// TestPaginationHelper demonstrates how to use the PaginationHelper in a real test
func TestPaginationHelper(t *testing.T) {
	// Skip this test as it's just an example
	t.Skip("This is just an example test")

	// Create a pagination helper
	paginator := NewPaginationHelper()

	// Test default pagination
	defaultPagination := paginator.DefaultPagination()
	assert.Equal(t, 0, defaultPagination.Start)
	assert.Equal(t, 100, defaultPagination.End)
	assert.Equal(t, 100, defaultPagination.PageSize)

	// Test creating pagination for a specific page
	page2 := paginator.CreatePagination(2, 10)
	assert.Equal(t, 10, page2.Start)
	assert.Equal(t, 20, page2.End)
	assert.Equal(t, 10, page2.PageSize)

	// Test first page
	firstPage := paginator.FirstPage(10)
	assert.Equal(t, 0, firstPage.Start)
	assert.Equal(t, 10, firstPage.End)
	assert.Equal(t, 10, firstPage.PageSize)

	// Test next page
	nextPage := paginator.NextPage(firstPage)
	assert.Equal(t, 10, nextPage.Start)
	assert.Equal(t, 20, nextPage.End)
	assert.Equal(t, 10, nextPage.PageSize)

	// Test previous page
	prevPage := paginator.PreviousPage(nextPage)
	assert.Equal(t, 0, prevPage.Start)
	assert.Equal(t, 10, prevPage.End)
	assert.Equal(t, 10, prevPage.PageSize)

	// Test calculating total pages
	assert.Equal(t, 10, paginator.CalculateTotalPages(95, 10))
	assert.Equal(t, 10, paginator.CalculateTotalPages(100, 10))
	assert.Equal(t, 0, paginator.CalculateTotalPages(0, 10))

	// Test is last page
	assert.True(t, paginator.IsLastPage(paginator.CreatePagination(10, 10), 95))
	assert.False(t, paginator.IsLastPage(paginator.CreatePagination(9, 10), 95))

	// Test get page number
	assert.Equal(t, 1, paginator.GetPageNumber(firstPage))
	assert.Equal(t, 2, paginator.GetPageNumber(nextPage))

	// Test using pagination in a service test
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Setup mock expectations for a paginated query
	userBuilder := NewModelRowBuilder("username", "email")
	userBuilder.AddModelRow(1, "user1", "user1@example.com")
	userBuilder.AddModelRow(2, "user2", "user2@example.com")

	// Set up expectations for count and find
	tc.ForTable("users").ExpectCount(10)
	tc.ForTable("users").ExpectFind().ReturnRows(userBuilder.Build())

	// Example service function that would use pagination
	findUsers := func(pagination queryutil.Pagination) ([]interface{}, int64, error) {
		// In a real service, this would query the database
		// For this example, we'll just return mock data
		return []interface{}{
			map[string]string{"username": "user1", "email": "user1@example.com"},
			map[string]string{"username": "user2", "email": "user2@example.com"},
		}, 10, nil
	}

	// Test the service function with pagination
	users, total, err := findUsers(paginator.FirstPage(10))
	assert.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, users, 2)

	// Test navigating through pages
	for i := 1; i <= paginator.CalculateTotalPages(total, 10); i++ {
		pagination := paginator.CreatePagination(i, 10)
		_, _, err := findUsers(pagination)
		assert.NoError(t, err)
	}
}
