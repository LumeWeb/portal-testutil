package testutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestCountExpectationBuilder(t *testing.T) {
	t.Run("SQL pattern matching works correctly", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model to avoid soft delete complexity
		type Item struct {
			ID int
		}
		tc.RegisterModel(&Item{})

		// Set up count expectation
		tc.ForTable("items").ExpectCount().ReturnCount(5)

		// Execute the count query
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.NoError(t, result.Error, "Count query should succeed")
		assert.Equal(t, int64(5), count, "Count should match expected value")

		// If we get here without errors, the test passes
		tc.VerifyExpectations()
	})
	t.Run("ExpectCount with direct count", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Use the direct count approach
		tc.ForTable("items").ExpectCount(5)

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(5), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with direct count using int argument", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Use the direct count approach with int argument
		tc.ForTable("items").ExpectCount(5)

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(5), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with builder ReturnCount", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model to avoid soft delete complexity
		type Item struct {
			ID int
		}
		tc.RegisterModel(&Item{})

		// Use builder style count API
		tc.ForTable("items").ExpectCount().ReturnCount(10)

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(10), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with custom column name", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model to avoid soft delete complexity
		type Item struct {
			ID int
		}
		tc.RegisterModel(&Item{})

		// Use builder style API with custom column name
		tc.ForTable("items").ExpectCount(WithColumnName("count")).ReturnCount(15)

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(15), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with direct count and custom column name", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Use the direct count approach with custom column name
		tc.ForTable("items").ExpectCount(5, WithColumnName("count"))

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(5), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with ReturnError", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model to avoid soft delete complexity
		type Item struct {
			ID int
		}
		tc.RegisterModel(&Item{})

		// Set up an error expectation with builder API
		expectedErr := errors.New("database count error")
		tc.ForTable("items").ExpectCount().ReturnError(expectedErr)

		// Execute a count query, expect an error
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.Error(t, result.Error)
		assert.Equal(t, expectedErr, result.Error)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with Where condition", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up a count with where condition using builder API
		tc.ForTable("items").ExpectCount().Where("status = ?", "active").ReturnCount(3)

		// Execute a count query with where condition
		var count int64
		result := tc.DB().Table("items").Where("status = ?", "active").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(3), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with Where condition and custom column name", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up a count with where condition and custom column name using builder API
		tc.ForTable("items").ExpectCount(WithColumnName("count")).
			Where("status = ?", "active").
			ReturnCount(3)

		// Execute a count query with where condition
		var count int64
		result := tc.DB().Table("items").Where("status = ?", "active").Count(&count)
		assert.NoError(t, result.Error)
		assert.Equal(t, int64(3), count)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with Where condition and ReturnError", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up an error expectation with where condition using builder API
		expectedErr := errors.New("filtered count error")
		tc.ForTable("items").ExpectCount().
			Where("status = ?", "inactive").
			ReturnError(expectedErr)

		// Execute a count query with where condition, expect an error
		var count int64
		result := tc.DB().Table("items").Where("status = ?", "inactive").Count(&count)
		assert.Error(t, result.Error)
		assert.Equal(t, expectedErr, result.Error)

		tc.VerifyExpectations()
	})

	t.Run("ExpectCount with NotFound error", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model to avoid soft delete complexity
		type Item struct {
			ID int
		}
		tc.RegisterModel(&Item{})

		// Set up a not found error expectation using builder API
		tc.ForTable("items").ExpectCount().ReturnError(gorm.ErrRecordNotFound)

		// Execute a count query, expect a not found error
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.Error(t, result.Error)
		assert.Equal(t, gorm.ErrRecordNotFound, result.Error)

		tc.VerifyExpectations()
	})

	t.Run("Multiple count queries", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register models to avoid soft delete complexity
		type User struct {
			ID int
		}
		type Item struct {
			ID int
		}
		type Product struct {
			ID int
		}
		tc.RegisterModel(&User{})
		tc.RegisterModel(&Item{})
		tc.RegisterModel(&Product{})

		// Set up all expectations using builder API
		tc.ForTable("users").ExpectCount().ReturnCount(15)
		tc.ForTable("items").ExpectCount().ReturnCount(25)
		tc.ForTable("products").ExpectCount().Where("category = ?", "electronics").ReturnCount(5)

		// Execute all count queries
		var userCount, itemCount, productCount int64

		userResult := tc.DB().Table("users").Count(&userCount)
		assert.NoError(t, userResult.Error)
		assert.Equal(t, int64(15), userCount)

		itemResult := tc.DB().Table("items").Count(&itemCount)
		assert.NoError(t, itemResult.Error)
		assert.Equal(t, int64(25), itemCount)

		productResult := tc.DB().Table("products").Where("category = ?", "electronics").Count(&productCount)
		assert.NoError(t, productResult.Error)
		assert.Equal(t, int64(5), productCount)

		tc.VerifyExpectations()
	})
}

// TestCountSQLPatternRegression tests the fix for the SQL pattern matching issue where
// the ^ anchor in ExpectCount patterns prevented matching with GORM's actual SQL.
func TestCountSQLPatternRegression(t *testing.T) {
	// Test the direct ExpectCount(n) approach
	t.Run("Direct ExpectCount works with GORM queries", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Use the direct count approach
		tc.ForTable("items").ExpectCount(5)

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)

		// Verify it works correctly
		assert.NoError(t, result.Error, "Count query should succeed with direct approach")
		assert.Equal(t, int64(5), count, "Count should match expected value")
	})

	// Test the builder API approach
	t.Run("Builder API works with GORM queries", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model to avoid soft delete complexity
		type Item struct {
			ID int
		}
		tc.RegisterModel(&Item{})

		// Use the builder API
		tc.ForTable("items").ExpectCount().ReturnCount(10)

		// Execute a count query
		var count int64
		result := tc.DB().Table("items").Count(&count)

		// Verify it works correctly
		assert.NoError(t, result.Error, "Count query should succeed with builder API")
		assert.Equal(t, int64(10), count, "Count should match expected value")
	})

	// Test with WHERE clause
	t.Run("With WHERE clause", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up expectation with WHERE condition
		tc.ForTable("items").ExpectCount().Where("status = ?", "active").ReturnCount(3)

		// Execute count query with WHERE condition
		var count int64
		result := tc.DB().Table("items").Where("status = ?", "active").Count(&count)

		// Verify it works correctly
		assert.NoError(t, result.Error, "Count with WHERE query should succeed")
		assert.Equal(t, int64(3), count, "Count should match expected value")
	})

	// Test ReturnError
	t.Run("With ReturnError", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up expectation with error
		tc.ForTable("items").ExpectCount().ReturnError(assert.AnError)

		// Execute count query
		var count int64
		result := tc.DB().Table("items").Count(&count)

		// Verify an error is returned (don't check exact error)
		assert.Error(t, result.Error, "Count query should return an error")
	})

	// Test the Mock() method directly
	t.Run("Using Mock method directly", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Get direct access to the sqlmock via Mock() method
		mock := tc.Mock()

		// Set up the mock with exact pattern that works with GORM
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `items`").
			WillReturnRows(mock.NewRows([]string{"count(*)"}).AddRow(2))

		// Execute count query
		var count int64
		err := tc.DB().Table("items").Count(&count).Error

		// Verify it worked correctly
		assert.NoError(t, err, "Count query should succeed with direct mock setup")
		assert.Equal(t, int64(2), count, "Count should match expected value")
	})
}
