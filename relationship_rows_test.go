package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Test models for relationship testing
type ParentModel struct {
	gorm.Model
	Name     string
	Children []ChildModel `gorm:"foreignKey:ParentID"`
}

type ChildModel struct {
	gorm.Model
	Name     string
	ParentID uint
	Parent   ParentModel `gorm:"foreignKey:ParentID"`
}

func (ParentModel) TableName() string {
	return "parent_models"
}

func (ChildModel) TableName() string {
	return "child_models"
}

// TestBuildRowsWithRelations tests that BuildRowsWithRelations properly handles model relationships
func TestBuildRowsWithRelations(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register models
	RegisterModelWithRelationships[ParentModel](tc)
	RegisterModelWithRelationships[ChildModel](tc)

	// Set up test data with relationships
	now := time.Now()
	parents := []ParentModel{
		{
			Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
			Name:  "Parent 1",
			Children: []ChildModel{
				{
					Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
					Name:     "Child 1",
					ParentID: 1,
				},
				{
					Model:    gorm.Model{ID: 2, CreatedAt: now, UpdatedAt: now},
					Name:     "Child 2",
					ParentID: 1,
				},
			},
		},
	}

	// Build rows WITH relationship support
	rows := tc.BuildRowsWithRelations("parent_models", parents)

	// Set up the SQL mock expectation
	mock := tc.Raw()
	pattern := "SELECT \\* FROM `parent_models`"
	mock.ExpectQuery(pattern).WillReturnRows(rows)

	// Execute the query - this should now work without "unsupported data type: &map[]" errors
	var results []ParentModel
	err := tc.DB().Find(&results).Error
	assert.NoError(t, err)

	// Validate the results
	assert.Len(t, results, 1)
	assert.Equal(t, "Parent 1", results[0].Name)

	// The key success metric is that the query executes without the "unsupported data type: &map[]"
	// error that would normally occur with relationship fields
}
