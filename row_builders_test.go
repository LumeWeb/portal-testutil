package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRowBuilder(t *testing.T) {
	// Create builder with columns
	builder := NewRowBuilder("id", "name", "email")

	// Verify it has the correct columns
	assert.Equal(t, []string{"id", "name", "email"}, builder.columns)
	assert.NotNil(t, builder.rows)
}

func TestNewModelRowBuilder(t *testing.T) {
	// Create model row builder with additional columns
	builder := NewModelRowBuilder("name", "email")

	// Verify it has the model columns plus additional ones
	expectedColumns := []string{"id", "created_at", "updated_at", "deleted_at", "name", "email"}
	assert.Equal(t, expectedColumns, builder.columns)
}

func TestRowBuilder_WithTimestamps(t *testing.T) {
	// Create basic builder and add timestamps
	builder := NewRowBuilder("id", "name")
	builder.WithTimestamps()

	// Verify columns were added
	assert.Equal(t, []string{"id", "name", "created_at", "updated_at"}, builder.columns)
}

func TestRowBuilder_WithSoftDelete(t *testing.T) {
	// Create basic builder and add soft delete
	builder := NewRowBuilder("id", "name")
	builder.WithSoftDelete()

	// Verify column was added
	assert.Equal(t, []string{"id", "name", "deleted_at"}, builder.columns)
}

func TestRowBuilder_WithCustomColumns(t *testing.T) {
	// Create basic builder and add custom columns
	builder := NewRowBuilder("id")
	builder.WithCustomColumns("name", "email", "phone")

	// Verify columns were added
	assert.Equal(t, []string{"id", "name", "email", "phone"}, builder.columns)
}

func TestRowBuilder_AddRow(t *testing.T) {
	// Create builder and add a row
	builder := NewRowBuilder("id", "name", "age")
	builder.AddRow(1, "Alice", 30)

	// Verify row was added by scanning it
	rows := builder.Build()

	// Verify the sqlmock.Rows was created
	assert.NotNil(t, rows)

	// For testing row content using Next/Scan, use the wrapper
	wrappedRows := NewRowsWrapper(rows)

	// Next and scan
	assert.True(t, wrappedRows.Next())
	var id int
	var name string
	var age int
	err := wrappedRows.Scan(&id, &name, &age)

	// Verify values
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.Equal(t, "Alice", name)
	assert.Equal(t, 30, age)

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestRowBuilder_AddRowWithMap(t *testing.T) {
	// Create builder
	builder := NewRowBuilder("id", "name", "age")

	// Add row with map
	values := map[string]interface{}{
		"id":   1,
		"name": "Bob",
		"age":  25,
	}
	builder.AddRowWithMap(values)

	// Verify row was added
	rows := builder.Build()

	// Verify the sqlmock.Rows was created
	assert.NotNil(t, rows)

	// For testing row content using Next/Scan, use the wrapper
	wrappedRows := NewRowsWrapper(rows)

	// Next and scan
	assert.True(t, wrappedRows.Next())
	var id int
	var name string
	var age int
	err := wrappedRows.Scan(&id, &name, &age)

	// Verify values
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.Equal(t, "Bob", name)
	assert.Equal(t, 25, age)

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestRowBuilder_AddMultipleRows(t *testing.T) {
	// Create builder
	builder := NewRowBuilder("id", "name")

	// Add multiple rows
	rows := [][]interface{}{
		{1, "Alice"},
		{2, "Bob"},
		{3, "Charlie"},
	}
	builder.AddMultipleRows(rows)

	// Verify rows were added
	mockRows := builder.Build()

	// Verify the sqlmock.Rows was created
	assert.NotNil(t, mockRows)

	// Wrap rows for testing with Next/Scan
	wrappedRows := NewRowsWrapper(mockRows)

	// Verify number of rows by iterating through them
	count := 0
	for wrappedRows.Next() {
		count++
	}
	assert.Equal(t, 3, count)
}

func TestRowBuilder_AddMultipleModelRows(t *testing.T) {
	// Create model builder
	builder := NewModelRowBuilder("name")

	// Add multiple model rows
	ids := []uint{1, 2, 3}
	additionalValues := [][]interface{}{
		{"Alice"},
		{"Bob"},
		{"Charlie"},
	}
	builder.AddMultipleModelRows(ids, additionalValues)

	// Verify rows were added
	mockRows := builder.Build()

	// Verify the sqlmock.Rows was created
	assert.NotNil(t, mockRows)

	// Wrap rows for testing with Next/Scan
	wrappedRows := NewRowsWrapper(mockRows)

	// Verify number of rows by iterating through them
	count := 0
	for wrappedRows.Next() {
		count++
	}
	assert.Equal(t, 3, count)
}

func TestRowBuilder_AddModelRow(t *testing.T) {
	// Create model builder
	builder := NewModelRowBuilder("name", "email")

	// Add model row
	builder.AddModelRow(1, "Alice", "alice@example.com")

	// Verify row was added
	rows := builder.Build()

	// Verify the sqlmock.Rows was created
	assert.NotNil(t, rows)

	// Wrap rows for testing with Next/Scan
	wrappedRows := NewRowsWrapper(rows)

	// Next and scan (we can only verify the ID and additional values easily)
	assert.True(t, wrappedRows.Next())
	var id int
	var createdAt, updatedAt, deletedAt time.Time
	var name, email string
	err := wrappedRows.Scan(&id, &createdAt, &updatedAt, &deletedAt, &name, &email)

	// Verify values
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.Equal(t, "Alice", name)
	assert.Equal(t, "alice@example.com", email)

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestRowBuilder_AddModelRowWithTime(t *testing.T) {
	// Create model builder
	builder := NewModelRowBuilder("name")

	// Define times
	created := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	deleted := time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC)

	// Add model row with specific times
	builder.AddModelRowWithTime(1, created, updated, &deleted, "Alice")

	// Verify row was added
	rows := builder.Build()

	// Verify the sqlmock.Rows was created
	assert.NotNil(t, rows)

	// Wrap rows for testing with Next/Scan
	wrappedRows := NewRowsWrapper(rows)

	// Next and scan
	assert.True(t, wrappedRows.Next())
	var id int
	var createdAt, updatedAt time.Time
	var deletedAtPtr *time.Time
	var name string
	err := wrappedRows.Scan(&id, &createdAt, &updatedAt, &deletedAtPtr, &name)

	// Verify values
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.Equal(t, created.Year(), createdAt.Year())
	assert.Equal(t, created.Month(), createdAt.Month())
	assert.Equal(t, created.Day(), createdAt.Day())
	assert.Equal(t, updated.Year(), updatedAt.Year())
	assert.Equal(t, updated.Month(), updatedAt.Month())
	assert.Equal(t, updated.Day(), updatedAt.Day())
	assert.NotNil(t, deletedAtPtr)
	if deletedAtPtr != nil {
		assert.Equal(t, deleted.Year(), deletedAtPtr.Year())
		assert.Equal(t, deleted.Month(), deletedAtPtr.Month())
		assert.Equal(t, deleted.Day(), deletedAtPtr.Day())
	}
	assert.Equal(t, "Alice", name)
}

func TestRowBuilder_BuildSingle(t *testing.T) {
	// Create empty builder
	builder := NewRowBuilder("id", "name")

	// Build a single row
	rows := builder.BuildSingle()

	// Verify a row was built
	assert.NotNil(t, rows)

	// Wrap rows for testing with Next/Scan
	wrappedRows := NewRowsWrapper(rows)

	// Verify we can scan the row
	assert.True(t, wrappedRows.Next())

	var id int
	var name string
	err := wrappedRows.Scan(&id, &name)
	assert.NoError(t, err)

	// The BuildSingle method adds a default row with ID=1 and name=nil
	assert.Equal(t, 1, id)
	assert.Equal(t, "", name) // Name will be empty/zero value because it's nil in the DB row

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestModelRowValues(t *testing.T) {
	// Get model row values
	values := ModelRowValues(1, "Alice", "alice@example.com")

	// Verify the number of values
	assert.Equal(t, 6, len(values))

	// Verify the ID and additional values
	assert.Equal(t, int64(1), values[0])
	assert.Equal(t, "Alice", values[4])
	assert.Equal(t, "alice@example.com", values[5])

	// The timestamps are dynamic, so we just verify they're there
	_, ok := values[1].(time.Time)
	assert.True(t, ok)
	_, ok = values[2].(time.Time)
	assert.True(t, ok)
	assert.Nil(t, values[3])
}

func TestCreateTestModel(t *testing.T) {
	// Create a test model
	model := CreateTestModel(1)

	// Verify the model fields
	assert.Equal(t, uint(1), model.ID)
	assert.False(t, model.CreatedAt.IsZero())
	assert.False(t, model.UpdatedAt.IsZero())
	assert.True(t, model.DeletedAt.Time.IsZero())
}
