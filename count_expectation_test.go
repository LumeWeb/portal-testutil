package testutil

import (
	"errors"
	"testing"

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

		// Use the builder approach
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

		// Use custom column name for backward compatibility
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

		// Set up an error expectation
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

		// Set up a count with where condition
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

		// Set up a count with where condition and custom column name
		tc.ForTable("items").ExpectCount(WithColumnName("count")).Where("status = ?", "active").ReturnCount(3)

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

		// Set up an error expectation with where condition
		expectedErr := errors.New("filtered count error")
		tc.ForTable("items").ExpectCount().Where("status = ?", "inactive").ReturnError(expectedErr)

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

		// Set up a not found error expectation
		tc.ForTable("items").ExpectCount().ReturnError(gorm.ErrRecordNotFound)

		// Execute a count query, expect a not found error
		var count int64
		result := tc.DB().Table("items").Count(&count)
		assert.Error(t, result.Error)
		assert.Equal(t, gorm.ErrRecordNotFound, result.Error)

		tc.VerifyExpectations()
	})

	t.Run("Both count expectation approaches work together", func(t *testing.T) {
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up both styles of expectations
		tc.ForTable("users").ExpectCount(15)
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
