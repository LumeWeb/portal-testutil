package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Test models with different TableName implementations
type ValueReceiverModel struct {
	gorm.Model
	Name string
}

func (ValueReceiverModel) TableName() string {
	return "value_models"
}

type PointerReceiverModel struct {
	gorm.Model
	Name string
}

func (*PointerReceiverModel) TableName() string {
	return "pointer_models"
}

type NoTableNameModel struct {
	gorm.Model
	Name string
}

// TestTableNameResolution tests the table name resolution fixes
func TestTableNameResolution(t *testing.T) {
	// Create test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()

	// Create transaction helper
	th := NewTransactionTestHelper(testCtx)

	// Test value receiver model
	t.Run("ValueReceiverModel", func(t *testing.T) {
		// Get table name from value receiver
		model := ValueReceiverModel{Name: "test"}
		tableName := th.tryGetTableName(model)
		assert.Equal(t, "value_models", tableName, "Should correctly get table name from value receiver")

		// Get table name from pointer to value receiver
		ptrModel := &ValueReceiverModel{Name: "test"}
		tableName = th.tryGetTableName(ptrModel)
		assert.Equal(t, "value_models", tableName, "Should correctly get table name from pointer to value receiver")
	})

	// Test pointer receiver model
	t.Run("PointerReceiverModel", func(t *testing.T) {
		// Get table name from pointer receiver
		model := &PointerReceiverModel{Name: "test"}
		tableName := th.tryGetTableName(model)
		assert.Equal(t, "pointer_models", tableName, "Should correctly get table name from pointer receiver")

		// Get table name from value with pointer receiver
		valModel := PointerReceiverModel{Name: "test"}
		tableName = th.tryGetTableName(valModel)
		assert.Equal(t, "pointer_models", tableName, "Should correctly get table name from value with pointer receiver")
	})

	// Test nil pointer handling
	t.Run("NilPointerHandling", func(t *testing.T) {
		// Create nil pointer to model
		var nilModel *ValueReceiverModel
		tableName := th.tryGetTableName(nilModel)
		assert.Equal(t, "value_models", tableName, "Should handle nil pointer and still get table name")

		// Create nil pointer with pointer receiver
		var nilPtrModel *PointerReceiverModel
		tableName = th.tryGetTableName(nilPtrModel)
		assert.Equal(t, "pointer_models", tableName, "Should handle nil pointer with pointer receiver")
	})
}

// TestTableResolutionInTransaction tests that our fixes properly resolve table names in transactions
func TestTableResolutionInTransaction(t *testing.T) {
	// Create test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()

	// Register models
	testCtx.RegisterModel(&ValueReceiverModel{})
	testCtx.RegisterModel(&PointerReceiverModel{})
	testCtx.RegisterModel(&NoTableNameModel{})

	// Test value receiver model
	t.Run("ValueReceiverModel", func(t *testing.T) {
		// Set expectation - using false to disable automatic transaction handling
		// because ExecuteInTransaction will handle the Begin/Commit
		testCtx.ForTable("value_models").ExpectInsert(1, false)

		// Execute in transaction
		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&ValueReceiverModel{Name: "test"}).Error
		})

		// Should succeed
		assert.NoError(t, err, "Transaction should succeed with value receiver model")
	})

	// Test pointer receiver model
	t.Run("PointerReceiverModel", func(t *testing.T) {
		// Set expectation - using false to disable automatic transaction handling
		testCtx.ForTable("pointer_models").ExpectInsert(2, false)

		// Execute in transaction
		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&PointerReceiverModel{Name: "test"}).Error
		})

		// Should succeed
		assert.NoError(t, err, "Transaction should succeed with pointer receiver model")
	})

	// Test direct GORM transaction
	t.Run("DirectGORMTransaction", func(t *testing.T) {
		// For direct GORM transaction, we can use the automatic transaction handling
		// provided by the updated ExpectInsert method
		testCtx.ForTable("value_models").ExpectInsert(3)

		// Execute direct GORM transaction
		err := testCtx.DB().Transaction(func(tx *gorm.DB) error {
			return tx.Create(&ValueReceiverModel{Name: "test2"}).Error
		})

		// Should succeed
		assert.NoError(t, err, "Direct GORM transaction should succeed")
	})

	// Test slice of models
	t.Run("SliceOfModels", func(t *testing.T) {
		// Set expectation - using false to disable automatic transaction handling
		testCtx.ForTable("value_models").ExpectInsert(4, false)

		// Create slice of models
		models := []ValueReceiverModel{
			{Name: "test1"},
			{Name: "test2"},
		}

		// Execute in transaction
		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&models).Error
		})

		// Should succeed
		assert.NoError(t, err, "Transaction should succeed with slice of models")
	})

	// Test with explicit table
	t.Run("ExplicitTable", func(t *testing.T) {
		// Set expectation - using false to disable automatic transaction handling
		testCtx.ForTable("custom_table").ExpectInsert(5, false)

		// Execute in transaction with explicit table
		err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Table("custom_table").Create(&ValueReceiverModel{Name: "explicit"}).Error
		})

		// Should succeed
		assert.NoError(t, err, "Transaction should succeed with explicit table")
	})
}

// TestCallbackDeduplication tests that our fix for callback deduplication works
func TestCallbackDeduplication(t *testing.T) {
	// Create test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()

	// Register model
	testCtx.RegisterModel(&ValueReceiverModel{})

	// Create multiple transaction helpers
	th1 := NewTransactionTestHelper(testCtx)
	th2 := NewTransactionTestHelper(testCtx)

	// This would cause duplicate callback warnings without our fix
	tx1 := th1.wrapTransactionWithTableInfo(testCtx.DB())
	tx2 := th2.wrapTransactionWithTableInfo(testCtx.DB())

	// Basic checks
	assert.NotNil(t, tx1, "First transaction wrapper should not be nil")
	assert.NotNil(t, tx2, "Second transaction wrapper should not be nil")

	// For direct transaction usage, we need to expect transaction Begin/Commit pairs
	testCtx.mock.ExpectBegin()
	testCtx.ForTable("value_models").ExpectInsert(1)
	testCtx.mock.ExpectCommit()

	testCtx.mock.ExpectBegin()
	testCtx.ForTable("value_models").ExpectInsert(2)
	testCtx.mock.ExpectCommit()

	// Use both transactions with Begin/Commit
	err1 := tx1.Begin().Create(&ValueReceiverModel{Name: "test1"}).Commit().Error
	err2 := tx2.Begin().Create(&ValueReceiverModel{Name: "test2"}).Commit().Error

	// Both should succeed
	assert.NoError(t, err1, "First transaction should succeed")
	assert.NoError(t, err2, "Second transaction should succeed")
}
