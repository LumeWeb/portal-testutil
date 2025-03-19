package testutil

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// RowsWrapper wraps sqlmock.Rows to provide additional methods that make it compatible
// with the standard database/sql.Rows interface. This allows test code to use the
// same scanning patterns that would be used with real database rows.
//
// RowsWrapper adds the following functionality:
// - Next() and Scan() methods that work like their sql.Rows counterparts
// - Sophisticated type conversion to match how real database drivers work
// - Support for scanning into various Go types (strings, ints, floats, times, etc.)
// - Proper handling of nil values
//
// This wrapper makes it much easier to test code that uses row scanning without
// having to change the scanning patterns for tests.
type RowsWrapper struct {
	rows        *sqlmock.Rows   // The underlying sqlmock.Rows
	columns     []string        // Column names extracted from the rows
	currentRow  int             // Current row index for iteration
	initialized bool            // Whether the wrapper has been initialized
	data        [][]interface{} // Extracted row data
}

// NewRowsWrapper creates a new wrapper around sqlmock.Rows that provides a sql.Rows-like interface.
//
// It extracts the column metadata and row data from the sqlmock.Rows, allowing the
// wrapper to behave like a real database result set with Next() and Scan() methods.
//
// Example:
//
//	// Create rows with a wrapper
//	rows := sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "John")
//	wrappedRows := NewRowsWrapper(rows)
//
//	// Use like real database rows
//	for wrappedRows.Next() {
//	    var id int
//	    var name string
//	    wrappedRows.Scan(&id, &name)
//	    // Use id and name...
//	}
func NewRowsWrapper(rows *sqlmock.Rows) *RowsWrapper {
	// For sqlmock.Rows, we need to extract columns from the fields
	// Create a temporary DB connection
	mockDB, mock, _ := sqlmock.New()
	defer mockDB.Close()

	// Register the rows with the mock
	mock.ExpectQuery("").WillReturnRows(rows)

	// Execute a query to get the SQL rows
	sqlRows, _ := mockDB.Query("")
	defer sqlRows.Close()

	// Get columns
	columns, _ := sqlRows.Columns()

	return &RowsWrapper{
		rows:       rows,
		columns:    columns,
		currentRow: -1,
		data:       extractRowData(rows),
	}
}

// extractRowData extracts data from sqlmock.Rows
// This is a bit hacky, but sqlmock doesn't expose the row data directly
func extractRowData(rows *sqlmock.Rows) [][]interface{} {
	// Create a clone of the rows to use for data extraction
	mockDB, mock, _ := sqlmock.New()
	defer mockDB.Close()

	// Register the rows with the mock
	mock.ExpectQuery("").WillReturnRows(rows)

	// Execute a query to get the SQL rows
	sqlRows, _ := mockDB.Query("")
	defer sqlRows.Close()

	// Get columns
	columns, _ := sqlRows.Columns()
	columnCount := len(columns)

	// Extract data
	var result [][]interface{}
	for sqlRows.Next() {
		// Create value holders
		values := make([]interface{}, columnCount)
		scanArgs := make([]interface{}, columnCount)
		for i := range values {
			scanArgs[i] = &values[i]
		}

		// Scan the row
		sqlRows.Scan(scanArgs...)

		// Copy scanned values and convert types
		rowValues := make([]interface{}, columnCount)
		for i, v := range values {
			// Handle different types of values
			switch val := v.(type) {
			case []byte:
				// Convert []byte to string for text data
				rowValues[i] = string(val)
			case time.Time:
				// Keep time.Time as is
				rowValues[i] = val
			case nil:
				rowValues[i] = nil
			default:
				rowValues[i] = val
			}
		}

		result = append(result, rowValues)
	}

	return result
}

// Next advances the cursor to the next row
func (w *RowsWrapper) Next() bool {
	w.currentRow++
	return w.currentRow < len(w.data)
}

// convertValue converts a source value to the appropriate type for the destination pointer.
//
// This is the heart of the RowsWrapper's type conversion system. It handles converting
// between various types in a way that's similar to how real database drivers work.
// This allows the mock rows to behave very similarly to real database rows, making
// tests more realistic and reducing the need for special test-only scanning code.
//
// The function supports conversion between many common types:
// - Basic types: string, []byte, int/int64, uint/uint64, float32/float64, bool
// - Time values: time.Time, *time.Time
// - String conversions: parsing numbers and booleans from strings
// - Nil handling: setting appropriate zero values for nil source values
//
// If a specific conversion isn't explicitly handled, it falls back to using
// reflection to try to convert between compatible types.
//
// Example:
//
//	// These would work automatically in Scan():
//	var id int
//	var name string
//	var createdAt time.Time
//	var deletedAt *time.Time
//	wrappedRows.Scan(&id, &name, &createdAt, &deletedAt)
func convertValue(src interface{}, destPtr interface{}) error {
	// Handle nil source value - set destination to zero value
	if src == nil {
		destVal := reflect.ValueOf(destPtr)
		if destVal.Kind() != reflect.Ptr {
			return fmt.Errorf("destination not a pointer")
		}
		destElem := destVal.Elem()
		destElem.Set(reflect.Zero(destElem.Type()))
		return nil
	}

	// Handle common destination types with type switch
	switch dest := destPtr.(type) {
	case *string:
		return convertToString(src, dest)
	case *[]byte:
		return convertToBytes(src, dest)
	case *int:
		return convertToInt(src, dest)
	case *int64:
		return convertToInt64(src, dest)
	case *int32:
		return convertToInt32(src, dest)
	case *uint:
		return convertToUint(src, dest)
	case *uint64:
		return convertToUint64(src, dest)
	case *float64:
		return convertToFloat64(src, dest)
	case *float32:
		return convertToFloat32(src, dest)
	case *bool:
		return convertToBool(src, dest)
	case *time.Time:
		return convertToTime(src, dest)
	case **time.Time:
		return convertToTimePtr(src, dest)
	default:
		// Use reflection for other types
		return convertWithReflection(src, destPtr)
	}
}

// convertToString converts a value to string
func convertToString(src interface{}, dest *string) error {
	switch v := src.(type) {
	case string:
		*dest = v
	case []byte:
		*dest = string(v)
	default:
		*dest = fmt.Sprintf("%v", v)
	}
	return nil
}

// convertToBytes converts a value to []byte
func convertToBytes(src interface{}, dest *[]byte) error {
	switch v := src.(type) {
	case []byte:
		*dest = v
	case string:
		*dest = []byte(v)
	default:
		*dest = []byte(fmt.Sprintf("%v", v))
	}
	return nil
}

// convertToInt converts a value to int
func convertToInt(src interface{}, dest *int) error {
	switch v := src.(type) {
	case int:
		*dest = v
	case int64:
		*dest = int(v)
	case int32:
		*dest = int(v)
	case float64:
		*dest = int(v)
	case float32:
		*dest = int(v)
	case []byte:
		i, err := strconv.Atoi(string(v))
		if err != nil {
			return err
		}
		*dest = i
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		*dest = i
	default:
		return fmt.Errorf("cannot convert %T to int", src)
	}
	return nil
}

// convertToInt64 converts a value to int64
func convertToInt64(src interface{}, dest *int64) error {
	switch v := src.(type) {
	case int64:
		*dest = v
	case int:
		*dest = int64(v)
	case int32:
		*dest = int64(v)
	case float64:
		*dest = int64(v)
	case []byte:
		i, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return err
		}
		*dest = i
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		*dest = i
	default:
		return fmt.Errorf("cannot convert %T to int64", src)
	}
	return nil
}

// convertToInt32 converts a value to int32
func convertToInt32(src interface{}, dest *int32) error {
	switch v := src.(type) {
	case int32:
		*dest = v
	case int:
		*dest = int32(v)
	case int64:
		*dest = int32(v)
	case float32:
		*dest = int32(v)
	case float64:
		*dest = int32(v)
	case []byte:
		i, err := strconv.ParseInt(string(v), 10, 32)
		if err != nil {
			return err
		}
		*dest = int32(i)
	case string:
		i, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return err
		}
		*dest = int32(i)
	default:
		return fmt.Errorf("cannot convert %T to int32", src)
	}
	return nil
}

// convertToUint converts a value to uint
func convertToUint(src interface{}, dest *uint) error {
	switch v := src.(type) {
	case uint:
		*dest = v
	case int:
		if v < 0 {
			return fmt.Errorf("cannot convert negative value %d to uint", v)
		}
		*dest = uint(v)
	case int64:
		if v < 0 {
			return fmt.Errorf("cannot convert negative value %d to uint", v)
		}
		*dest = uint(v)
	case uint64:
		*dest = uint(v)
	case []byte:
		i, err := strconv.ParseUint(string(v), 10, 0)
		if err != nil {
			return err
		}
		*dest = uint(i)
	case string:
		i, err := strconv.ParseUint(v, 10, 0)
		if err != nil {
			return err
		}
		*dest = uint(i)
	default:
		return fmt.Errorf("cannot convert %T to uint", src)
	}
	return nil
}

// convertToUint64 converts a value to uint64
func convertToUint64(src interface{}, dest *uint64) error {
	switch v := src.(type) {
	case uint64:
		*dest = v
	case uint:
		*dest = uint64(v)
	case int:
		if v < 0 {
			return fmt.Errorf("cannot convert negative value %d to uint64", v)
		}
		*dest = uint64(v)
	case int64:
		if v < 0 {
			return fmt.Errorf("cannot convert negative value %d to uint64", v)
		}
		*dest = uint64(v)
	case []byte:
		i, err := strconv.ParseUint(string(v), 10, 64)
		if err != nil {
			return err
		}
		*dest = i
	case string:
		i, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}
		*dest = i
	default:
		return fmt.Errorf("cannot convert %T to uint64", src)
	}
	return nil
}

// convertToFloat64 converts a value to float64
func convertToFloat64(src interface{}, dest *float64) error {
	switch v := src.(type) {
	case float64:
		*dest = v
	case float32:
		*dest = float64(v)
	case int:
		*dest = float64(v)
	case int64:
		*dest = float64(v)
	case []byte:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return err
		}
		*dest = f
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		*dest = f
	default:
		return fmt.Errorf("cannot convert %T to float64", src)
	}
	return nil
}

// convertToFloat32 converts a value to float32
func convertToFloat32(src interface{}, dest *float32) error {
	switch v := src.(type) {
	case float32:
		*dest = v
	case float64:
		*dest = float32(v)
	case int:
		*dest = float32(v)
	case int64:
		*dest = float32(v)
	case []byte:
		f, err := strconv.ParseFloat(string(v), 32)
		if err != nil {
			return err
		}
		*dest = float32(f)
	case string:
		f, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return err
		}
		*dest = float32(f)
	default:
		return fmt.Errorf("cannot convert %T to float32", src)
	}
	return nil
}

// convertToBool converts a value to bool
func convertToBool(src interface{}, dest *bool) error {
	switch v := src.(type) {
	case bool:
		*dest = v
	case int:
		*dest = v != 0
	case int64:
		*dest = v != 0
	case []byte:
		b, err := strconv.ParseBool(string(v))
		if err != nil {
			return err
		}
		*dest = b
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		*dest = b
	default:
		return fmt.Errorf("cannot convert %T to bool", src)
	}
	return nil
}

// convertToTime converts a value to time.Time
func convertToTime(src interface{}, dest *time.Time) error {
	switch v := src.(type) {
	case time.Time:
		*dest = v
	case string:
		// Try common time formats
		formats := []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				*dest = t
				return nil
			}
		}
		return fmt.Errorf("cannot parse time string %q", v)
	case []byte:
		return convertToTime(string(v), dest)
	default:
		return fmt.Errorf("cannot convert %T to time.Time", src)
	}
	return nil
}

// convertToTimePtr converts a value to *time.Time
func convertToTimePtr(src interface{}, dest **time.Time) error {
	switch v := src.(type) {
	case time.Time:
		t := v
		*dest = &t
	case *time.Time:
		*dest = v
	case string:
		if v == "" {
			*dest = nil
			return nil
		}
		var t time.Time
		if err := convertToTime(v, &t); err != nil {
			return err
		}
		*dest = &t
	case []byte:
		if len(v) == 0 {
			*dest = nil
			return nil
		}
		return convertToTimePtr(string(v), dest)
	case nil:
		*dest = nil
	default:
		return fmt.Errorf("cannot convert %T to *time.Time", src)
	}
	return nil
}

// convertWithReflection tries to convert values using reflection
func convertWithReflection(src interface{}, destPtr interface{}) error {
	destVal := reflect.ValueOf(destPtr)
	if destVal.Kind() != reflect.Ptr {
		return fmt.Errorf("destination not a pointer")
	}

	destElem := destVal.Elem()
	srcVal := reflect.ValueOf(src)

	// Direct assignment if types are compatible
	if destElem.Type().AssignableTo(srcVal.Type()) {
		destElem.Set(srcVal)
		return nil
	}

	// Try to convert if possible
	if srcVal.Type().ConvertibleTo(destElem.Type()) {
		destElem.Set(srcVal.Convert(destElem.Type()))
		return nil
	}

	return fmt.Errorf("cannot convert %T to %T", src, destElem.Interface())
}

// Scan copies the columns in the current row into the values pointed at by dest.
//
// This method implements the same behavior as sql.Rows.Scan() but for mock rows.
// It uses the convertValue function to handle type conversions between the source
// data and destination pointers, supporting a wide range of types and conversions.
//
// The arguments must be pointers to variables of the appropriate types for the
// columns being scanned. If there are fewer destination pointers than columns,
// the extra columns are ignored. If a destination is not a pointer, an error
// is returned.
//
// Example:
//
//	// Scan values from the current row
//	var id int
//	var name string
//	var createdAt time.Time
//	var deletedAt *time.Time
//	err := rows.Scan(&id, &name, &createdAt, &deletedAt)
//	if err != nil {
//	    // Handle error
//	}
func (w *RowsWrapper) Scan(dest ...interface{}) error {
	if w.currentRow < 0 || w.currentRow >= len(w.data) {
		return sql.ErrNoRows
	}

	row := w.data[w.currentRow]

	// Copy values to the destination pointers
	for i, src := range row {
		if i >= len(dest) {
			continue // Skip if there's no corresponding destination
		}

		destPtr := dest[i]
		if reflect.ValueOf(destPtr).Kind() != reflect.Ptr {
			return fmt.Errorf("destination argument %d not a pointer", i)
		}

		// Use the value converter for the appropriate type
		if err := convertValue(src, destPtr); err != nil {
			return fmt.Errorf("error converting column %d: %w", i, err)
		}
	}

	return nil
}

// Close closes the rows iterator
func (w *RowsWrapper) Close() error {
	return nil
}

// Columns returns the column names
func (w *RowsWrapper) Columns() ([]string, error) {
	return w.columns, nil
}

// ColumnTypes returns column information
func (w *RowsWrapper) ColumnTypes() ([]*sql.ColumnType, error) {
	return nil, nil // Not implemented for mocks
}

// Err returns the error, if any, that was encountered during iteration
func (w *RowsWrapper) Err() error {
	return nil
}

// WrapRows wraps sqlmock.Rows in a RowsWrapper
func WrapRows(rows *sqlmock.Rows) *RowsWrapper {
	return NewRowsWrapper(rows)
}

// BuildWithWrapper returns rows wrapped with Next and Scan methods
func (rb *RowBuilder) BuildWithWrapper() *RowsWrapper {
	return NewRowsWrapper(rb.Build())
}
