package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ExampleRowBuilder demonstrates how to use the enhanced row builders
func ExampleRowBuilder() {
	// This is just an example and won't actually run

	// Basic row builder with custom columns
	basicBuilder := NewRowBuilder("id", "name", "email")
	basicBuilder.AddRow(1, "John Doe", "john@example.com")
	basicBuilder.AddRow(2, "Jane Smith", "jane@example.com")
	// rows := basicBuilder.Build() // In real code, you would build and use these rows

	// Row builder with model columns
	modelBuilder := NewModelRowBuilder("name", "email")
	modelBuilder.AddModelRow(1, "John Doe", "john@example.com")
	// modelRows := modelBuilder.Build() // In real code, you would build and use these rows

	// Using WithTimestamps and WithSoftDelete
	customBuilder := NewRowBuilder("id", "name")
	customBuilder.WithTimestamps().WithSoftDelete()
	customBuilder.AddRow(1, "John Doe", time.Now(), time.Now(), nil)

	// Using AddRowWithMap
	mapBuilder := NewRowBuilder("id", "name", "email", "age")
	mapBuilder.AddRowWithMap(map[string]interface{}{
		"id":    1,
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	})

	// Example of creating a custom row builder for a specific model
	userBuilder := NewModelRowBuilder("username", "email", "role")
	userBuilder.AddModelRow(1, "johndoe", "john@example.com", "admin")
	// userRows := userBuilder.Build() // In real code, you would build and use these rows
}

// TestRowBuilder demonstrates how to use the row builders in a real test
func TestRowBuilder(t *testing.T) {
	// Skip this test as it's just an example
	t.Skip("This is just an example test")

	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create a custom row builder for a user model
	userBuilder := NewModelRowBuilder("username", "email", "role")
	userBuilder.AddModelRow(1, "johndoe", "john@example.com", "admin")
	userRows := userBuilder.Build() // Build rows for test expectations

	// Set up expectations
	tc.ForTable("users").ExpectFind().ByID(1).ReturnRows(userRows)

	// Using AddRowWithMap
	mapBuilder := NewRowBuilder("id", "name", "email")
	mapBuilder.AddRowWithMap(map[string]interface{}{
		"id":    1,
		"name":  "John Doe",
		"email": "john@example.com",
	})

	// Verify the row was added correctly
	rows := mapBuilder.Build()
	assert.NotNil(t, rows)
}
