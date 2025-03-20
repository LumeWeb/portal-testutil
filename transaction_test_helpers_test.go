package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Test models for the complex validation case
type TestParent struct {
	gorm.Model
	Name string
}

func (TestParent) TableName() string {
	return "test_parents"
}

// Type definitions for enums
type TestType string
type TestDirection string

const (
	TypeOne TestType      = "one"
	TypeTwo TestType      = "two"
	DirIn   TestDirection = "in"
	DirOut  TestDirection = "out"
)

// TestChild model with relationship AND hooks that call other methods
type TestChild struct {
	gorm.Model
	ParentID  uint
	Parent    TestParent `gorm:"foreignKey:ParentID"`
	Type      TestType
	Direction TestDirection
	Content   string
	ThreadID  string
}

func (TestChild) TableName() string {
	return "test_children"
}

// This hook calls another method - key to reproducing the issue
func (c *TestChild) BeforeCreate(tx *gorm.DB) error {
	return c.Validate()
}

func (c *TestChild) BeforeUpdate(tx *gorm.DB) error {
	return c.Validate()
}

// Complex validation method with map checks
func (c *TestChild) Validate() error {
	// Content validation
	if c.Content == "" {
		return fmt.Errorf("content is required")
	}

	// Type validation using map
	validTypes := map[TestType]bool{
		TypeOne: true,
		TypeTwo: true,
	}
	if !validTypes[c.Type] {
		return fmt.Errorf("invalid type: %s", c.Type)
	}

	// Direction validation using map
	validDirs := map[TestDirection]bool{
		DirIn:  true,
		DirOut: true,
	}
	if !validDirs[c.Direction] {
		return fmt.Errorf("invalid direction: %s", c.Direction)
	}

	return nil
}

// TestModelsWithRelationshipsAndValidation verifies that models with both relationships
// and validation hooks work correctly in transactions without "Table not set" errors
func TestModelsWithRelationshipsAndValidation(t *testing.T) {
	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register models with relationships
	tc.RegisterModel(&TestChild{})
	tc.RegisterModel(&TestParent{})

	// Mock expectation for create operation
	tc.ForTable("test_children").ExpectCreate(1)

	// Execute test with complex model - should use our fix
	err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		child := &TestChild{
			ParentID:  1,
			Type:      TypeOne,
			Direction: DirIn,
			Content:   "Test content",
			ThreadID:  "thread-123",
		}

		// This should succeed without "Table not set" error now
		return tx.Create(child).Error
	})

	// Verify no error occurred
	assert.NoError(t, err, "Create with complex model should succeed")
}

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

	// Add expectation for the count query using table helper
	tc.ForTable("custom_table_names").ExpectCount(0)

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

	// Register the model
	tc.RegisterModel(&TestTransactionModel{})

	// Use our transaction wrapper instead of direct GORM transaction
	txHelper := tc.Transaction()

	// Set up create expectations using table helpers
	tc.ForTable("test_transaction_models").ExpectCreate(1)

	// Set up find expectations - using builder for rows
	now := time.Now()
	rows := tc.BuildRows("test_transaction_models", map[string]any{
		"id":         1,
		"created_at": now,
		"updated_at": now,
		"deleted_at": nil,
		"name":       "test",
		"age":        30,
	})

	// Use HandleStandardFirstRows which is specifically designed for First(id)
	tc.ForTable("test_transaction_models").ExpectFind().
		Where("`test_transaction_models`.`id` = ?", 1).
		HandleStandardFirstRows(rows)

	// Set up update expectations - will match GORM's Save() method
	tc.ForTable("test_transaction_models").ExpectUpdate()

	// Create a test model
	model := &TestTransactionModel{
		Name: "test",
		Age:  30,
	}

	// Execute the transaction using our helper instead of direct GORM
	err := txHelper.ExecuteInTransaction(func(tx *gorm.DB) error {
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

// TableModelForTest is a model with custom TableName method for testing the transaction bug
type TableModelForTest struct {
	gorm.Model
	Field1 string
}

// TableName returns the custom table name
func (TableModelForTest) TableName() string {
	return "my_test_table"
}

// TestTableNameResolutionInTransaction tests the bug fix for table name resolution in transactions
// The v0.2.0 issue was that GORM models with TableName() methods would lose their table name
// information when used within transactions, resulting in "Table not set" errors
func TestTableNameResolutionInTransaction(t *testing.T) {
	t.Run("with direct RegisterModel", func(t *testing.T) {
		// Create test context
		testCtx := NewDBTestContext(t)
		defer testCtx.Teardown()
		testCtx.SkipVerification()

		// Register model directly
		testCtx.RegisterModel(&TableModelForTest{})

		// Create test model
		model := &TableModelForTest{Field1: "test"}

		// Set transaction expectations
		testCtx.mock.ExpectBegin()
		testCtx.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
			sqlmock.NewRows([]string{"id"}).AddRow(1))
		testCtx.mock.ExpectCommit()

		// This would fail with "Table not set" error before our fix
		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			// Without our fix, this would fail with "Table not set" error
			return tx.Create(model).Error
		})

		// The test passes if the execution doesn't throw a "Table not set" error
		assert.NoError(t, err, "Transaction should execute without 'Table not set' error")
	})

	t.Run("with RegisterModelWithRelationships", func(t *testing.T) {
		// Create test context
		testCtx := NewDBTestContext(t)
		defer testCtx.Teardown()
		testCtx.SkipVerification()

		// Register model with relationships API - as mentioned in the bug report
		RegisterModelWithRelationships[TableModelForTest](testCtx)

		// Create test model
		model := &TableModelForTest{Field1: "test"}

		// Set transaction expectations
		testCtx.mock.ExpectBegin()
		testCtx.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
			sqlmock.NewRows([]string{"id"}).AddRow(1))
		testCtx.mock.ExpectCommit()

		// This would fail with "Table not set" error before our fix
		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(model).Error
		})

		assert.NoError(t, err, "Transaction with RegisterModelWithRelationships should work now")
	})
}

// TestTableNameResolutionWithMapValues tests the edge case where a map destination is used in a transaction
// This specifically tests the fix for when ReflectValue is a map but the table name is lost
func TestTableNameResolutionWithMapValues(t *testing.T) {
	// Create test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()
	testCtx.SkipVerification()

	// Register model with relationships
	testCtx.RegisterModel(&TableModelForTest{})

	// Create a map with values to insert
	values := map[string]interface{}{
		"field1": "test value",
	}

	// Set transaction expectations
	testCtx.mock.ExpectBegin()
	testCtx.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))
	testCtx.mock.ExpectCommit()

	// Test with a map destination - should now work with our improved resolution
	err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		// This used to fail with "Table not set" because the model info was getting lost
		return tx.Model(&TableModelForTest{}).Create(values).Error
	})

	assert.NoError(t, err, "Transaction with map values should succeed")
}

// TestCallbackTracking tests the callback tracking logic that prevents duplicate
// warnings when registering callbacks
func TestCallbackTracking(t *testing.T) {
	// Create two test contexts to simulate multiple transactions
	testCtx1 := NewDBTestContext(t)
	defer testCtx1.Teardown()
	testCtx1.SkipVerification()

	testCtx2 := NewDBTestContext(t)
	defer testCtx2.Teardown()
	testCtx2.SkipVerification()

	// Create transaction helpers
	txHelper1 := NewTransactionTestHelper(testCtx1)
	txHelper2 := NewTransactionTestHelper(testCtx2)

	// Verify that each helper has a unique session ID
	assert.NotEqual(t, txHelper1.sessionID, txHelper2.sessionID,
		"Transaction helpers should have unique session IDs")

	// Register a model with each context
	testCtx1.RegisterModel(&TableModelForTest{})
	testCtx2.RegisterModel(&TableModelForTest{})

	// Create models
	model1 := &TableModelForTest{Field1: "test1"}
	model2 := &TableModelForTest{Field1: "test2"}

	// Set transaction expectations for both contexts
	testCtx1.mock.ExpectBegin()
	testCtx1.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))
	testCtx1.mock.ExpectCommit()

	testCtx2.mock.ExpectBegin()
	testCtx2.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(2))
	testCtx2.mock.ExpectCommit()

	// Execute transactions in sequence - this tests our callback tracking
	err1 := txHelper1.ExecuteInTransaction(func(tx *gorm.DB) error {
		return tx.Create(model1).Error
	})

	err2 := txHelper2.ExecuteInTransaction(func(tx *gorm.DB) error {
		return tx.Create(model2).Error
	})

	// Verify both transactions succeeded
	assert.NoError(t, err1, "First transaction should succeed")
	assert.NoError(t, err2, "Second transaction should succeed")

	// Verify that both transaction helpers have registered callbacks
	assert.Greater(t, len(txHelper1.registeredCallbacks), 0, "First helper should have registered callbacks")
	assert.Greater(t, len(txHelper2.registeredCallbacks), 0, "Second helper should have registered callbacks")

	// Verify that each helper has different callback names due to unique session IDs
	var helper1Names, helper2Names []string
	for name := range txHelper1.registeredCallbacks {
		helper1Names = append(helper1Names, name)
	}
	for name := range txHelper2.registeredCallbacks {
		helper2Names = append(helper2Names, name)
	}

	// Verify that no callback name appears in both helpers
	for _, name1 := range helper1Names {
		for _, name2 := range helper2Names {
			assert.NotEqual(t, name1, name2, "Callback names should be unique between helpers")
		}
	}
}

// TestUniqueCallbacksWithSameContext tests that even with the same test context,
// different transaction helpers have unique callback names to prevent duplicate warnings
func TestUniqueCallbacksWithSameContext(t *testing.T) {
	// Create test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()
	testCtx.SkipVerification()

	// Register model
	testCtx.RegisterModel(&TableModelForTest{})

	// Create two transaction helpers with the same context
	txHelper1 := NewTransactionTestHelper(testCtx)
	txHelper2 := NewTransactionTestHelper(testCtx)

	// Verify that each helper has a unique session ID even with the same context
	assert.NotEqual(t, txHelper1.sessionID, txHelper2.sessionID,
		"Transaction helpers should have unique session IDs even with the same context")

	// Set expectations for first transaction
	testCtx.mock.ExpectBegin()
	testCtx.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1))
	testCtx.mock.ExpectCommit()

	// Execute first transaction with first helper
	err1 := txHelper1.ExecuteInTransaction(func(tx *gorm.DB) error {
		return tx.Create(&TableModelForTest{Field1: "test1"}).Error
	})
	assert.NoError(t, err1, "First transaction should succeed")

	// Set expectations for second transaction
	testCtx.mock.ExpectBegin()
	testCtx.mock.ExpectQuery("INSERT INTO `my_test_table`").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(2))
	testCtx.mock.ExpectCommit()

	// Execute second transaction with second helper
	err2 := txHelper2.ExecuteInTransaction(func(tx *gorm.DB) error {
		return tx.Create(&TableModelForTest{Field1: "test2"}).Error
	})
	assert.NoError(t, err2, "Second transaction should succeed")

	// Get callback names from both helpers
	var helper1Callbacks, helper2Callbacks []string
	for name := range txHelper1.registeredCallbacks {
		helper1Callbacks = append(helper1Callbacks, name)
	}
	for name := range txHelper2.registeredCallbacks {
		helper2Callbacks = append(helper2Callbacks, name)
	}

	// Despite using the same test context, each helper should have unique callback names
	// This is what prevents duplicate callback warnings
	for _, name1 := range helper1Callbacks {
		for _, name2 := range helper2Callbacks {
			assert.NotEqual(t, name1, name2, "Callback names should be unique between helpers with same context")
		}
	}
}
