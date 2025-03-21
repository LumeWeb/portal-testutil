package testutil

import (
	"testing"
)

// TestReturnModels_FindExpectation demonstrates the new ReturnModels method
// which allows you to directly pass model slices to ExpectFind.
func TestReturnModels_FindExpectation(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Define models to be returned
	type User struct {
		ID   int
		Name string
		Age  int
	}

	users := []User{
		{ID: 1, Name: "Alice", Age: 30},
		{ID: 2, Name: "Bob", Age: 25},
	}

	// Use ReturnModels directly instead of manually converting to rows
	tc.ForTable("users").
		ExpectFind().
		Where("age > ?", 20).
		ReturnModels(users)

	// Execute a find query
	var results []User
	tc.DB().Table("users").Where("age > ?", 20).Find(&results)

	// Verify it worked correctly
	tc.VerifyExpectations()
}

// TestReturnModels_SearchExpectation demonstrates the ReturnModels method for search expectations.
func TestReturnModels_SearchExpectation(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Define models to be returned
	type Product struct {
		ID    int
		Name  string
		Price float64
	}

	products := []Product{
		{ID: 1, Name: "Smartphone", Price: 799.99},
		{ID: 2, Name: "Smart Watch", Price: 299.99},
	}

	// Use ReturnModels directly with search expectation
	tc.ForTable("products").
		ExpectSearch("smart").
		WithFields("name").
		ReturnCount(2).
		ReturnModels(products)

	// Execute a search query
	var results []Product
	tc.DB().Table("products").Where("name LIKE ?", "%smart%").Find(&results)

	// Verify it worked correctly
	tc.VerifyExpectations()
}

// TestReturnModels_QueryExpectation demonstrates the ReturnModels method for custom query expectations.
func TestReturnModels_QueryExpectation(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Define models to be returned
	type Article struct {
		ID      int
		Title   string
		Content string
	}

	articles := []Article{
		{ID: 1, Title: "First Article", Content: "Content 1"},
		{ID: 2, Title: "Second Article", Content: "Content 2"},
	}

	// Use ReturnModels with a custom query
	tc.Expect().
		Query("SELECT \\* FROM articles WHERE id IN \\(\\?, \\?\\)").
		WithArgs(1, 2).
		ReturnModels("articles", articles)

	// Execute a raw query
	var results []Article
	tc.DB().Raw("SELECT * FROM articles WHERE id IN (?, ?)", 1, 2).Scan(&results)

	// Verify it worked correctly
	tc.VerifyExpectations()
}
