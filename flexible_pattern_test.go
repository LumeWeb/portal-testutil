package testutil

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestGormFirstQueryPattern tests that our flexible SQL pattern matching works
// with GORM's First() method which adds additional conditions beyond what we specify
func TestGormFirstQueryPattern(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register a model with soft delete
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()
	user := TestUser{
		Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Username: "john",
		Email:    "john@example.com",
		Active:   true,
	}

	// Create row for single user
	userRow := tc.BuildRowsFrom("test_users", user)

	// Set up a find expectation with a simple WHERE clause
	// Use our specialized handler for standard First() queries
	tc.ForTable("test_users").
		ExpectFind().
		Where("username = ?", "john").
		HandleStandardFirstRows(userRow)

	// Execute GORM First() query which will add ORDER BY id LIMIT 1
	var foundUser TestUser
	result := tc.DB().Where("username = ?", "john").First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "john", foundUser.Username)

	// The mock should match the query even with GORM's additions
	tc.VerifyExpectations()
}

// TestComplexGormQueryPatterns tests that our flexible pattern matching works
// with more complex GORM query patterns including joins and multiple conditions
func TestComplexGormQueryPatterns(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register models
	tc.RegisterModel(&TestUser{})
	tc.RegisterModel(&TestPost{})

	// Set up a count expectation with a complex WHERE clause
	// Our pattern matching should handle GORM's additional conditions
	tc.ForTable("test_users").
		ExpectCount().
		Where("active = ? AND username LIKE ?", true, "%john%").
		ReturnCount(5)

	// Execute GORM query with multiple conditions
	var count int64
	result := tc.DB().Model(&TestUser{}).
		Where("active = ? AND username LIKE ?", true, "%john%").
		Count(&count)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, int64(5), count)

	// The mock should match the query even with complex patterns
	tc.VerifyExpectations()
}

// TestSoftDeleteHandlingWithExplicitCondition tests that we don't add duplicate
// deleted_at conditions when the query already has one
func TestSoftDeleteHandlingWithExplicitCondition(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register a model with soft delete
	tc.RegisterModel(&TestUser{})

	// Create test data including a deleted user
	now := time.Now()
	deletedUser := TestUser{
		Model:    gorm.Model{ID: 2, CreatedAt: now, UpdatedAt: now, DeletedAt: gorm.DeletedAt{Time: now, Valid: true}},
		Username: "deleted_user",
		Email:    "deleted@example.com",
		Active:   false,
	}

	// Create row for the deleted user
	userRow := tc.BuildRowsFrom("test_users", deletedUser)

	// Set up an expectation with an explicit deleted_at condition
	// Use our specialized handler for deleted_at IS NOT NULL conditions
	tc.ForTable("test_users").
		ExpectFind().
		Where("username = ? AND deleted_at IS NOT NULL", "deleted_user").
		HandleDeletedNotNullRows(userRow)

	// Execute GORM query with Unscoped to include deleted records
	// and our explicit deleted_at condition
	var foundUser TestUser
	result := tc.DB().Unscoped().
		Where("username = ? AND deleted_at IS NOT NULL", "deleted_user").
		First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "deleted_user", foundUser.Username)
	assert.True(t, foundUser.DeletedAt.Valid)

	// The mock should match the query with our explicit deleted_at condition
	tc.VerifyExpectations()
}

// TestModelWithoutSoftDelete tests that our flexible pattern matching works
// with models that don't have soft delete
func TestModelWithoutSoftDelete(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Define a model without soft delete
	type SimpleModel struct {
		ID   uint `gorm:"primarykey"`
		Name string
	}

	// Register the model without soft delete
	tc.RegisterModel(&SimpleModel{})

	// Set up a find expectation - framework should detect no deleted_at field
	// and not add a soft delete condition
	rows := sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Simple Model")
	tc.ForTable("simple_models").
		ExpectFind().
		ByID(1).
		ReturnRows(rows)

	// Execute GORM query
	var model SimpleModel
	result := tc.DB().First(&model, 1)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "Simple Model", model.Name)

	// The mock should match the query without any deleted_at condition
	tc.VerifyExpectations()
}

// TestFirstWithComplexConditions tests First() with multiple AND/OR conditions
func TestFirstWithComplexConditions(t *testing.T) {
	// Create a test context with SQL debug enabled to help troubleshoot pattern matching
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model with soft delete
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()

	// Create row for the user
	userRow := sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "username", "email", "active"}).
		AddRow(1, now, now, nil, "complexuser", "complex@example.com", true)

	// Use Raw with exact SQL pattern
	tc.Raw().ExpectQuery("^SELECT \\* FROM `test_users` WHERE \\(\\(username = \\? OR email = \\?\\) AND active = \\?\\) AND `test_users`.`deleted_at` IS NULL ORDER BY `test_users`.`id` LIMIT 1$").
		WithArgs("complexuser", "complex@example.com", true).
		WillReturnRows(userRow)

	// Execute GORM query with complex condition
	var foundUser TestUser
	result := tc.DB().Where("(username = ? OR email = ?) AND active = ?",
		"complexuser", "complex@example.com", true).First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "complexuser", foundUser.Username)

	// The mock should match the query with complex conditions
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// TestFirstWithJoins tests First() with table joins
func TestFirstWithJoins(t *testing.T) {
	// Create a test context with SQL debug enabled
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register models
	tc.RegisterModel(&TestUser{})
	tc.RegisterModel(&TestPost{})

	// Create test data
	now := time.Now()

	// Create a more accurate struct for the results
	type JoinResult struct {
		ID        uint
		CreatedAt time.Time
		UpdatedAt time.Time
		DeletedAt gorm.DeletedAt
		Username  string
		Email     string
		Active    bool
		PostTitle string
	}

	// Create mock result row with appropriate fields that match what our SELECT clause will return
	joinRow := sqlmock.NewRows([]string{
		"id", "created_at", "updated_at", "deleted_at",
		"username", "email", "active",
		"post_title"}).
		AddRow(1, now, now, nil, "joinuser", "join@example.com", true, "Test Post")

	// We need to use Raw() for complex joins with a pattern that exactly matches what GORM generates
	tc.Raw().ExpectQuery("^SELECT test_users.\\*, test_posts.title as post_title FROM `test_users` JOIN test_posts ON test_posts.user_id = test_users.id WHERE test_users.id = \\? AND `test_users`.`deleted_at` IS NULL ORDER BY `test_users`.`id` LIMIT 1$").
		WithArgs(1).
		WillReturnRows(joinRow)

	// Execute GORM query with join
	var result JoinResult
	queryResult := tc.DB().Table("test_users").
		Select("test_users.*, test_posts.title as post_title").
		Joins("JOIN test_posts ON test_posts.user_id = test_users.id").
		Where("test_users.id = ?", 1).
		First(&result)

	// Assertions
	assert.NoError(t, queryResult.Error)
	assert.Equal(t, "joinuser", result.Username)
	assert.Equal(t, "Test Post", result.PostTitle)

	// Verify expectations
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// TestFirstWithPreload tests First() with preloaded associations
func TestFirstWithPreload(t *testing.T) {
	// Create a test context with SQL debug
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register models
	tc.RegisterModel(&TestUser{})
	tc.RegisterModel(&TestPost{})

	// Create test data
	now := time.Now()

	// User row
	userRow := sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "username", "email", "active"}).
		AddRow(1, now, now, nil, "preloaduser", "preload@example.com", true)

	// Posts row for preloading
	postsRow := sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "content", "user_id", "author_id"}).
		AddRow(1, now, now, nil, "First Post", "Content 1", 1, 1).
		AddRow(2, now, now, nil, "Second Post", "Content 2", 1, 1)

	// Use Raw for precise control
	// Main user query with First()
	tc.Raw().ExpectQuery("^SELECT `test_users`.`id`,`test_users`.`created_at`,`test_users`.`updated_at`,`test_users`.`deleted_at`,`test_users`.`username`,`test_users`.`email`,`test_users`.`active` FROM `test_users` WHERE `test_users`.`id` = \\? AND `test_users`.`deleted_at` IS NULL ORDER BY `test_users`.`id` LIMIT 1$").
		WithArgs(1).
		WillReturnRows(userRow)

	// Then for the preloaded posts
	tc.Raw().ExpectQuery("^SELECT \\* FROM `test_posts` WHERE `test_posts`.`user_id` = \\? AND `test_posts`.`deleted_at` IS NULL$").
		WithArgs(1).
		WillReturnRows(postsRow)

	// Define a model with association
	type UserWithPosts struct {
		TestUser
		Posts []TestPost `gorm:"foreignKey:UserID"`
	}

	// Execute GORM query with preload
	var user UserWithPosts
	result := tc.DB().Model(&TestUser{}).Preload("Posts").First(&user, 1)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "preloaduser", user.Username)

	// Verify expectations
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// TestFirstWithExplicitOrder tests First() with explicit Order clause
func TestFirstWithExplicitOrder(t *testing.T) {
	// Create a test context with SQL debug
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()

	// Create row for the user
	userRow := sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "username", "email", "active"}).
		AddRow(1, now, now, nil, "orderuser", "order@example.com", true)

	// For custom ORDER BY clauses, we need to use Raw() to match the exact SQL
	// GORM will generate a query with the custom ORDER BY first, then add the default id ORDER BY
	tc.Raw().ExpectQuery("^SELECT \\* FROM `test_users` WHERE `test_users`.`deleted_at` IS NULL ORDER BY username DESC,`test_users`.`id` LIMIT 1$").
		WillReturnRows(userRow)

	// Execute GORM query with explicit Order and First
	var foundUser TestUser
	result := tc.DB().Order("username DESC").First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "orderuser", foundUser.Username)

	// Verify expectations
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// TestFirstWithORCondition tests First() with OR conditions
func TestFirstWithORCondition(t *testing.T) {
	// Create a test context with SQL debug
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()

	// Create row for the user
	userRow := sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "username", "email", "active"}).
		AddRow(1, now, now, nil, "oruser", "or@example.com", true)

	// Use Raw with the exact SQL pattern that GORM generates
	tc.Raw().ExpectQuery("^SELECT \\* FROM `test_users` WHERE \\(username = \\? OR email = \\?\\) AND `test_users`.`deleted_at` IS NULL ORDER BY `test_users`.`id` LIMIT 1$").
		WithArgs("oruser", "or@example.com").
		WillReturnRows(userRow)

	// Execute GORM query with OR and First
	var foundUser TestUser
	result := tc.DB().Where("username = ? OR email = ?", "oruser", "or@example.com").First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "oruser", foundUser.Username)

	// Verify expectations
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// TestHandleStandardFirstRows tests the new specialized handler for First() queries
func TestHandleStandardFirstRows(t *testing.T) {
	// Create a test context with SQL debug
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()
	user := TestUser{
		Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Username: "specialized_handler_user",
		Email:    "specialized@example.com",
		Active:   true,
	}

	// Create row for the user
	userRow := tc.BuildRowsFrom("test_users", user)

	// Use the specialized handler for First() queries
	tc.ForTable("test_users").
		ExpectFind().
		Where("username = ?", "specialized_handler_user").
		HandleStandardFirstRows(userRow)

	// Execute GORM query with First()
	var foundUser TestUser
	result := tc.DB().Where("username = ?", "specialized_handler_user").First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "specialized_handler_user", foundUser.Username)
	assert.Equal(t, "specialized@example.com", foundUser.Email)

	// Verify expectations
	tc.VerifyExpectations()
}

// TestHandleDeletedNotNullRows tests the specialized handler for deleted_at IS NOT NULL queries
func TestHandleDeletedNotNullRows(t *testing.T) {
	// Create a test context with SQL debug
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Create test data for a soft-deleted user
	now := time.Now()
	deletedUser := TestUser{
		Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now, DeletedAt: gorm.DeletedAt{Time: now, Valid: true}},
		Username: "deleted_handler_user",
		Email:    "deleted_handler@example.com",
		Active:   false,
	}

	// Create row for the deleted user
	userRow := tc.BuildRowsFrom("test_users", deletedUser)

	// Use the specialized handler for deleted_at IS NOT NULL queries
	tc.ForTable("test_users").
		ExpectFind().
		Where("username = ? AND deleted_at IS NOT NULL", "deleted_handler_user").
		HandleDeletedNotNullRows(userRow)

	// Execute GORM query with Unscoped to include deleted records
	var foundUser TestUser
	result := tc.DB().Unscoped().
		Where("username = ? AND deleted_at IS NOT NULL", "deleted_handler_user").
		First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "deleted_handler_user", foundUser.Username)
	assert.True(t, foundUser.DeletedAt.Valid)

	// Verify expectations
	tc.VerifyExpectations()
}

// TestHandleDeletedAtRows tests the specialized handler for deleted_at IS NULL queries
func TestHandleDeletedAtRows(t *testing.T) {
	// Create a test context with SQL debug
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()
	user := TestUser{
		Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Username: "deleted_at_handler_user",
		Email:    "deleted_at_handler@example.com",
		Active:   true,
	}

	// Create row for the user
	userRow := tc.BuildRowsFrom("test_users", user)

	// Use the specialized handler for deleted_at IS NULL queries
	tc.ForTable("test_users").
		ExpectFind().
		Where("deleted_at IS NULL").
		WithDeletedAt().
		HandleDeletedAtRows(userRow)

	// Execute GORM query
	var foundUser TestUser
	result := tc.DB().Where("deleted_at IS NULL").First(&foundUser)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "deleted_at_handler_user", foundUser.Username)

	// Verify expectations
	tc.VerifyExpectations()
}

// TestWithDeletedAtAndByIDAndFirst tests the specific combination of WithDeletedAt, ByID, and First
// that was reported in the bug. This tests that our SQL pattern correctly matches GORM's actual
// query pattern when all three methods are used together.
func TestWithDeletedAtAndByIDAndFirst(t *testing.T) {
	// Create a test context with SQL debug enabled
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model with soft delete
	tc.RegisterModel(&TestUser{})

	// Create test data
	now := time.Now()
	user := TestUser{
		Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Username: "combined_methods_user",
		Email:    "combined@example.com",
		Active:   true,
	}

	// Create row for the user
	userRow := tc.BuildRowsFrom("test_users", user)

	// Set up the test case with the combined methods
	tc.ForTable("test_users").
		ExpectFind().
		ByID(uint(1)).   // Filter by primary key
		WithDeletedAt(). // Handle deleted_at IS NULL condition
		First().         // Add ORDER BY and LIMIT 1
		ReturnRows(userRow)

	// Execute GORM query with First() and ID
	var foundUser TestUser
	result := tc.DB().First(&foundUser, 1)

	// Assertions
	assert.NoError(t, result.Error)
	assert.Equal(t, "combined_methods_user", foundUser.Username)
	assert.Equal(t, "combined@example.com", foundUser.Email)

	// Verify expectations
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// TestWithDeletedAtAndByIDAndFirstError tests the combination of WithDeletedAt, ByID, and First
// with ReturnError. This ensures our pattern matching works correctly with both ReturnRows and ReturnError.
func TestWithDeletedAtAndByIDAndFirstError(t *testing.T) {
	// Create a test context with SQL debug enabled
	tc := NewDBTestContext(t, WithSQLDebug())
	defer tc.Teardown()

	// Register a model with soft delete
	tc.RegisterModel(&TestUser{})

	// Create a test error
	testErr := errors.New("record not found")

	// Set up the test case with the combined methods
	tc.ForTable("test_users").
		ExpectFind().
		ByID(uint(999)).     // Filter by primary key (non-existent ID)
		WithDeletedAt().     // Handle deleted_at IS NULL condition
		First().             // Add ORDER BY and LIMIT 1
		ReturnError(testErr) // Return an error

	// Execute GORM query with First() and ID
	var foundUser TestUser
	result := tc.DB().First(&foundUser, 999)

	// Assertions
	assert.Error(t, result.Error)
	assert.Equal(t, testErr, result.Error)
	assert.Empty(t, foundUser.Username) // User should not be populated

	// Verify expectations
	tc.VerifyExpectations()

	// Log the diagnostics for debugging
	if diag := tc.GetLastSQLDiagnostics(); diag != "" {
		t.Logf("SQL Diagnostics: %s", diag)
	}
}

// Define a test model for our test post
type TestPost struct {
	gorm.Model
	Title    string
	Content  string
	UserID   uint
	AuthorID uint
}
