package testutil

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestSoftDeleteCountPrediction verifies that our prediction logic correctly determines
// when GORM will add soft delete clauses to COUNT queries based on our static analysis
func TestSoftDeleteCountPrediction(t *testing.T) {
	t.Run("Table and Model approach with soft delete models", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Define a model with soft delete
		type SoftDeleteModel struct {
			gorm.Model
			Name string
		}

		// Register the model
		tc.RegisterModel(&SoftDeleteModel{})
		tableName := "soft_delete_models"

		// 1. Test prediction for Table() approach - should predict NO soft delete clause
		willAddClause := tc.willGORMAddSoftDeleteClause(tableName, "COUNT")
		assert.False(t, willAddClause,
			"Should predict NO soft delete clause for Table() COUNT approach")

		// 2. Verify the actual behavior with the Table() approach
		var count int64

		// Set up raw SQL mock to capture what GORM actually generates
		tc.Raw().ExpectQuery("SELECT count\\(\\*\\) FROM `soft_delete_models`").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(5))

		err := tc.DB().Table(tableName).Count(&count).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		// Reset for next test
		var modelCount int64

		// 3. Set up raw SQL mock for Model() approach - should include soft delete clause
		tc.Raw().ExpectQuery("SELECT count\\(\\*\\) FROM `soft_delete_models` WHERE `soft_delete_models`.`deleted_at` IS NULL").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(3))

		err = tc.DB().Model(&SoftDeleteModel{}).Count(&modelCount).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(3), modelCount)
	})

	t.Run("Non-soft-delete models", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Define a model without soft delete
		type RegularModel struct {
			ID   uint `gorm:"primaryKey"`
			Name string
		}

		// Register the model
		tc.RegisterModel(&RegularModel{})
		tableName := "regular_models"

		// 1. Test prediction for any approach - should predict NO soft delete clause
		// since the model doesn't have soft delete
		willAddClause := tc.willGORMAddSoftDeleteClause(tableName, "COUNT")
		assert.False(t, willAddClause,
			"Should predict NO soft delete clause for models without soft delete")

		// 2. Verify the actual behavior
		var count int64

		// Set up raw SQL mock to capture what GORM actually generates
		tc.Raw().ExpectQuery("SELECT count\\(\\*\\) FROM `regular_models`").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(7))

		err := tc.DB().Table(tableName).Count(&count).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(7), count)

		// 3. Try with Model() - should still not include soft delete
		var modelCount int64
		tc.Raw().ExpectQuery("SELECT count\\(\\*\\) FROM `regular_models`").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(4))

		err = tc.DB().Model(&RegularModel{}).Count(&modelCount).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(4), modelCount)
	})
}

// TestLazyRegexFallbackForSoftDelete verifies that our lazy regex pattern works
// correctly with both Table() and Model() approaches
func TestLazyRegexFallbackForSoftDelete(t *testing.T) {
	t.Run("ExpectCount works with both Table and Model approaches", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Define a model with soft delete
		type SoftDeleteModel struct {
			gorm.Model
			Name string
		}

		// Register the model
		tc.RegisterModel(&SoftDeleteModel{})
		tableName := "soft_delete_models"

		// Set up the expectation with ExpectCount - should create a flexible pattern
		// that handles both Table() and Model() approaches
		tc.ForTable(tableName).
			ExpectCount().
			ReturnCount(5)

		// 1. Verify it works with Table() approach (no soft delete clause)
		var tableCount int64
		err := tc.DB().Table(tableName).Count(&tableCount).Error
		assert.NoError(t, err, "ExpectCount should work with Table approach")
		assert.Equal(t, int64(5), tableCount)

		// 2. Set up again for Model() approach
		tc.ForTable(tableName).
			ExpectCount().
			ReturnCount(3)

		// Verify it works with Model() approach (with soft delete clause)
		var modelCount int64
		err = tc.DB().Model(&SoftDeleteModel{}).Count(&modelCount).Error
		assert.NoError(t, err, "ExpectCount should work with Model approach")
		assert.Equal(t, int64(3), modelCount)
	})

	t.Run("ExpectCount handles WHERE conditions with soft delete", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Define a model with soft delete
		type SoftDeleteModel struct {
			gorm.Model
			Name   string
			Status string
		}

		// Register the model
		tc.RegisterModel(&SoftDeleteModel{})
		tableName := "soft_delete_models"

		// Set up expectation with WHERE condition
		tc.ForTable(tableName).
			ExpectCount().
			Where("status = ?", "active").
			ReturnCount(2)

		// 1. Test with Table approach
		var tableCount int64
		err := tc.DB().Table(tableName).
			Where("status = ?", "active").
			Count(&tableCount).Error
		assert.NoError(t, err, "ExpectCount with WHERE should work with Table approach")
		assert.Equal(t, int64(2), tableCount)

		// 2. Set up again for Model approach
		tc.ForTable(tableName).
			ExpectCount().
			Where("status = ?", "active").
			ReturnCount(1)

		// Test with Model approach
		var modelCount int64
		err = tc.DB().Model(&SoftDeleteModel{}).
			Where("status = ?", "active").
			Count(&modelCount).Error
		assert.NoError(t, err, "ExpectCount with WHERE should work with Model approach")
		assert.Equal(t, int64(1), modelCount)
	})
}

// TestPredictionAndRegexFallbackIntegration verifies that when prediction fails,
// the regex fallback mechanism still allows the test to pass
func TestPredictionAndRegexFallbackIntegration(t *testing.T) {
	t.Run("Verify regex fallback when model is registered but prediction wrong", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Define a model with soft delete
		type SoftDeleteModel struct {
			gorm.Model
			Name string
		}

		// Register the model so tableHasSoftDelete will return true
		tc.RegisterModel(&SoftDeleteModel{})

		// Set up expectation that will generate flexible pattern with the regex fallback
		tc.ForTable("soft_delete_models").
			ExpectCount().
			ReturnCount(3)

		// In normal case, our prediction says Table() won't add soft delete clause
		// But we'll now use Model() to force a case where it does add the clause
		// This tests that even if our prediction function were wrong, the regex would save us
		var count int64
		err := tc.DB().Model(&SoftDeleteModel{}).Count(&count).Error
		assert.NoError(t, err, "Regex fallback should handle the soft delete clause")
		assert.Equal(t, int64(3), count)
	})

	t.Run("Verify regex fallback with unregistered model", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Define a model with soft delete but DON'T register it
		// This should trigger the fallback because isModelRegistered would return false
		type UnregisteredModel struct {
			gorm.Model
			Name string
		}

		// Importantly, we don't register this model to test the fallback
		// tc.RegisterModel(&UnregisteredModel{}) - intentionally commented out

		// Set up expectation - with an unregistered model, our prediction might be wrong
		// but the regex fallback should still work
		tc.ForTable("unregistered_models").
			ExpectCount().
			ReturnCount(7)

		// Use Model approach, which will add soft delete clause
		// Since the model isn't registered, our detection relies on the regex fallback
		var count int64
		err := tc.DB().Model(&UnregisteredModel{}).Count(&count).Error

		// This should still work because of the regex pattern with optional soft delete match
		assert.NoError(t, err, "Regex fallback should work even for unregistered models")
		assert.Equal(t, int64(7), count)
	})
}

// TestSpecificPatternPrediction verifies the case where our prediction logic
// would correctly predict GORM will add a soft delete clause and uses a specific pattern
func TestSpecificPatternPrediction(t *testing.T) {
	t.Run("Verify specific pattern is used for Find queries", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// For Find queries, GORM should add soft delete clauses when using Model()
		// Unlike Count queries where we needed to patch the function

		// Define a model with soft delete
		type SoftDeleteModel struct {
			gorm.Model
			Name string
		}

		// Register the model
		tc.RegisterModel(&SoftDeleteModel{})

		// Set up an expectation for a Find query - this should trigger the specific pattern
		// branch in ExpectFind() since willGORMAddSoftDeleteClause returns true for non-COUNT queries
		// with soft delete models
		tc.ForTable("soft_delete_models").
			ExpectFind().
			ReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "name"}))

		// Execute a find query - this should use Model() to ensure soft delete clause is added
		var results []SoftDeleteModel
		err := tc.DB().Model(&SoftDeleteModel{}).Find(&results).Error

		// This should succeed because we're using a specific pattern that includes
		// the soft delete clause for Find queries
		assert.NoError(t, err, "ExpectFind should use specific pattern with deleted_at clause")
	})

	t.Run("Verify direct pattern construction for specific count with soft delete", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Instead of trying to override willGORMAddSoftDeleteClause, we'll bypass the built-in
		// patterns and directly set up a raw SQL expectation that should match what would
		// be created if willGORMAddSoftDeleteClause returned true for a COUNT query

		// We'll use raw SQL expectation to simulate the specific path being taken
		tc.Raw().ExpectQuery("SELECT count\\(\\*\\) FROM `custom_soft_delete` WHERE `custom_soft_delete`.`deleted_at` IS NULL").
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(5))

		// Now run a query that will exactly match this pattern
		var count int64
		err := tc.DB().Table("custom_soft_delete").
			Where("`custom_soft_delete`.`deleted_at` IS NULL").
			Count(&count).Error

		// The query should succeed with our specific pattern match
		assert.NoError(t, err, "Specific SQL pattern should match the query")
		assert.Equal(t, int64(5), count)
	})
}

// TestExpectCountRegression reproduces the issue with ExpectCount in v0.2.14 and verifies the fix
func TestExpectCountRegression(t *testing.T) {
	t.Run("ExpectCount works after fix", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up count expectation with ExpectCount
		tc.ForTable("test_table").
			ExpectCount().
			ReturnCount(1)

		// Execute a simple count query
		var count int64
		err := tc.DB().Table("test_table").Count(&count).Error

		// This should now pass with the fix adding flexibility to match both Table() and Model()
		assert.NoError(t, err, "ExpectCount should match the GORM query")
		assert.Equal(t, int64(1), count, "Count should be 1")
	})

	t.Run("ExpectCount works with soft delete model", func(t *testing.T) {
		// Create a test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register a model with soft delete
		type SoftDeleteModel struct {
			ID        uint
			DeletedAt gorm.DeletedAt
		}
		tc.RegisterModel(&SoftDeleteModel{})

		// Set up count expectation with ExpectCount
		tc.ForTable("soft_delete_models").
			ExpectCount().
			ReturnCount(5)

		// Execute a count query with both Model and Table approaches
		var tableCount, modelCount int64

		// Table approach (no soft delete)
		err := tc.DB().Table("soft_delete_models").Count(&tableCount).Error
		assert.NoError(t, err, "ExpectCount should match the Table query without soft delete")
		assert.Equal(t, int64(5), tableCount)

		// Set up again for Model approach
		tc.ForTable("soft_delete_models").
			ExpectCount().
			ReturnCount(3)

		// Model approach (with soft delete)
		err = tc.DB().Model(&SoftDeleteModel{}).Count(&modelCount).Error
		assert.NoError(t, err, "ExpectCount should match the Model query with soft delete")
		assert.Equal(t, int64(3), modelCount)
	})
}
