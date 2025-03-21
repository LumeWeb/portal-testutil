package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// Define test models with relationship fields
type RelTestReporter struct {
	gorm.Model
	Name  string
	Email string
}

type RelTestSubject struct {
	gorm.Model
	Identifier string
	Type       string
}

type RelTestMessage struct {
	gorm.Model
	CaseID  uint
	Content string
	SentAt  time.Time
}

// Case with relationship fields - both belongs-to and has-many
type RelTestCase struct {
	gorm.Model
	Reference   string
	Description string
	Status      string

	// Belongs-to relationships
	ReporterID uint
	Reporter   RelTestReporter `gorm:"foreignKey:ReporterID"`

	SubjectID uint
	Subject   RelTestSubject `gorm:"foreignKey:SubjectID"`

	// Has-many relationship
	Messages []RelTestMessage `gorm:"foreignKey:CaseID"`
}

// TestBuildRowsFrom_HandlesRelationships tests that BuildRowsFrom can now handle
// relationship fields without panicking
func TestBuildRowsFrom_HandlesRelationships(t *testing.T) {
	// Create a minimal test context
	tc := &DBTestContext{}

	now := time.Now()

	// Create a case with relationship fields populated
	testCase := RelTestCase{
		Model:       gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Reference:   "CASE-001",
		Description: "Test case",
		Status:      "OPEN",
		ReporterID:  10,
		Reporter: RelTestReporter{
			Model: gorm.Model{ID: 10},
			Name:  "John Doe",
		},
		SubjectID: 20,
		Subject: RelTestSubject{
			Model:      gorm.Model{ID: 20},
			Identifier: "SUB-001",
		},
		Messages: []RelTestMessage{
			{
				Model:   gorm.Model{ID: 100},
				Content: "Message 1",
			},
		},
	}

	// Before our fix, this would panic with:
	// "panic: row #1, column #X ("reporter") type models.Reporter: unsupported type models.Reporter, a struct"
	// Now it should work fine
	rows := tc.BuildRowsFrom("test_cases", testCase)

	// If we get here without a panic, the test passes
	assert.NotNil(t, rows, "BuildRowsFrom should handle relationship fields without panicking")
}

// Case with pointer relationships
type RelTestCaseWithPointers struct {
	gorm.Model
	Reference   string
	Description string
	Status      string

	ReporterID uint
	Reporter   *RelTestReporter `gorm:"foreignKey:ReporterID"`

	SubjectID uint
	Subject   *RelTestSubject `gorm:"foreignKey:SubjectID"`

	Messages []*RelTestMessage `gorm:"foreignKey:CaseID"`
}

// TestBuildRowsFrom_HandlesPointerRelationships tests handling of pointer relationships
func TestBuildRowsFrom_HandlesPointerRelationships(t *testing.T) {
	// Create a minimal test context
	tc := &DBTestContext{}

	now := time.Now()

	// Create a case with pointer relationship fields
	testCase := RelTestCaseWithPointers{
		Model:       gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Reference:   "CASE-001",
		Description: "Test case with pointers",
		Status:      "OPEN",
		ReporterID:  10,
		Reporter: &RelTestReporter{
			Model: gorm.Model{ID: 10},
			Name:  "John Doe",
		},
		SubjectID: 20,
		Subject: &RelTestSubject{
			Model:      gorm.Model{ID: 20},
			Identifier: "SUB-001",
		},
		Messages: []*RelTestMessage{
			{
				Model:   gorm.Model{ID: 100},
				Content: "Message 1",
			},
		},
	}

	// This would also panic before the fix
	rows := tc.BuildRowsFrom("test_cases_pointers", testCase)

	// If we get here without a panic, the test passes
	assert.NotNil(t, rows, "BuildRowsFrom should handle pointer relationship fields without panicking")
}

// TestBuildRowsFrom_HandlesNilPointers tests handling of nil pointer relationships
func TestBuildRowsFrom_HandlesNilPointers(t *testing.T) {
	// Create a minimal test context
	tc := &DBTestContext{}

	now := time.Now()

	// Create a case with nil pointer relationships
	testCase := RelTestCaseWithPointers{
		Model:       gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Reference:   "CASE-001",
		Description: "Test case with nil pointers",
		Status:      "OPEN",
		ReporterID:  10,
		Reporter:    nil, // nil relationship
		SubjectID:   20,
		Subject:     nil, // nil relationship
		Messages:    nil, // nil slice
	}

	// This should also work now
	rows := tc.BuildRowsFrom("test_cases_nil_pointers", testCase)

	// If we get here without a panic, the test passes
	assert.NotNil(t, rows, "BuildRowsFrom should handle nil relationship pointers without panicking")
}

// TestBuildRowsFrom_MapsStillWork tests that maps with relationship fields still work
func TestBuildRowsFrom_MapsStillWork(t *testing.T) {
	// Create a minimal test context
	tc := &DBTestContext{}

	now := time.Now()

	// The traditional approach using maps
	mapData := map[string]any{
		"id":          1,
		"created_at":  now,
		"updated_at":  now,
		"reference":   "CASE-001",
		"description": "Test case",
		"status":      "OPEN",
		"reporter_id": 10,
		"subject_id":  20,
	}

	// This should still work
	rows := tc.BuildRowsFrom("test_cases", mapData)

	// If we get here without a panic, the test passes
	assert.NotNil(t, rows, "BuildRowsFrom should still work with maps containing relationship IDs")
}
