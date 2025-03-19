package testutil

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ModelValidator is a function that validates a model
type ModelValidator func(interface{}) error

// ModelTestCase represents a test case for model validation
type ModelTestCase struct {
	Name          string
	Model         interface{}
	Validator     ModelValidator
	ExpectError   bool
	ErrorContains string
}

// RunModelValidationTests runs a set of model validation tests
func RunModelValidationTests(t *testing.T, testCases []ModelTestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Execute validation
			err := tc.Validator(tc.Model)

			// Verify expectations
			if tc.ExpectError {
				assert.Error(t, err)
				if tc.ErrorContains != "" {
					assert.Contains(t, err.Error(), tc.ErrorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ModelCRUDTester provides utilities for testing model CRUD operations
type ModelCRUDTester struct {
	t       *testing.T
	testCtx *DBTestContext
	table   string
}

// NewModelCRUDTester creates a new model CRUD tester
func NewModelCRUDTester(t *testing.T, testCtx *DBTestContext, table string) *ModelCRUDTester {
	return &ModelCRUDTester{
		t:       t,
		testCtx: testCtx,
		table:   table,
	}
}

// TestCreate tests the creation of a model
func (m *ModelCRUDTester) TestCreate(model interface{}, id uint) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectInsert(id)

	// Execute create operation
	err := m.testCtx.DB().Create(model).Error
	require.NoError(m.t, err)

	// Verify ID was set
	idField := getReflectValue(model).FieldByName("ID")
	if idField.IsValid() {
		assert.Equal(m.t, id, idField.Interface())
	}

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestCreateError tests that model creation fails as expected
func (m *ModelCRUDTester) TestCreateError(model interface{}, expectedError error) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectInsertError(expectedError)

	// Execute create operation
	err := m.testCtx.DB().Create(model).Error
	require.Error(m.t, err)
	assert.Contains(m.t, err.Error(), expectedError.Error())

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestFind tests finding a model by ID
func (m *ModelCRUDTester) TestFind(id uint, result interface{}, rowBuilder *RowBuilder) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectFind().ByID(id).ReturnRows(rowBuilder.Build())

	// Execute find operation
	err := m.testCtx.DB().First(result, id).Error
	require.NoError(m.t, err)

	// Verify ID matches
	idField := getReflectValue(result).FieldByName("ID")
	if idField.IsValid() {
		assert.Equal(m.t, id, idField.Interface())
	}

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestFindNotFound tests that finding a non-existent model returns a not found error
func (m *ModelCRUDTester) TestFindNotFound(id uint, result interface{}) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectFind().ByID(id).NotFound()

	// Execute find operation
	err := m.testCtx.DB().First(result, id).Error
	require.Error(m.t, err)
	assert.Equal(m.t, gorm.ErrRecordNotFound, err)

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestUpdate tests updating a model
func (m *ModelCRUDTester) TestUpdate(model interface{}) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectUpdate()

	// Execute update operation
	err := m.testCtx.DB().Save(model).Error
	require.NoError(m.t, err)

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestUpdateError tests that model update fails as expected
func (m *ModelCRUDTester) TestUpdateError(model interface{}, expectedError error) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectUpdateError(expectedError)

	// Execute update operation
	err := m.testCtx.DB().Save(model).Error
	require.Error(m.t, err)
	assert.Contains(m.t, err.Error(), expectedError.Error())

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestDelete tests deleting a model
func (m *ModelCRUDTester) TestDelete(model interface{}) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectDelete()

	// Execute delete operation
	err := m.testCtx.DB().Delete(model).Error
	require.NoError(m.t, err)

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestDeleteError tests that model deletion fails as expected
func (m *ModelCRUDTester) TestDeleteError(model interface{}, expectedError error) {
	// Setup expectations
	m.testCtx.ForTable(m.table).ExpectDeleteError(expectedError)

	// Execute delete operation
	err := m.testCtx.DB().Delete(model).Error
	require.Error(m.t, err)
	assert.Contains(m.t, err.Error(), expectedError.Error())

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// TestList tests listing models with filters
func (m *ModelCRUDTester) TestList(result interface{}, filters []interface{}, rowBuilder *RowBuilder, expectedCount int64) {
	// Setup expectations for count
	m.testCtx.ForTable(m.table).ExpectCount(expectedCount)

	// Setup expectations for find
	query := m.testCtx.ForTable(m.table).ExpectFind()
	if len(filters) > 0 {
		// Add where clause if filters are provided
		query = query.Where("?", filters...)
	}
	query.ReturnRows(rowBuilder.Build())

	// Execute list operation
	tx := m.testCtx.DB()
	if len(filters) > 0 {
		tx = tx.Where("?", filters...)
	}

	var count int64
	err := tx.Model(result).Count(&count).Error
	require.NoError(m.t, err)
	assert.Equal(m.t, expectedCount, count)

	err = tx.Find(result).Error
	require.NoError(m.t, err)

	// Verify expectations
	m.testCtx.VerifyExpectations()
}

// Helper function to get reflect.Value
func getReflectValue(obj interface{}) reflect.Value {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val
}
