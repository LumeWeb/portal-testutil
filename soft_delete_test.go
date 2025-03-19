package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Test soft delete with error conditions
func TestSoftDeleteWithErrors(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register the model
	tc.RegisterModel(&TestUser{})

	// Use our high-level API with a small adjustment for the soft delete condition
	tc.ForTable("test_users").
		ExpectCount().
		Where("`test_users`.`deleted_at` IS NULL"). // Explicitly include the soft delete condition
		ReturnError(fmt.Errorf("database error"))

	// Execute the query
	var count int64
	err := tc.DB().Model(&TestUser{}).Count(&count).Error

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")

	// The actual SQL GORM will execute is:
	// "SELECT * FROM `test_users` WHERE username = ? AND `test_users`.`deleted_at` IS NULL ORDER BY `test_users`.`id` LIMIT 1"

	// Set up expectation for NotFound error
	tc.Raw().ExpectQuery("^SELECT \\* FROM `test_users` WHERE username = \\? AND `test_users`.`deleted_at` IS NULL").
		WithArgs("nonexistent").
		WillReturnError(gorm.ErrRecordNotFound)

	// Execute the query
	var user TestUser
	err = tc.DB().Model(&TestUser{}).Where("username = ?", "nonexistent").First(&user).Error

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	// Verify all expectations were met
	tc.VerifyExpectations()
}

// Test soft delete handling with explicit deleted_at condition
func TestExplicitDeletedAtCondition(t *testing.T) {
	// Create a test context with SQL debugging enabled
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register the model
	tc.RegisterModel(&TestUser{})

	// Set up count expectation with an explicit deleted_at condition
	// Our updated implementation should detect this and not add another one
	tc.ForTable("test_users").
		ExpectCount().
		Where("`test_users`.`deleted_at` IS NULL AND username = ?", "john").
		ReturnCount(1)

	// Execute query with the same condition
	var count int64
	err := tc.DB().Model(&TestUser{}).
		Where("`test_users`.`deleted_at` IS NULL AND username = ?", "john").
		Count(&count).Error

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Verify all expectations were met
	tc.VerifyExpectations()
}

// TestExplicitDeletedAtInFind tests explicit deleted_at conditions in GORM Find queries
func TestExplicitDeletedAtInFind(t *testing.T) {
	// Create a test context with SQL debugging enabled
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register the model
	tc.RegisterModel(&TestUser{})

	// Set up a find with direct deleted_at reference
	now := time.Now()
	user := TestUser{
		Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Username: "john",
		Email:    "john@example.com",
		Active:   true,
	}
	userRow := tc.BuildRowsFrom("test_users", user)

	// Use the high-level API with our specialized method for deleted_at handling
	// This method creates a very precise pattern that matches exactly what GORM generates
	tc.ForTable("test_users").
		ExpectFind().
		Where("deleted_at IS NULL").
		WithDeletedAt().
		HandleDeletedAtRows(userRow)

	// Execute the query
	var foundUser TestUser
	err := tc.DB().Model(&TestUser{}).
		Where("deleted_at IS NULL").
		First(&foundUser).Error

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "john", foundUser.Username)

	// Verify all expectations were met
	tc.VerifyExpectations()
}

// Test that non-soft-delete models work correctly
type NonSoftDeleteModel struct {
	ID       uint `gorm:"primaryKey"`
	Name     string
	IsActive bool
}

func TestNonSoftDeleteModels(t *testing.T) {
	// Create a test context with SQL debugging enabled
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register the non-soft-delete model
	tc.RegisterModel(&NonSoftDeleteModel{})

	// Set up count expectation - should NOT add deleted_at
	tc.ForTable("non_soft_delete_models").
		ExpectCount().
		ReturnCount(5)

	// Execute the query
	var count int64
	err := tc.DB().Model(&NonSoftDeleteModel{}).Count(&count).Error

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Use our high-level API
	// Create a row with test data using our row builder
	testModel := NonSoftDeleteModel{
		ID:       1,
		Name:     "item",
		IsActive: true,
	}
	modelRow := tc.BuildRowsFrom("non_soft_delete_models", testModel)

	// Set up a find expectation with WHERE - should NOT add deleted_at
	// We need to use First() explicitly since our test is using First()
	tc.ForTable("non_soft_delete_models").
		ExpectFind().
		Where("is_active = ?", true).
		First(). // Add this to match the First() query in the test
		ReturnRows(modelRow)

	// Execute the query
	var foundModel NonSoftDeleteModel
	err = tc.DB().Model(&NonSoftDeleteModel{}).Where("is_active = ?", true).First(&foundModel).Error

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "item", foundModel.Name)

	// Verify all expectations were met
	tc.VerifyExpectations()
}
