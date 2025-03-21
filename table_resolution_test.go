package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestTableResolutionIntegration tests the table resolution fix with models
// defined directly in the test file to avoid dependency issues.
func TestTableResolutionIntegration(t *testing.T) {
	// Create test context with mock DB
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register all models from our unit test file
	tc.RegisterModel(&SimpleModelForTest{})
	tc.RegisterModel(&ModelWithHooksForTest{})
	tc.RegisterModel(&RelatedModelForTest{})
	tc.RegisterModel(&ExternalPackageModel{})

	// Test case 1: Simple models should work without special handling
	t.Run("SimpleModel_works_without_fix", func(t *testing.T) {
		// Set up expectation builders using our framework's API
		tc.ForTable("simple_test_models").ExpectCreate(1)

		// Use our transaction helper API
		err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&SimpleModelForTest{
				Name: "Simple model",
			}).Error
		})

		// Verify
		assert.NoError(t, err, "Simple model should not need special table resolution")
	})

	// Test case 2: Models with hooks should work with our table resolution fix
	t.Run("ModelWithHooks_works_with_fix", func(t *testing.T) {
		// Set up expectation builders using our framework's API
		tc.ForTable("hook_test_models").ExpectCreate(1)

		// Use our transaction helper API
		err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&ModelWithHooksForTest{
				Name: "Model with hooks",
			}).Error
		})

		// Verify
		assert.NoError(t, err, "Model with hooks should work with our table resolution fix")
	})

	// Test case 3: Models with relationships should work with our fix
	t.Run("RelatedModel_works_with_fix", func(t *testing.T) {
		// Set up prerequisites
		externalModel := &ExternalPackageModel{
			Name: "External model",
		}

		// Set up expectation for creating prerequisite
		tc.ForTable("external_package_models").ExpectCreate(1)

		// Create external model
		require.NoError(t, tc.DB().Create(externalModel).Error)

		// Set up expectation for the related model
		tc.ForTable("related_test_models").ExpectCreate(1)

		// Execute transaction with related model - this would fail without our fix
		err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			return tx.Create(&RelatedModelForTest{
				ExternalID: externalModel.ID,
				Name:       "test",
			}).Error
		})

		// Verify
		assert.NoError(t, err, "Related model should work with our table resolution fix")
	})

	// Test case 4: RegisterModelWithRelationships provides special handling
	t.Run("RegisterModelWithRelationships_special_handling", func(t *testing.T) {
		// Create a new test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Register using the enhanced registration that detects relationships
		RegisterModelWithRelationships[RelatedModelForTest](tc)

		// Set up prerequisites
		externalModel := &ExternalPackageModel{
			Name: "Test model for relationship registration",
		}

		// Set up expectation for creating prerequisite
		tc.ForTable("external_package_models").ExpectCreate(1)

		// Create external model
		require.NoError(t, tc.DB().Create(externalModel).Error)

		// Set up expectation for the related model
		tc.ForTable("related_test_models").ExpectCreate(1)

		// Execute transaction with related model using RegisterModelWithRelationships
		err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
			// Create model without explicit table setting
			return tx.Create(&RelatedModelForTest{
				ExternalID: externalModel.ID,
				Name:       "Auto-detected relationships",
			}).Error
		})

		// Verify
		assert.NoError(t, err, "RegisterModelWithRelationships should handle table resolution")
	})
}

// TestMultipleTransactionOperations tests that our table resolution fix works
// consistently across multiple operations in the same transaction.
func TestMultipleTransactionOperations(t *testing.T) {
	// Create test context with mock DB
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register the models including related models
	RegisterModelWithRelationships[RelatedModelForTest](tc)

	// Set up expectation for prerequisite creation
	tc.ForTable("external_package_models").ExpectCreate(1)

	// Create external model
	externalModel := &ExternalPackageModel{
		Name: "External for multiple operations",
	}
	require.NoError(t, tc.DB().Create(externalModel).Error)

	// Set up expectations for multiple operations
	// Each table will be created once in the transaction
	tc.ForTable("related_test_models").ExpectCreate(1)
	tc.ForTable("simple_test_models").ExpectCreate(1)
	tc.ForTable("hook_test_models").ExpectCreate(1)

	// Execute transaction with multiple operations
	err := tc.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
		// First create related model
		relatedModel := &RelatedModelForTest{
			ExternalID: externalModel.ID,
			Name:       "test",
		}
		if err := tx.Create(relatedModel).Error; err != nil {
			return err
		}

		// Then create simple model
		simpleModel := &SimpleModelForTest{
			Name: "Simple part of multi-operation",
		}
		if err := tx.Create(simpleModel).Error; err != nil {
			return err
		}

		// Finally create model with hooks
		hookModel := &ModelWithHooksForTest{
			Name: "Hook part of multi-operation",
		}
		return tx.Create(hookModel).Error
	})

	// Verify
	assert.NoError(t, err, "Multiple operations in transaction should work with our fix")
}

// MockResult type for SQL results in tests
type MockResult struct {
	lastInsertID int64
	rowsAffected int64
}

// LastInsertId implements the driver.Result interface
func (r MockResult) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

// RowsAffected implements the driver.Result interface
func (r MockResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func NewMockResult(lastInsertID, rowsAffected int64) MockResult {
	return MockResult{
		lastInsertID: lastInsertID,
		rowsAffected: rowsAffected,
	}
}
