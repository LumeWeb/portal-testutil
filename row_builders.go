package testutil

import (
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

// RowBuilder helps build mock database rows with common patterns.
//
// It provides a fluent interface for creating sqlmock.Rows objects with
// various patterns of data, making it easier to set up test data for
// database operations. It supports common patterns like GORM models,
// custom columns, and conversion between Go types and database types.
//
// The builder can be used to create rows for various testing scenarios:
// - Single row for GetByID operations
// - Multiple rows for listing operations
// - Rows with timestamps for time-based operations
// - Rows with soft-delete patterns
//
// Example:
//
//	// Create rows with specified columns
//	rows := NewRowBuilder("id", "name", "email")
//	    .AddRow(1, "John", "john@example.com")
//	    .AddRow(2, "Jane", "jane@example.com")
//	    .Build()
//
//	// Use with ForTable
//	testCtx.ForTable("users").ExpectFind().ReturnRows(rows)
type RowBuilder struct {
	columns []string      // The column names for the rows
	rows    *sqlmock.Rows // The underlying sqlmock.Rows being built
}

// NewRowBuilder creates a new row builder with the specified column names.
//
// This is the most basic way to create a row builder. You specify the exact
// column names you want, and then add rows with values for those columns.
//
// Example:
//
//	// Create a row builder with specific columns
//	builder := NewRowBuilder("id", "name", "email")
//
//	// Add rows and build
//	rows := builder.AddRow(1, "John", "john@example.com").Build()
func NewRowBuilder(columns ...string) *RowBuilder {
	return &RowBuilder{
		columns: columns,
		rows:    sqlmock.NewRows(columns),
	}
}

// NewModelRowBuilder creates a row builder with standard gorm.Model columns plus any additional columns.
//
// The gorm.Model columns are:
// - id: The primary key
// - created_at: When the record was created
// - updated_at: When the record was last updated
// - deleted_at: When the record was soft-deleted (nil if not deleted)
//
// This is useful for testing code that works with GORM models, as it creates
// rows with the standard columns that GORM expects.
//
// Example:
//
//	// Create a model row builder with additional columns
//	builder := NewModelRowBuilder("name", "email")
//
//	// Add model rows and build
//	rows := builder.AddModelRow(1, "John", "john@example.com").Build()
func NewModelRowBuilder(additionalColumns ...string) *RowBuilder {
	// Start with basic model columns
	columns := []string{"id", "created_at", "updated_at", "deleted_at"}

	// Add any additional columns
	columns = append(columns, additionalColumns...)

	return NewRowBuilder(columns...)
}

// WithTimestamps adds standard timestamp columns to the row builder
func (rb *RowBuilder) WithTimestamps() *RowBuilder {
	rb.columns = append(rb.columns, "created_at", "updated_at")
	return rb
}

// WithSoftDelete adds a deleted_at column to the row builder
func (rb *RowBuilder) WithSoftDelete() *RowBuilder {
	rb.columns = append(rb.columns, "deleted_at")
	return rb
}

// WithCustomColumns adds custom columns to the row builder
func (rb *RowBuilder) WithCustomColumns(columns ...string) *RowBuilder {
	rb.columns = append(rb.columns, columns...)
	return rb
}

// AddRow adds a row with the specified values
func (rb *RowBuilder) AddRow(values ...interface{}) *RowBuilder {
	// Convert each value to driver.Value if needed
	driverValues := make([]driver.Value, len(values))
	for i, v := range values {
		switch v := v.(type) {
		case nil:
			driverValues[i] = nil
		case int:
			driverValues[i] = int64(v)
		case int8:
			driverValues[i] = int64(v)
		case int16:
			driverValues[i] = int64(v)
		case int32:
			driverValues[i] = int64(v)
		case int64, float64, float32, bool, string, []byte, time.Time:
			// These types are already driver.Value compatible
			driverValues[i] = v
		case map[string]interface{}:
			// Convert maps to JSON strings
			jsonData, err := json.Marshal(v)
			if err == nil {
				driverValues[i] = string(jsonData)
			} else {
				driverValues[i] = "{}"
			}
		case []map[string]interface{}:
			// Convert slices of maps to JSON strings
			jsonData, err := json.Marshal(v)
			if err == nil {
				driverValues[i] = string(jsonData)
			} else {
				driverValues[i] = "[]"
			}
		case map[string]map[string]interface{}:
			// Convert nested maps to JSON strings
			jsonData, err := json.Marshal(v)
			if err == nil {
				driverValues[i] = string(jsonData)
			} else {
				driverValues[i] = "{}"
			}
		default:
			// For map-like or slice-of-map-like structures, serialize to JSON
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Map ||
				(rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Map) {
				jsonData, err := json.Marshal(v)
				if err == nil {
					driverValues[i] = string(jsonData)
				} else {
					if rv.Kind() == reflect.Slice {
						driverValues[i] = "[]"
					} else {
						driverValues[i] = "{}"
					}
				}
			} else {
				// For other types, let sqlmock handle the conversion
				// If this fails, the test will fail with an appropriate error
				driverValues[i] = v
			}
		}
	}

	rb.rows.AddRow(driverValues...)
	return rb
}

// AddRowWithMap adds a row using a map of column names to values
func (rb *RowBuilder) AddRowWithMap(values map[string]interface{}) *RowBuilder {
	rowValues := make([]interface{}, len(rb.columns))

	for i, col := range rb.columns {
		if val, ok := values[col]; ok {
			rowValues[i] = val
		} else {
			rowValues[i] = nil
		}
	}

	return rb.AddRow(rowValues...)
}

// AddMultipleRows adds multiple rows at once
func (rb *RowBuilder) AddMultipleRows(rows [][]interface{}) *RowBuilder {
	for _, row := range rows {
		rb.AddRow(row...)
	}
	return rb
}

// AddMultipleModelRows adds multiple model rows at once
func (rb *RowBuilder) AddMultipleModelRows(ids []uint, additionalValuesList [][]interface{}) *RowBuilder {
	if len(ids) != len(additionalValuesList) {
		panic("ids and additionalValuesList must have the same length")
	}

	for i, id := range ids {
		rb.AddModelRow(id, additionalValuesList[i]...)
	}
	return rb
}

// AddMultipleMapsAsRows adds multiple maps as rows
func (rb *RowBuilder) AddMultipleMapsAsRows(maps []map[string]interface{}) *RowBuilder {
	for _, m := range maps {
		rb.AddRowWithMap(m)
	}
	return rb
}

// AddModelRow adds a row with standard gorm.Model values plus additional values
func (rb *RowBuilder) AddModelRow(id uint, additionalValues ...interface{}) *RowBuilder {
	now := time.Now()

	// Start with the model values - convert uint to int64 for SQL compatibility
	values := []interface{}{int64(id), now, now, nil}

	// Add additional values
	values = append(values, additionalValues...)

	return rb.AddRow(values...)
}

// AddModelRowWithTime adds a row with standard gorm.Model values with specific timestamps
func (rb *RowBuilder) AddModelRowWithTime(id uint, createdAt, updatedAt time.Time, deletedAt *time.Time, additionalValues ...interface{}) *RowBuilder {
	// Start with the model values
	values := []interface{}{int64(id), createdAt, updatedAt, deletedAt}

	// Add additional values
	values = append(values, additionalValues...)

	return rb.AddRow(values...)
}

// Build returns the constructed sqlmock.Rows ready to be used in expectations.
//
// This method finalizes the row building process and returns the underlying
// sqlmock.Rows object, which can be used with ExpectationsBuilder methods
// that accept rows.
//
// Example:
//
//	// Build rows and use them in expectations
//	rows := builder.AddRow(1, "John").AddRow(2, "Jane").Build()
//	testCtx.ForTable("users").ExpectFind().ReturnRows(rows)
func (rb *RowBuilder) Build() *sqlmock.Rows {
	return rb.rows
}

// BuildSingle builds a single row with minimal default values and returns the rows.
//
// This is useful when you just need a row to exist but don't care about specific values,
// such as when testing a "record exists" condition. It creates a row with an ID of 1
// and all other fields set to nil.
//
// Example:
//
//	// Create a single row with default values
//	rows := builder.BuildSingle()
//	testCtx.ForTable("users").ExpectFind().ByID(1).ReturnRows(rows)
func (rb *RowBuilder) BuildSingle() *sqlmock.Rows {
	// Generate a single row with minimal default values - most fields will be nil
	values := make([]interface{}, len(rb.columns))

	// Only set ID to 1 if ID column exists, other fields stay nil
	for i, col := range rb.columns {
		if col == "id" {
			values[i] = int64(1)
		} else {
			values[i] = nil
		}
	}

	// Add the row with minimal values
	rb.AddRow(values...)
	return rb.rows
}

// ModelRowValues returns the values for a model row with the specified ID
func ModelRowValues(id uint, additionalValues ...interface{}) []interface{} {
	now := time.Now()

	// Start with the model values - convert uint to int64 for SQL compatibility
	values := []interface{}{int64(id), now, now, nil}

	// Add additional values
	values = append(values, additionalValues...)

	return values
}

// CreateTestModel creates a simple model with ID field
func CreateTestModel(id uint) *gorm.Model {
	return &gorm.Model{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
