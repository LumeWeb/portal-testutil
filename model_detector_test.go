package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestTableHasSoftDelete tests the soft delete detection functionality
func TestTableHasSoftDelete(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Create models to test with
	type ModelWithGormModel struct {
		gorm.Model // has DeletedAt
		Name       string
	}

	type ModelWithDeletedAt struct {
		ID        uint
		DeletedAt gorm.DeletedAt
		Name      string
	}

	type ModelWithoutSoftDelete struct {
		ID   uint
		Name string
	}

	// Register the models
	tc.RegisterModel(&ModelWithGormModel{})
	tc.RegisterModel(&ModelWithDeletedAt{})
	tc.RegisterModel(&ModelWithoutSoftDelete{})

	// Test the detection function directly
	assert.True(t, tc.tableHasSoftDelete("model_with_gorm_models"),
		"Should detect soft delete in models with gorm.Model")

	assert.True(t, tc.tableHasSoftDelete("model_with_deleted_ats"),
		"Should detect soft delete in models with explicit DeletedAt field")

	assert.False(t, tc.tableHasSoftDelete("model_without_soft_deletes"),
		"Should not detect soft delete in models without DeletedAt field")

	// Test with an unregistered table - should default to true
	assert.True(t, tc.tableHasSoftDelete("unregistered_table"),
		"Should default to assuming soft delete for unregistered tables")
}
