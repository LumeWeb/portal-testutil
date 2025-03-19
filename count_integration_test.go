package testutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Example model for testing
type TestItem struct {
	ID     uint   `gorm:"primarykey"`
	Name   string `gorm:"column:name"`
	Status string `gorm:"column:status"`
}

// Example service that uses count operations
type TestItemService struct {
	db *gorm.DB
}

func NewTestItemService(db *gorm.DB) *TestItemService {
	return &TestItemService{db: db}
}

// CountAll returns the total count of items
func (s *TestItemService) CountAll() (int64, error) {
	var count int64
	result := s.db.Model(&TestItem{}).Count(&count)
	return count, result.Error
}

// CountActive returns the count of items with 'active' status
func (s *TestItemService) CountActive() (int64, error) {
	var count int64
	result := s.db.Model(&TestItem{}).Where("status = ?", "active").Count(&count)
	return count, result.Error
}

func TestCountIntegration(t *testing.T) {
	t.Run("Test service with successful count", func(t *testing.T) {
		// Create test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up expectations for the count operation
		tc.ForTable("test_items").ExpectCount(25)

		// Create and use the service
		service := NewTestItemService(tc.DB())
		count, err := service.CountAll()

		// Verify results
		assert.NoError(t, err)
		assert.Equal(t, int64(25), count)
		tc.VerifyExpectations()
	})

	t.Run("Test service with filtered count", func(t *testing.T) {
		// Create test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up expectations for the filtered count operation
		tc.ForTable("test_items").ExpectCount().Where("status = ?", "active").ReturnCount(10)

		// Create and use the service
		service := NewTestItemService(tc.DB())
		count, err := service.CountActive()

		// Verify results
		assert.NoError(t, err)
		assert.Equal(t, int64(10), count)
		tc.VerifyExpectations()
	})

	t.Run("Test service with count error", func(t *testing.T) {
		// Create test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up expectations for the count operation with error
		expectedErr := errors.New("database error during count")
		tc.ForTable("test_items").ExpectCount().ReturnError(expectedErr)

		// Create and use the service
		service := NewTestItemService(tc.DB())
		count, err := service.CountAll()

		// Verify error is returned
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, int64(0), count)
		tc.VerifyExpectations()
	})

	t.Run("Test service with filtered count error", func(t *testing.T) {
		// Create test context
		tc := NewDBTestContext(t)
		defer tc.Teardown()

		// Set up expectations for the filtered count operation with error
		expectedErr := errors.New("filtered count error")
		tc.ForTable("test_items").ExpectCount().Where("status = ?", "active").ReturnError(expectedErr)

		// Create and use the service
		service := NewTestItemService(tc.DB())
		count, err := service.CountActive()

		// Verify error is returned
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, int64(0), count)
		tc.VerifyExpectations()
	})
}
