// Package testutil provides utilities for testing service components within the Portal ecosystem.
package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Parent model with Children relationship - used to demonstrate relationship preloading functionality
type ReproParentModel struct {
	gorm.Model
	Name     string
	Children []ReproChildModel `gorm:"foreignKey:ParentID"`
}

// Child model with Parent relationship - used to demonstrate relationship preloading functionality
type ReproChildModel struct {
	gorm.Model
	Name     string
	ParentID uint
}

// TestRelationshipPreloadingWithNewMethod demonstrates that ReturnModelsWithPreload
// successfully populates relationship fields in GORM query results.
//
// This test shows how the new ReturnModelsWithPreload method solves the common issue
// where relationship fields (like slices of related models) remain empty in test results
// even though they were present in the original test data.
func TestRelationshipPreloadingWithNewMethod(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register model with relationships
	RegisterModelWithRelationships[ReproParentModel](tc)

	// Create a parent with children
	now := time.Now()
	parent := ReproParentModel{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent",
		Children: []ReproChildModel{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child",
				ParentID: 1,
			},
		},
	}

	// Use ReturnModelsWithPreload to set up the expectation with automatic relationship injection
	tc.ForTable("repro_parent_models").
		ExpectFind().
		ReturnModelsWithPreload([]ReproParentModel{parent})

	// Execute query that should return the model with relationships
	var results []ReproParentModel
	err := tc.DB().Model(&ReproParentModel{}).Find(&results).Error

	// Verify basic fields work
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Parent", results[0].Name)

	// Print what we found for debug purposes
	t.Logf("Result: %+v", results[0])
	t.Logf("Children: %+v", results[0].Children)

	// With ReturnModelsWithPreload, children relationships ARE populated
	// without requiring explicit Preload("Children") in the query
	assert.Len(t, results[0].Children, 1, "Children slice should be populated")
	assert.Equal(t, "Child", results[0].Children[0].Name)
}

// TestRelationshipPreloadingOriginalBehavior demonstrates the original behavior of ReturnModels
// where relationship fields remain empty in query results, even though they exist in the test data.
//
// This test shows the limitation that ReturnModelsWithPreload was designed to solve,
// where relationship fields aren't automatically populated without explicit GORM Preload calls.
func TestRelationshipPreloadingOriginalBehavior(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register model with relationships
	RegisterModelWithRelationships[ReproParentModel](tc)

	// Create a parent with children
	now := time.Now()
	parent := ReproParentModel{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent",
		Children: []ReproChildModel{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child",
				ParentID: 1,
			},
		},
	}

	// Use the original ReturnModels to set up the expectation
	tc.ForTable("repro_parent_models").
		ExpectFind().
		ReturnModels([]ReproParentModel{parent})

	// Execute query without Preload
	var results []ReproParentModel
	err := tc.DB().Model(&ReproParentModel{}).Find(&results).Error

	// Verify basic fields work
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Parent", results[0].Name)

	// Print what we found for debug purposes
	t.Logf("Result: %+v", results[0])
	t.Logf("Children: %+v", results[0].Children)

	// ORIGINAL BEHAVIOR: Children relationships are NOT populated
	// when using ReturnModels without explicit GORM Preload calls
	assert.Len(t, results[0].Children, 0, "Children slice is empty with original ReturnModels")
}

// TestWhereClauseWithReturnModelsWithPreload verifies that WHERE clauses work
// properly with ReturnModelsWithPreload
func TestWhereClauseWithReturnModelsWithPreload(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register model with relationships
	RegisterModelWithRelationships[ReproParentModel](tc)

	// Create parent with child
	now := time.Now()
	parent := ReproParentModel{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Test Parent",
		Children: []ReproChildModel{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Test Child",
				ParentID: 1,
			},
		},
	}

	// Use Where clause with ReturnModelsWithPreload
	tc.ForTable("repro_parent_models").
		ExpectFind().
		Where("name = ?", "Test Parent").
		ReturnModelsWithPreload([]ReproParentModel{parent})

	// Execute query with WHERE clause
	var results []ReproParentModel
	err := tc.DB().Where("name = ?", "Test Parent").Find(&results).Error

	// Verify query executed without error
	assert.NoError(t, err, "Query should execute without error")

	// Verify result contains the parent
	assert.Len(t, results, 1, "Should have one result")
	assert.Equal(t, "Test Parent", results[0].Name, "Name should match")

	// Verify relationship was preloaded
	assert.Len(t, results[0].Children, 1, "Should have one child")
	assert.Equal(t, "Test Child", results[0].Children[0].Name, "Child name should match")
}

// TestSearchWithReturnModelsWithPreload verifies that search queries work
// properly with ReturnModelsWithPreload
func TestSearchWithReturnModelsWithPreload(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register model with relationships
	RegisterModelWithRelationships[ReproParentModel](tc)

	// Create parent with child
	now := time.Now()
	parent := ReproParentModel{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Test Parent",
		Children: []ReproChildModel{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Test Child",
				ParentID: 1,
			},
		},
	}

	// Use Search with ReturnModelsWithPreload
	tc.ForTable("repro_parent_models").
		ExpectSearch("Parent").
		WithFields("name").
		ReturnModelsWithPreload([]ReproParentModel{parent})

	// Execute search query
	var results []ReproParentModel
	err := tc.DB().Where("name LIKE ?", "%Parent%").Find(&results).Error

	// Verify query executed without error
	assert.NoError(t, err, "Query should execute without error")

	// Verify result contains the parent
	assert.Len(t, results, 1, "Should have one result")
	assert.Equal(t, "Test Parent", results[0].Name, "Name should match")

	// Verify relationship was preloaded
	assert.Len(t, results[0].Children, 1, "Should have one child")
	assert.Equal(t, "Test Child", results[0].Children[0].Name, "Child name should match")
}
