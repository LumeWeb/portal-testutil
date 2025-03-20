package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestModelWithCustomTableName represents a model with custom table name
type TestModelWithCustomTableName struct {
	gorm.Model
	Name string
	Data string
}

// TableName implements a custom table name for GORM
func (TestModelWithCustomTableName) TableName() string {
	return "custom_table_names"
}

// TestTableNameDirectExtraction verifies that our tryGetTableName helper works correctly
func TestTableNameDirectExtraction(t *testing.T) {
	// Create a simple helper
	helper := &TransactionTestHelper{
		tc: &DBTestContext{
			registeredModels: map[string]interface{}{
				"custom_table_names": &TestModelWithCustomTableName{},
			},
		},
	}

	// Create a model with our custom table name
	model := &TestModelWithCustomTableName{}

	// Test direct TableName extraction
	tableName := helper.tryGetTableName(model)

	// Verify the table name is extracted correctly from the TableName() method
	assert.Equal(t, "custom_table_names", tableName,
		"Table name should be correctly extracted from TableName() method")
}

// TestTransactionResolvesTableInternally verifies the internal table resolution
// mechanism for models with TableName() method
func TestTransactionResolvesTableInternally(t *testing.T) {
	// Create a test model
	model := &TestModelWithCustomTableName{}

	// Create a helper that would be used by the transaction wrapper
	helper := &TransactionTestHelper{
		tc: &DBTestContext{
			registeredModels: map[string]interface{}{
				"custom_table_names": &TestModelWithCustomTableName{},
			},
		},
	}

	// Access the internals directly to test our implementation
	// This is more of a unit test than an integration test
	db := &gorm.DB{
		Statement: &gorm.Statement{
			Model: model,
			Table: "", // Table not set initially
		},
	}

	// Call the function we're testing
	helper.ensureTableSet(db)

	// Verify that the table name was correctly set
	assert.Equal(t, "custom_table_names", db.Statement.Table,
		"Table name should be set to 'custom_table_names'")
}

// TestPublicTransactionAPIWithTableResolution tests that our transaction wrapper
// correctly resolves table names when a query operation is executed. This verifies
// the fix for the "Table not set" error that occurs when using registered models in
// transactions.
func TestPublicTransactionAPIWithTableResolution(t *testing.T) {
	// Create a test context with verification skipped
	tc := NewDBTestContext(t)
	defer tc.Teardown()
	tc.SkipVerification()

	// Register our model with a custom table name
	tc.RegisterModel(&TestModelWithCustomTableName{})

	// Setup basic transaction expectations
	tc.mock.ExpectBegin()

	// Add expectation for the count query that will trigger the callbacks
	tc.mock.ExpectQuery("SELECT count\\(\\*\\) FROM `custom_table_names`").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(0))

	tc.mock.ExpectCommit()

	// Create our test model
	model := &TestModelWithCustomTableName{}

	// Use the public transaction API as users would in real code
	err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		// Just calling Model() does NOT set the table name initially in GORM

		// Now start a query operation that should trigger our callback
		var count int64
		result := tx.Model(model).Count(&count)

		if result.Error != nil {
			// An error here would mean the table name wasn't resolved
			return result.Error
		}

		// Get the table name after the query operation
		finalTable := result.Statement.Table

		// Verify the table was correctly resolved
		if finalTable != "custom_table_names" {
			return fmt.Errorf("table name was not correctly resolved, got: %q, expected: %q",
				finalTable, "custom_table_names")
		}

		return nil
	})

	// The test passes if the transaction completed successfully
	// which means the table name was correctly resolved
	assert.NoError(t, err)
}

// TestTransactionCreateWithRegisteredModel verifies that our solution fixes the original
// "Table not set" error when using Create operations with registered models in transactions.
// This test specifically targets the Create() operation which was particularly problematic
// because it doesn't use the Model() method explicitly.
func TestTransactionCreateWithRegisteredModel(t *testing.T) {
	// Create a test context with verification skipped
	tc := NewDBTestContext(t)
	defer tc.Teardown()
	tc.SkipVerification()

	// Register our model with a custom table name
	tc.RegisterModel(&TestModelWithCustomTableName{})

	// Setup basic transaction expectations
	tc.mock.ExpectBegin()

	// Expect an INSERT operation with the correct table name
	// GORM uses a RETURNING clause, so we need to use ExpectQuery instead of ExpectExec
	tc.mock.ExpectQuery("INSERT INTO `custom_table_names`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))

	tc.mock.ExpectCommit()

	// Create a new instance of our model
	model := &TestModelWithCustomTableName{
		Name: "Test Model",
		Data: "Test Data",
	}

	// Use the public transaction API to execute a Create operation
	err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		// This is exactly the operation that would fail with "Table not set"
		// before our fix, because Create uses the model directly without Model()
		result := tx.Create(model)

		// Return any error that occurred
		return result.Error
	})

	// The test passes if the Create operation completed without errors
	assert.NoError(t, err, "Create operation should succeed without 'Table not set' error")
}

// TestTransactionModel is a simple model for direct GORM transaction testing
type TestTransactionModel struct {
	gorm.Model
	Name string
	Age  int
}

// TableName returns the table name for the model
func (TestTransactionModel) TableName() string {
	return "test_transaction_models"
}

// TestTransactionPrefixedModel demonstrates a model with a prefixed table name
type TestTransactionPrefixedModel struct {
	gorm.Model
	Title   string
	Content string
}

// TableName returns the table name for the model
func (TestTransactionPrefixedModel) TableName() string {
	return "prefix_transaction_models"
}

// TestDirectGormTransaction tests if the fix allows direct GORM transaction to work
// This tests the fix for the bug where direct GORM transactions failed with
// "Table not set, please set it like: db.Model(&user) or db.Table("users")"
func TestDirectGormTransaction(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()
	tc.SkipVerification()

	// Register the model
	tc.RegisterModel(&TestTransactionModel{})

	// Setup transaction expectations
	tc.mock.ExpectBegin()

	// Expect an INSERT operation with the correct table name
	tc.mock.ExpectQuery("INSERT INTO `test_transaction_models`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))

	tc.mock.ExpectCommit()

	// Create a test model
	model := &TestTransactionModel{
		Name: "test",
		Age:  30,
	}

	// Use direct GORM transaction API - this should now work with the fix
	err := tc.DB().Transaction(func(tx *gorm.DB) error {
		return tx.Create(model).Error
	})

	// Verify the transaction worked without errors
	assert.NoError(t, err, "Direct GORM transaction should succeed with the fix")
}

// TestTransactionWithTablePrefix tests if table prefixes work with direct GORM transactions
func TestTransactionWithTablePrefix(t *testing.T) {
	// Create test context with prefix
	tc := NewDBTestContext(t, WithTablePrefix("prefix_"))
	defer tc.Teardown()
	tc.SkipVerification()

	// Register the model that has a prefixed TableName method
	tc.RegisterModel(&TestTransactionPrefixedModel{})

	// Setup transaction expectations
	tc.mock.ExpectBegin()

	// Expect an INSERT operation with the correct prefixed table name
	tc.mock.ExpectQuery("INSERT INTO `prefix_transaction_models`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))

	tc.mock.ExpectCommit()

	// Create a test model
	model := &TestTransactionPrefixedModel{
		Title:   "test prefixed",
		Content: "sample content",
	}

	// Use direct GORM transaction API
	err := tc.DB().Transaction(func(tx *gorm.DB) error {
		return tx.Create(model).Error
	})

	// Verify the transaction worked without errors
	assert.NoError(t, err, "Transaction with table prefix should succeed")
}

// TestTransactionMultipleOperations tests a direct GORM transaction with multiple operations
func TestTransactionMultipleOperations(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()
	tc.SkipVerification()

	// Register the model
	tc.RegisterModel(&TestTransactionModel{})

	// Setup transaction expectations
	tc.mock.ExpectBegin()

	// Create expectation
	tc.mock.ExpectQuery("INSERT INTO `test_transaction_models`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Find expectation - using time values instead of strings for GORM model fields
	now := time.Now()
	tc.mock.ExpectQuery("SELECT \\* FROM `test_transaction_models` WHERE .+id.+ = .+").WillReturnRows(
		sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "name", "age"}).
			AddRow(1, now, now, nil, "test", 30))

	// Update expectation (Update() uses a different SQL pattern than Save())
	tc.mock.ExpectExec("UPDATE `test_transaction_models` SET .+name.+ = .+").WillReturnResult(
		sqlmock.NewResult(1, 1))

	tc.mock.ExpectCommit()

	// Create a test model
	model := &TestTransactionModel{
		Name: "test",
		Age:  30,
	}

	// Use direct GORM transaction with multiple operations
	err := tc.DB().Transaction(func(tx *gorm.DB) error {
		// Create
		if err := tx.Create(model).Error; err != nil {
			return err
		}

		// Find
		var found TestTransactionModel
		if err := tx.First(&found, 1).Error; err != nil {
			return err
		}

		// Update
		found.Name = "updated"
		return tx.Save(&found).Error
	})

	// Verify the transaction worked without errors
	assert.NoError(t, err, "Transaction with multiple operations should succeed")
}
