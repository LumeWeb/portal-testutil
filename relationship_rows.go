package testutil

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
)

// BuildRowsWithRelations is an enhanced version of BuildRowsFrom that properly handles GORM relationships.
//
// This method extends the standard BuildRowsFrom by using JSON serialization for relationship fields,
// allowing GORM tests with relationship models to execute without the "unsupported data type: &map[]" error
// that commonly occurs in these scenarios.
//
// # Problem Solved
//
// When testing GORM models with relationships using sqlmock, you often encounter this error:
//
//	"sql: Scan error on column index X: unsupported data type: &map[]"
//
// This happens because:
// 1. GORM's relationship loading mechanism tries to convert mock data into relationship structs
// 2. SQLMock provides simple data types that don't match GORM's expected structure
// 3. The mismatch causes the error when scanning relationship fields
//
// # Features
//
//   - Serializes relationship structs to JSON column values
//   - Handles belongs-to, has-one and has-many relationships
//   - Maintains proper foreign key references
//   - Works with nested relationship structures
//   - Special handling for gorm.DeletedAt fields
//
// # Example Usage
//
//	// Create test models with relationships
//	parents := []models.Parent{
//	    {
//	        Model: gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
//	        Name:  "Parent 1",
//	        Children: []models.Child{
//	            {
//	                Model:    gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
//	                Name:     "Child 1",
//	                ParentID: 1,
//	            },
//	        },
//	    },
//	}
//
//	// Use enhanced builder with relationship support
//	rows := tc.BuildRowsWithRelations("parents", parents)
//
//	// Use in test expectations
//	tc.Raw().ExpectQuery("SELECT (.+) FROM `parents`").WillReturnRows(rows)
//
//	// Now the query will execute without "unsupported data type: &map[]" errors
//	var results []models.Parent
//	tc.DB().Find(&results)
//
// For advanced scenarios requiring full relationship reconstruction,
// use the provided SQLRelationshipScanner.
func (tc *DBTestContext) BuildRowsWithRelations(table string, models any) *sqlmock.Rows {
	// For nil models, return an empty result
	if models == nil {
		return sqlmock.NewRows([]string{})
	}

	// Use reflection to get the type of the models
	modelsVal := reflect.ValueOf(models)

	// Handle different kinds of input
	switch modelsVal.Kind() {
	case reflect.Slice:
		// For empty slices, return empty rows
		if modelsVal.Len() == 0 {
			return sqlmock.NewRows([]string{})
		}

		// Get the first element to determine the structure
		firstModel := modelsVal.Index(0)

		// If the slice contains maps, handle them with the standard function
		if firstModel.Kind() == reflect.Map {
			return tc.buildRowsFromMaps(table, models)
		}

		// Otherwise, use our enhanced struct processor
		return tc.buildRowsFromStructsWithRelations(table, models)

	case reflect.Struct:
		// Single struct - wrap in a slice and process
		sliceType := reflect.SliceOf(modelsVal.Type())
		slice := reflect.MakeSlice(sliceType, 1, 1)
		slice.Index(0).Set(modelsVal)
		return tc.buildRowsFromStructsWithRelations(table, slice.Interface())

	default:
		// For other types, use the standard implementation
		return tc.BuildRowsFrom(table, models)
	}
}

// buildRowsFromStructsWithRelations creates mock rows with relationship fields and maps serialized to JSON.
// This internal function builds upon the standard implementation but handles relationship fields and maps differently.
func (tc *DBTestContext) buildRowsFromStructsWithRelations(table string, models any) *sqlmock.Rows {
	modelsVal := reflect.ValueOf(models)

	// Must be a slice with at least one element
	if modelsVal.Kind() != reflect.Slice || modelsVal.Len() == 0 {
		return sqlmock.NewRows([]string{})
	}

	// Get the first model to determine structure and field names
	firstModel := modelsVal.Index(0)
	firstModelType := firstModel.Type()

	// Extract column names from the struct fields
	columns := make([]string, 0)
	fieldInfos := make(map[string]*fieldInfo)

	// Check for gorm.Model embedding first
	for i := 0; i < firstModelType.NumField(); i++ {
		field := firstModelType.Field(i)
		if field.Anonymous {
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			// If this is gorm.Model, ensure we include standard fields
			if fieldType.Name() == "Model" && fieldType.PkgPath() == "gorm.io/gorm" {
				// Add critical gorm.Model fields
				ensureColumnInList(&columns, "id")
				ensureColumnInList(&columns, "created_at")
				ensureColumnInList(&columns, "updated_at")
				ensureColumnInList(&columns, "deleted_at")

				// Store field info
				fieldInfos["id"] = &fieldInfo{path: []int{i, 0}, isRelationship: false}
				fieldInfos["created_at"] = &fieldInfo{path: []int{i, 1}, isRelationship: false}
				fieldInfos["updated_at"] = &fieldInfo{path: []int{i, 2}, isRelationship: false}
				fieldInfos["deleted_at"] = &fieldInfo{path: []int{i, 3}, isRelationship: false}
				break
			}
		}
	}

	// Process all fields, including relationships
	tc.extractFieldNamesWithRelations(firstModelType, []int{}, &columns, fieldInfos)

	// Create a row builder with the columns
	builder := NewRowBuilder(columns...)

	// For each model, extract values and add as a row
	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		values := make(map[string]any)

		// For each column, find the corresponding field value
		for _, col := range columns {
			info, ok := fieldInfos[col]
			if !ok {
				values[col] = nil
				continue
			}

			// Follow the path to get the field
			field := model
			for _, idx := range info.path {
				if field.Kind() == reflect.Ptr && !field.IsNil() {
					field = field.Elem()
				}
				field = field.Field(idx)
			}

			// Handle the field based on its type
			if info.isRelationship {
				// For relationship fields, serialize to JSON
				jsonData, err := serializeFieldToJSON(field)
				if err != nil {
					// On error, store null
					values[col] = nil
				} else {
					// Store the JSON string
					values[col] = jsonData
				}
			} else {
				// For other non-relationship fields, use standard handling
				values[col] = getFieldValue(field)
			}
		}

		builder.AddRowWithMap(values)
	}

	return builder.Build()
}

// fieldInfo stores information about a field for row building
type fieldInfo struct {
	path           []int  // Path of indices to reach the field
	isRelationship bool   // Whether this is a relationship field
	foreignKey     string // Foreign key name if applicable
	references     string // Reference field if applicable
}

// extractFieldNamesWithRelations extracts column names with enhanced relationship handling
func (tc *DBTestContext) extractFieldNamesWithRelations(t reflect.Type, path []int, columns *[]string, fieldInfos map[string]*fieldInfo) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Create a new path including the current field
		newPath := append(append([]int{}, path...), i)

		if field.Anonymous {
			// Handle embedded structs
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				tc.extractFieldNamesWithRelations(fieldType, newPath, columns, fieldInfos)
			}
			continue
		}

		// Check for map fields which need special handling
		isMapType := field.Type.Kind() == reflect.Map

		// Determine if this is a relationship field
		isRelationship, relationshipType := isRelationshipField(field)

		// Skip has-many slice/array relationships as direct columns
		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			// But include them as JSON columns with the _json suffix
			if isRelationship || isMapType || containsMapType(field.Type) {
				colName := toSnakeCase(field.Name) + "_json"
				ensureColumnInList(columns, colName)
				fieldInfos[colName] = &fieldInfo{
					path:           newPath,
					isRelationship: true, // Treat as relationship for serialization
				}
			}
			continue
		}

		// Get column name from tags or field name
		colName := getColumnName(field, isRelationship)

		// Add normal column to the list
		if colName != "" {
			ensureColumnInList(columns, colName)

			// For map types, mark them as "relationships" so they'll be serialized to JSON
			if isMapType {
				fieldInfos[colName] = &fieldInfo{
					path:           newPath,
					isRelationship: true, // Treat maps like relationships for serialization
				}
			} else {
				fieldInfos[colName] = &fieldInfo{
					path:           newPath,
					isRelationship: false,
				}
			}
		}

		// For relationship fields, add an additional JSON column
		if isRelationship && relationshipType != "belongs_to" {
			jsonColName := toSnakeCase(field.Name) + "_json"
			ensureColumnInList(columns, jsonColName)
			fieldInfos[jsonColName] = &fieldInfo{
				path:           newPath,
				isRelationship: true,
			}
		}
	}
}

// isRelationshipField determines if a field is a GORM relationship.
// Returns isRelationship bool and the type of relationship (belongs_to, has_one, has_many).
func isRelationshipField(field reflect.StructField) (bool, string) {
	// Check GORM tags for relationship indicators
	gormTag := field.Tag.Get("gorm")

	// Check for known relationship indicators in tags
	if strings.Contains(gormTag, "foreignKey:") ||
		strings.Contains(gormTag, "references:") ||
		strings.Contains(gormTag, "many2many:") ||
		strings.Contains(gormTag, "polymorphic:") {

		// Determine relationship type
		if strings.Contains(gormTag, "many2many:") {
			return true, "many_to_many"
		} else if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			return true, "has_many"
		} else {
			// Check if it's belongs_to or has_one
			// This is approximate - in a full implementation we'd check more thoroughly
			if strings.HasSuffix(field.Name, "ID") ||
				strings.Contains(gormTag, "belongsTo") {
				return true, "belongs_to"
			}
			return true, "has_one"
		}
	}

	// Check field type
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	// If it's a struct that's not a basic type, it's likely a relationship
	if fieldType.Kind() == reflect.Struct {
		// Skip time.Time which is a basic type
		if fieldType.PkgPath() == "time" && fieldType.Name() == "Time" {
			return false, ""
		}

		// Skip gorm.DeletedAt which is a special GORM type
		if fieldType.PkgPath() == "gorm.io/gorm" && fieldType.Name() == "DeletedAt" {
			return false, ""
		}

		// External package struct is likely a relationship
		if fieldType.PkgPath() != "" {
			if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
				return true, "has_many"
			}
			return true, "has_one"
		}
	}

	return false, ""
}

// getColumnName extracts the database column name from struct field tags or name
func getColumnName(field reflect.StructField, isRelationship bool) string {
	gormTag := field.Tag.Get("gorm")

	// Check for column name in GORM tag
	if gormTag != "" && gormTag != "-" {
		// For relationship fields with a foreignKey tag, extract that as main column
		if isRelationship && strings.Contains(gormTag, "foreignKey:") {
			parts := strings.Split(gormTag, ";")
			for _, part := range parts {
				if strings.HasPrefix(part, "foreignKey:") {
					foreignKey := strings.TrimPrefix(part, "foreignKey:")
					return toSnakeCase(foreignKey)
				}
			}
		}

		// Check for explicit column name
		if strings.Contains(gormTag, "column:") {
			parts := strings.Split(gormTag, ";")
			for _, part := range parts {
				if strings.HasPrefix(part, "column:") {
					return strings.TrimPrefix(part, "column:")
				}
			}
		}
	}

	// Try JSON tag
	if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		return strings.Split(jsonTag, ",")[0]
	}

	// Default to snake_case field name
	return toSnakeCase(field.Name)
}

// getFieldValue extracts a database-compatible value from a reflect.Value
func getFieldValue(field reflect.Value) interface{} {
	// Handle nil pointers
	if field.Kind() == reflect.Ptr && field.IsNil() {
		return nil
	}

	// Dereference pointers
	if field.Kind() == reflect.Ptr {
		field = field.Elem()
	}

	// Handle basic types
	switch field.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return field.Interface()
	case reflect.Struct:
		// Special handling for time.Time
		if timeVal, ok := field.Interface().(time.Time); ok {
			return timeVal
		}

		// Special handling for gorm.DeletedAt
		if field.Type().Name() == "DeletedAt" && field.Type().PkgPath() == "gorm.io/gorm" {
			// Get the Time field from DeletedAt
			timeField := field.FieldByName("Time")
			if timeField.IsValid() {
				// Check if it's a zero time (nil in the database)
				isZero := timeField.MethodByName("IsZero").Call(nil)[0].Bool()
				if isZero {
					return nil
				} else {
					return timeField.Interface()
				}
			}
			return nil
		}

		// For other structs, try to get ID field or convert to string
		idField := field.FieldByName("ID")
		if idField.IsValid() && idField.CanInterface() {
			return idField.Interface()
		}

		// Last resort: convert to string
		return field.String()
	case reflect.Map:
		// For map fields, serialize to JSON string
		if field.CanInterface() {
			jsonData, err := json.Marshal(field.Interface())
			if err == nil {
				return string(jsonData)
			}
		}
		// Fallback to empty JSON object if serialization fails
		return "{}"
	case reflect.Slice, reflect.Array:
		// Check if it's a slice of maps or complex types
		if field.Type().Elem().Kind() == reflect.Map || containsMapType(field.Type()) {
			// Serialize to JSON string
			if field.CanInterface() {
				jsonData, err := json.Marshal(field.Interface())
				if err == nil {
					return string(jsonData)
				}
			}
			// Fallback to empty JSON array if serialization fails
			return "[]"
		}

		// Regular slice/array handling
		return field.Interface()
	default:
		// Try to convert to string or return nil
		if field.CanInterface() {
			return field.Interface()
		}
		return nil
	}
}

// Helper functions for map type handling

// containsMapType checks if a type is or contains maps (like slice of maps)
func containsMapType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Map:
		return true
	case reflect.Slice, reflect.Array:
		return t.Elem().Kind() == reflect.Map || (t.Elem().Kind() == reflect.Ptr && t.Elem().Elem().Kind() == reflect.Map)
	case reflect.Ptr:
		return containsMapType(t.Elem())
	case reflect.Struct:
		// Check if any field is a map
		for i := 0; i < t.NumField(); i++ {
			if containsMapType(t.Field(i).Type) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// serializeFieldToJSON serializes a relationship field to JSON
func serializeFieldToJSON(field reflect.Value) (string, error) {
	// Handle nil pointers
	if field.Kind() == reflect.Ptr && field.IsNil() {
		return "null", nil
	}

	// For nil interfaces
	if field.Kind() == reflect.Interface && field.IsNil() {
		return "null", nil
	}

	// Dereference pointers to get the underlying value
	if field.Kind() == reflect.Ptr {
		field = field.Elem()
	}

	// Empty slices
	if (field.Kind() == reflect.Slice || field.Kind() == reflect.Array) && field.Len() == 0 {
		return "[]", nil
	}

	// Marshall the field to JSON
	if field.CanInterface() {
		data, err := json.Marshal(field.Interface())
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "null", nil
}

// SQLRelationshipScanner provides a custom scanning mechanism for GORM relationship fields
// in tests. This is an advanced utility that implements the sql.Scanner interface for
// deserializing JSON-encoded relationship data back into Go structs.
//
// # Usage
//
// For complete relationship support, you would typically implement a custom GORM
// scanner or hook to use this functionality. Basic usage:
//
//	// 1. Create a scanner with your target variable
//	var child ChildModel
//	scanner := &SQLRelationshipScanner{Value: &child}
//
//	// 2. Scan the JSON data (normally called by GORM internally)
//	scanner.Scan(jsonData)
//
//	// 3. Now child contains the reconstructed relationship data
//
// This scanner supports:
// - Scanning from string or []byte JSON data
// - Handling nil values
// - Basic type conversions from numeric and boolean types
// - Fallback to driver.Valuer for complex types
type SQLRelationshipScanner struct {
	Value interface{} // The target variable to populate (must be a pointer)
}

// Scan implements the sql.Scanner interface for deserializing JSON data
// into the target Value struct or slice.
func (s *SQLRelationshipScanner) Scan(src interface{}) error {
	switch src := src.(type) {
	case string:
		// If it's a JSON string, try to unmarshal it into the target
		return json.Unmarshal([]byte(src), s.Value)
	case []byte:
		// If it's a byte slice, try to unmarshal it into the target
		return json.Unmarshal(src, s.Value)
	case nil:
		// Handle nil values
		return nil
	default:
		// For other types, try to convert to string and then unmarshal
		var stringVal string

		switch v := src.(type) {
		case int64:
			stringVal = strconv.FormatInt(v, 10)
		case float64:
			stringVal = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			stringVal = strconv.FormatBool(v)
		case time.Time:
			stringVal = v.Format(time.RFC3339Nano)
		default:
			// Try to use driver.String as a last resort
			if valuer, ok := src.(driver.Valuer); ok {
				val, err := valuer.Value()
				if err != nil {
					return err
				}
				if val == nil {
					return nil
				}
				if str, ok := val.(string); ok {
					stringVal = str
				} else {
					return json.Unmarshal([]byte(`null`), s.Value)
				}
			} else {
				return json.Unmarshal([]byte(`null`), s.Value)
			}
		}

		return json.Unmarshal([]byte(stringVal), s.Value)
	}
}
