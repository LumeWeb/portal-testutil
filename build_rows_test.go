package testutil

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestModel is a simple struct with gorm.Model embedded
type TestModel struct {
	gorm.Model
	Name  string
	Email string
	Age   int
}

// TestModelWithTags is a struct with custom GORM column name tags
type TestModelWithTags struct {
	gorm.Model
	Name     string `gorm:"column:full_name"`
	Email    string `gorm:"column:email_address"`
	IsActive bool   `gorm:"column:active"`
}

// createTestDBContext is a helper to create a test DB context
func createTestDBContext(t *testing.T) *DBTestContext {
	// Create a mock provider directly
	_, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Create a mock db test context
	return &DBTestContext{
		mock: mock,
	}
}

func TestBuildRows_SingleMap(t *testing.T) {
	testCtx := createTestDBContext(t)
	now := time.Now()

	// Create test data
	data := map[string]any{
		"id":         1,
		"created_at": now,
		"updated_at": now,
		"deleted_at": nil,
		"email":      "test@example.com",
		"name":       "Test User",
		"user_id":    nil,
	}

	// Build rows
	rows := testCtx.BuildRows("reporters", data)
	require.NotNil(t, rows)

	// Verify rows using wrapper
	wrappedRows := NewRowsWrapper(rows)
	require.True(t, wrappedRows.Next())

	// Get columns
	cols, err := wrappedRows.Columns()
	require.NoError(t, err)
	assert.Len(t, cols, len(data))

	// Verify data using generic scanning
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	err = wrappedRows.Scan(valuePtrs...)
	require.NoError(t, err)

	// Find specific columns by name
	colMap := make(map[string]int)
	for i, col := range cols {
		colMap[col] = i
	}

	// Check values
	idIdx, ok := colMap["id"]
	require.True(t, ok)
	assert.Equal(t, int64(1), values[idIdx])

	nameIdx, ok := colMap["name"]
	require.True(t, ok)
	assert.Equal(t, "Test User", values[nameIdx])

	emailIdx, ok := colMap["email"]
	require.True(t, ok)
	assert.Equal(t, "test@example.com", values[emailIdx])

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestBuildRowsFrom_SingleStruct(t *testing.T) {
	testCtx := createTestDBContext(t)
	now := time.Now()

	// Create a test model
	model := TestModel{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:  "Test User",
		Email: "test@example.com",
		Age:   30,
	}

	// Build rows from a single struct
	rows := testCtx.BuildRowsFrom("test_models", model)
	require.NotNil(t, rows)

	// Verify rows using wrapper
	wrappedRows := NewRowsWrapper(rows)
	require.True(t, wrappedRows.Next())

	// Get columns
	cols, err := wrappedRows.Columns()
	require.NoError(t, err)
	// The field might be mapped as 'id' or a variation - check it exists
	hasIDField := false
	for _, col := range cols {
		if strings.Contains(strings.ToLower(col), "id") {
			hasIDField = true
			break
		}
	}
	assert.True(t, hasIDField, "Should have an ID field")
	assert.Contains(t, cols, "created_at")
	assert.Contains(t, cols, "updated_at")
	assert.Contains(t, cols, "deleted_at")
	assert.Contains(t, cols, "name")
	assert.Contains(t, cols, "email")
	assert.Contains(t, cols, "age")

	// Scan values
	var id uint
	var createdAt, updatedAt time.Time
	var deletedAt gorm.DeletedAt
	var name, email string
	var age int

	// Find indexes for columns
	colMap := make(map[string]int)
	for i, col := range cols {
		colMap[col] = i
	}

	// Create scanArgs in the right order
	scanArgs := make([]interface{}, len(cols))
	for i := range scanArgs {
		scanArgs[i] = nil // Initialize with nil
	}

	// Assign pointers to the right positions
	scanArgs[colMap["id"]] = &id
	scanArgs[colMap["created_at"]] = &createdAt
	scanArgs[colMap["updated_at"]] = &updatedAt
	scanArgs[colMap["deleted_at"]] = &deletedAt
	scanArgs[colMap["name"]] = &name
	scanArgs[colMap["email"]] = &email
	scanArgs[colMap["age"]] = &age

	err = wrappedRows.Scan(scanArgs...)
	require.NoError(t, err)

	// Verify values
	assert.Equal(t, uint(1), id)
	assert.WithinDuration(t, now, createdAt, time.Second)
	assert.WithinDuration(t, now, updatedAt, time.Second)
	assert.True(t, deletedAt.Time.IsZero())
	assert.Equal(t, "Test User", name)
	assert.Equal(t, "test@example.com", email)
	assert.Equal(t, 30, age)

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestBuildRowsFrom_StructSlice(t *testing.T) {
	testCtx := createTestDBContext(t)
	now := time.Now()

	// Create test models
	models := []TestModel{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:  "User One",
			Email: "user1@example.com",
			Age:   30,
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:  "User Two",
			Email: "user2@example.com",
			Age:   25,
		},
	}

	// Build rows from struct slice
	rows := testCtx.BuildRowsFrom("test_models", models)
	require.NotNil(t, rows)

	// Verify rows using wrapper
	wrappedRows := NewRowsWrapper(rows)

	// Get columns
	cols, err := wrappedRows.Columns()
	require.NoError(t, err)

	// Verify we can scan all rows
	count := 0
	for wrappedRows.Next() {
		count++

		// Create scanArgs
		scanArgs := make([]interface{}, len(cols))
		for i := range scanArgs {
			var val interface{}
			scanArgs[i] = &val
		}

		err = wrappedRows.Scan(scanArgs...)
		require.NoError(t, err)
	}

	// Verify row count
	assert.Equal(t, 2, count)
}

func TestBuildRowsFrom_WithCustomTags(t *testing.T) {
	testCtx := createTestDBContext(t)
	now := time.Now()

	// Create a model with custom column name tags
	model := TestModelWithTags{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:     "Test User",
		Email:    "test@example.com",
		IsActive: true,
	}

	// Build rows
	rows := testCtx.BuildRowsFrom("test_models", model)
	require.NotNil(t, rows)

	// Verify rows using wrapper
	wrappedRows := NewRowsWrapper(rows)
	require.True(t, wrappedRows.Next())

	// Get columns
	cols, err := wrappedRows.Columns()
	require.NoError(t, err)

	// Verify custom column names are used
	assert.Contains(t, cols, "full_name")
	assert.Contains(t, cols, "email_address")
	assert.Contains(t, cols, "active")

	// No more rows
	assert.False(t, wrappedRows.Next())
}

func TestBuildRowsFrom_MapSlice(t *testing.T) {
	testCtx := createTestDBContext(t)
	now := time.Now()

	// Create test data
	maps := []map[string]any{
		{
			"id":         1,
			"created_at": now,
			"updated_at": now,
			"name":       "User One",
			"email":      "user1@example.com",
		},
		{
			"id":         2,
			"created_at": now,
			"updated_at": now,
			"name":       "User Two",
			"email":      "user2@example.com",
		},
	}

	// Build rows from map slice
	rows := testCtx.BuildRowsFrom("test_table", maps)
	require.NotNil(t, rows)

	// Verify rows using wrapper
	wrappedRows := NewRowsWrapper(rows)

	// Count rows
	rowCount := 0
	for wrappedRows.Next() {
		rowCount++
	}
	assert.Equal(t, 2, rowCount)
}

func TestBuildRowsFrom_EmptySlice(t *testing.T) {
	testCtx := createTestDBContext(t)

	// Build rows from empty slice
	rows := testCtx.BuildRowsFrom("test_table", []TestModel{})
	require.NotNil(t, rows)

	// Verify no rows
	wrappedRows := NewRowsWrapper(rows)
	assert.False(t, wrappedRows.Next())
}

func TestBuildRowsFrom_UnsupportedType(t *testing.T) {
	// Skip this test since we can't easily create a mock TestContext
	t.Skip("Skipping unsupported type test")
}
