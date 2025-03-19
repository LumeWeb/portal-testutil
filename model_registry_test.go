package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Define test model structures for our tests
type TestUser struct {
	gorm.Model
	Username string
	Email    string
	Active   bool
}

// A model with a custom TableName method
type TestCustomTable struct {
	ID        uint
	Name      string
	CreatedAt time.Time
}

func (TestCustomTable) TableName() string {
	return "custom_tablename"
}

// A model that respects the table prefix
type TestPrefixedModel struct {
	gorm.Model
	Description string
	Status      string
}

func (TestPrefixedModel) TableName() string {
	// Return the non-prefixed name - the framework should add the prefix
	return "prefixed_models"
}

func TestRegisterModel(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Verify the model was registered
	models := tc.GetRegisteredTableNames()
	assert.Contains(t, models, "test_users")
}

func TestRegisterModels(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Register multiple models
	tc.RegisterModels(&TestUser{}, &TestCustomTable{})

	// Verify all models were registered
	models := tc.GetRegisteredTableNames()
	assert.Contains(t, models, "test_users")
	assert.Contains(t, models, "custom_tablename")
}

func TestSetupModels(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Create a slice of models
	modelSlice := []interface{}{&TestUser{}, &TestCustomTable{}}

	// Register models using SetupModels
	tc.SetupModels(modelSlice)

	// Verify all models were registered
	models := tc.GetRegisteredTableNames()
	assert.Contains(t, models, "test_users")
	assert.Contains(t, models, "custom_tablename")
}

func TestTableNameForModel(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Test standard model
	tableName := tc.TableNameForModel(&TestUser{})
	assert.Equal(t, "test_users", tableName)

	// Test custom table name model
	customTableName := tc.TableNameForModel(&TestCustomTable{})
	assert.Equal(t, "custom_tablename", customTableName)
}

func TestPrefixTableName(t *testing.T) {
	// Create a test context with a prefix
	prefix := "prefix_"
	tc := NewDBTestContext(t, WithTablePrefix(prefix))

	// Test adding prefix to a table name
	prefixedName := tc.PrefixTableName("users")
	assert.Equal(t, "prefix_users", prefixedName)

	// Test table name that already has the prefix
	alreadyPrefixed := tc.PrefixTableName("prefix_items")
	assert.Equal(t, "prefix_items", alreadyPrefixed)
}

func TestPluralize(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"user", "users"},
		{"class", "classes"},
		{"box", "boxes"},
		{"dish", "dishes"},
		{"church", "churches"},
		{"quiz", "quizes"},
		{"city", "cities"},
		{"key", "keys"},
		{"family", "families"},
		{"day", "days"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := pluralize(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestForTableWithRegisteredModels(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Create an expectations builder for the registered table
	builder := tc.ForTable("test_users")

	// Verify the builder was created with the right table
	assert.NotNil(t, builder)
	assert.Equal(t, "test_users", builder.table)
}

func TestExtractTableName(t *testing.T) {
	// Create a test context with a real t for the normal cases
	tc := NewDBTestContext(t)

	// Test with a regular struct
	regularTableName := tc.extractTableName(&TestUser{})
	assert.Equal(t, "test_users", regularTableName)

	// Test with a custom TableName struct
	customTableName := tc.extractTableName(&TestCustomTable{})
	assert.Equal(t, "custom_tablename", customTableName)
}

func TestExpectationsWithRegisteredModels(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)

	// Register a model
	tc.RegisterModel(&TestUser{})

	// Set up an expectation for the model's table
	tc.ForTable("test_users").ExpectCount(5)

	// Get the DB and execute a count query
	var count int64
	tc.DB().Model(&TestUser{}).Count(&count)

	// Verify expectations
	tc.VerifyExpectations()

	// Also verify the count value was correct
	assert.Equal(t, int64(5), count)
}

// For testing prefixed tables with non-TableName models
type TestUserWithPrefix struct {
	gorm.Model
	Username string
	Email    string
	Active   bool
}

// TableName returns a custom table name that includes our test prefix
func (TestUserWithPrefix) TableName() string {
	return "prefix_test_users"
}

func TestModelRegistrationWithPrefix(t *testing.T) {
	// Create a test context with a prefix
	tc := NewDBTestContext(t, WithTablePrefix("prefix_"))

	// Register our special prefixed model - this handles the discrepancy
	// between what ForTable generates and what GORM would use
	tc.RegisterModel(&TestUserWithPrefix{})

	// The table name should be prefixed for regular models
	regularTableName := tc.ForTable("test_users").table
	assert.Equal(t, "prefix_test_users", regularTableName)

	// For models with the TableName method that returns the prefixed name,
	// we should be able to use ForTable with the base name and it should work
	tc.ForTable("test_users").
		ExpectCount().
		Where("`prefix_test_users`.`deleted_at` IS NULL").
		ReturnCount(3)

	// Run a count query using the model with TableName that returns the prefixed name
	var count int64
	tc.DB().Model(&TestUserWithPrefix{}).Count(&count)

	// Verify expectations
	tc.VerifyExpectations()

	// Verify the count value was correct
	assert.Equal(t, int64(3), count)
}
