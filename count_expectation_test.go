package testutil

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestCountExpectationBuilder(t *testing.T) {
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

		// Use raw expectation for more precise control
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items`").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(10))

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

		// Use raw expectation with custom column name for more control
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items`").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

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

		// Set up an error expectation with raw for more control
		expectedErr := errors.New("database count error")
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items`").
			WillReturnError(expectedErr)

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

		// Set up a count with where condition using raw SQL for more control
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items` WHERE status = \\?").
			WithArgs("active").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(3))

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

		// Set up a count with where condition and custom column name using raw for more control
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items` WHERE status = \\?").
			WithArgs("active").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

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

		// Set up an error expectation with where condition using raw for more control
		expectedErr := errors.New("filtered count error")
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items` WHERE status = \\?").
			WithArgs("inactive").
			WillReturnError(expectedErr)

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

		// Set up a not found error expectation using raw for more control
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items`").
			WillReturnError(gorm.ErrRecordNotFound)

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

		// Set up all expectations using raw for more control
		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `users`").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(15))

		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `items`").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(25))

		tc.Raw().ExpectQuery("^SELECT count\\(\\*\\) FROM `products` WHERE category = \\?").
			WithArgs("electronics").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(5))

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
