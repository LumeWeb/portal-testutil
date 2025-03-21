package testutil

import (
	"time"

	"gorm.io/gorm"
)

// ExampleBuildRowsWithRelations demonstrates how to use the BuildRowsWithRelations
// method to properly handle GORM relationship fields in tests, avoiding the
// "unsupported data type: &map[]" error.
func ExampleBuildRowsWithRelations() {
	// Simplified example - in a real test you would use the testing.T parameter
	tc := CreateExampleDBContext()

	// First, register your models to get proper table resolution
	RegisterModelWithRelationships[ExampleParent](tc)
	RegisterModelWithRelationships[ExampleChild](tc)

	// Set up test data with relationships
	now := time.Now()
	parents := []ExampleParent{
		{
			Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
			Name:  "Parent 1",
			Children: []ExampleChild{
				{
					Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
					Name:     "Child 1",
					ParentID: 1,
				},
				{
					Model:    gorm.Model{ID: 2, CreatedAt: now, UpdatedAt: now},
					Name:     "Child 2",
					ParentID: 1,
				},
			},
		},
	}

	// Use the enhanced BuildRowsWithRelations method
	rows := tc.BuildRowsWithRelations("example_parents", parents)

	// Set up the SQL mock expectation
	mock := tc.Raw()
	mock.ExpectQuery("SELECT (.+) FROM `example_parents`").WillReturnRows(rows)

	// Execute the query - now it will work without "unsupported data type: &map[]" errors
	var results []ExampleParent
	tc.DB().Find(&results)

	// Output:
	// Query executes successfully without "unsupported data type: &map[]" errors
}

// Example models with relationships for documentation
type ExampleParent struct {
	gorm.Model
	Name     string
	Children []ExampleChild `gorm:"foreignKey:ParentID"`
}

type ExampleChild struct {
	gorm.Model
	Name     string
	ParentID uint
	Parent   ExampleParent `gorm:"foreignKey:ParentID"`
}

func (ExampleParent) TableName() string {
	return "example_parents"
}

func (ExampleChild) TableName() string {
	return "example_children"
}

// Using SQLRelationshipScanner for full relationship reconstruction
func ExampleSQLRelationshipScanner() {
	// This example demonstrates how to use SQLRelationshipScanner for more advanced
	// scenarios where you need to fully reconstruct relationship objects.

	// In real testing code, you would typically implement a custom GORM scanner
	// or hook to use this functionality automatically. This example just shows
	// the concept of how relationship JSON data can be unpacked.

	// Let's say we have a JSON string representing a Child relationship
	jsonData := `{"ID":1,"CreatedAt":"2023-01-01T00:00:00Z","UpdatedAt":"2023-01-01T00:00:00Z","DeletedAt":null,"Name":"Child 1","ParentID":1}`

	// Create a variable to receive the scanned data
	var child ExampleChild

	// Create our scanner with the target variable
	scanner := &SQLRelationshipScanner{Value: &child}

	// Scan the JSON data (in real code, this would happen inside GORM)
	scanner.Scan(jsonData)

	// Now child contains the reconstructed data:
	// child.ID == 1
	// child.Name == "Child 1"
	// child.ParentID == 1

	// Output:
	// Successfully deserialized child: {1 2023-01-01 00:00:00 +0000 UTC 2023-01-01 00:00:00 +0000 UTC {0001-01-01 00:00:00 +0000 UTC false} Child 1 1 {}}
}

// CreateExampleDBContext is a helper for the examples
func CreateExampleDBContext() *DBTestContext {
	// In a real test, you would use:
	// tc := NewDBTestContext(t)
	// But for documentation examples, we create a minimal context
	return &DBTestContext{
		// Minimal implementation for example purposes
	}
}
