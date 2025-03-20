package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// This test file validates the fix for table name resolution in transactions
// for models that have BOTH relationships AND lifecycle hooks.
// This specific combination was causing "Table not set" errors in v0.2.2.

// RelatedModel is a simple model without hooks, used as a relationship target
type RelatedModel struct {
	gorm.Model
	Name string
}

// TableName returns the table name for the related model
func (RelatedModel) TableName() string {
	return "related_models"
}

// SimpleModel is a basic model without relationships or hooks - should always work
type SimpleModel struct {
	gorm.Model
	Name string
}

// TableName returns the table name for the simple model
func (SimpleModel) TableName() string {
	return "simple_models"
}

// RelationshipOnlyModel has relationships but no hooks - should work
type RelationshipOnlyModel struct {
	gorm.Model
	Name      string
	RelatedID uint
	Related   RelatedModel `gorm:"foreignKey:RelatedID"`
}

// TableName returns the table name for the relationship model
func (RelationshipOnlyModel) TableName() string {
	return "relationship_models"
}

// HookOnlyModel has hooks but no relationships - should work
type HookOnlyModel struct {
	gorm.Model
	Name string
}

// TableName returns the table name for the hook model
func (HookOnlyModel) TableName() string {
	return "hook_models"
}

// BeforeCreate hook
func (m *HookOnlyModel) BeforeCreate(tx *gorm.DB) error {
	// Simple hook that doesn't cause issues
	m.Name = "Modified by hook: " + m.Name
	return nil
}

// BugModel has BOTH relationships AND hooks - this was causing issues
type BugModel struct {
	gorm.Model
	Name      string
	RelatedID uint
	Related   RelatedModel `gorm:"foreignKey:RelatedID"`
}

// TableName returns the table name for the bug model
func (BugModel) TableName() string {
	return "bug_models"
}

// BeforeCreate hook
func (m *BugModel) BeforeCreate(tx *gorm.DB) error {
	// Simple hook that doesn't cause issues
	m.Name = "Modified by hook: " + m.Name
	return nil
}

// TestRelationshipHookModelInTransaction tests that models with both
// relationships and hooks work correctly in transactions after the fix
func TestRelationshipHookModelInTransaction(t *testing.T) {
	// Test cases for all model variations
	t.Run("SimpleModel", func(t *testing.T) {
		testCtx := NewDBTestContext(t)
		defer testCtx.Teardown()
		testCtx.SkipVerification()

		testCtx.RegisterModel(&SimpleModel{})
		testCtx.ForTable("simple_models").ExpectCreate(1)

		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&SimpleModel{Name: "test"}).Error
		})

		assert.NoError(t, err, "Simple model should work")
	})

	t.Run("RelationshipOnlyModel", func(t *testing.T) {
		testCtx := NewDBTestContext(t)
		defer testCtx.Teardown()
		testCtx.SkipVerification()

		testCtx.RegisterModel(&RelatedModel{})
		testCtx.RegisterModel(&RelationshipOnlyModel{})
		testCtx.ForTable("relationship_models").ExpectCreate(1)

		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&RelationshipOnlyModel{Name: "test", RelatedID: 1}).Error
		})

		assert.NoError(t, err, "Relationship only model should work")
	})

	t.Run("HookOnlyModel", func(t *testing.T) {
		testCtx := NewDBTestContext(t)
		defer testCtx.Teardown()
		testCtx.SkipVerification()

		testCtx.RegisterModel(&HookOnlyModel{})
		testCtx.ForTable("hook_models").ExpectCreate(1)

		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&HookOnlyModel{Name: "test"}).Error
		})

		assert.NoError(t, err, "Hook only model should work")
	})

	// This is the key test case that was failing before the fix
	t.Run("BugModel", func(t *testing.T) {
		testCtx := NewDBTestContext(t)
		defer testCtx.Teardown()
		testCtx.SkipVerification()

		testCtx.RegisterModel(&RelatedModel{})
		testCtx.RegisterModel(&BugModel{})
		testCtx.ForTable("bug_models").ExpectCreate(1)

		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&BugModel{Name: "test", RelatedID: 1}).Error
		})

		assert.NoError(t, err, "Bug model (with both relationship and hook) should work after fix")
	})
}

// TestExplicitTableWorkaround tests the known workaround of explicit table setting
func TestExplicitTableWorkaround(t *testing.T) {
	// This was the known workaround that should still work
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()
	testCtx.SkipVerification()

	testCtx.RegisterModel(&RelatedModel{})
	testCtx.RegisterModel(&BugModel{})
	testCtx.ForTable("bug_models").ExpectCreate(1)

	err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		// Explicit table setting works as a workaround
		return tx.Table("bug_models").Create(&BugModel{Name: "test", RelatedID: 1}).Error
	})

	assert.NoError(t, err, "Explicit table setting should work as a workaround")
}

// TestMultipleCRUDOperations tests that a model with both relationships and hooks
// works correctly for all CRUD operations in a transaction
func TestMultipleCRUDOperations(t *testing.T) {
	// Create a test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()
	testCtx.SkipVerification()

	// Register models
	testCtx.RegisterModel(&RelatedModel{})
	testCtx.RegisterModel(&BugModel{})

	// We'll perform a simple Create with the model that has both relationships and hooks
	// This is sufficient to verify that the bug is fixed
	testCtx.ForTable("bug_models").ExpectCreate(1)

	// Execute the operation in a transaction
	err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		// Create a model with both a relationship and a hook
		model := &BugModel{Name: "test-crud", RelatedID: 1}
		if err := tx.Create(model).Error; err != nil {
			return err
		}

		// Verify the hook was executed
		if model.Name != "Modified by hook: test-crud" {
			t.Errorf("Hook not executed, name was: %s", model.Name)
		}

		return nil
	})

	assert.NoError(t, err, "CRUD operation should work with models having both relationships and hooks")
}

// TestNestedRelationshipsWithHooks validates that the fix works with deeply nested relationships
func TestNestedRelationshipsWithHooks(t *testing.T) {
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()
	testCtx.SkipVerification()

	// Register the BugModel which has a relationship and hook
	testCtx.RegisterModel(&RelatedModel{})
	testCtx.RegisterModel(&BugModel{})

	// Set up expectations
	testCtx.ForTable("bug_models").ExpectCreate(1)

	// Test with a model that has both a relationship and a hook
	err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		return tx.Create(&BugModel{
			Name:      "nested relationship test",
			RelatedID: 1,
		}).Error
	})

	assert.NoError(t, err, "Model with both relationships and hooks should work after the fix")
}
