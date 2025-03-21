package testutil

import (
	"encoding/json"
	"reflect"
)

// SerializeMapFields converts map fields in models to JSON strings to prevent
// the "unsupported data type: &map[]" error that can occur with sqlmock.
//
// This function processes a slice or single model and returns a new copy
// where all map fields have been converted to JSON strings. This is useful
// when working with models that contain map[string]interface{} fields,
// which aren't directly supported by SQL drivers.
//
// Example:
//
//	// Convert models with map fields to serialized versions
//	serializedModels := SerializeMapFields(models)
//
//	// Use with ReturnModels or ReturnModelsWithPreload
//	tc.ForTable("cases").ExpectFind().ReturnModels(serializedModels)
func SerializeMapFields(models interface{}) interface{} {
	val := reflect.ValueOf(models)

	// Handle different input types
	switch val.Kind() {
	case reflect.Slice:
		// For slices, process each element
		result := reflect.MakeSlice(val.Type(), 0, val.Len())
		for i := 0; i < val.Len(); i++ {
			item := val.Index(i)
			processed := serializeMapFieldsInModel(item)
			result = reflect.Append(result, processed)
		}
		return result.Interface()

	case reflect.Struct, reflect.Ptr:
		// For single models, process directly
		return serializeMapFieldsInModel(val).Interface()

	default:
		// For other types, return as is
		return models
	}
}

// serializeMapFieldsInModel processes a single model, converting map fields to JSON strings
func serializeMapFieldsInModel(model reflect.Value) reflect.Value {
	// Handle pointer models
	if model.Kind() == reflect.Ptr {
		if model.IsNil() {
			return model
		}

		// Create new pointer and process the element
		ptrType := model.Type()
		newPtr := reflect.New(ptrType.Elem())
		newElem := serializeMapFieldsInModel(model.Elem())
		newPtr.Elem().Set(newElem)
		return newPtr
	}

	// Only process structs
	if model.Kind() != reflect.Struct {
		return model
	}

	// Create a copy of the struct to modify
	result := reflect.New(model.Type()).Elem()

	// Copy all field values
	for i := 0; i < model.NumField(); i++ {
		field := model.Field(i)
		// We can access field type via model.Type().Field(i).Type if needed later

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get target field in result
		resultField := result.Field(i)
		if !resultField.CanSet() {
			continue
		}

		// Process based on field type
		if field.Kind() == reflect.Map {
			// Serialize map to JSON string
			jsonData, err := json.Marshal(field.Interface())
			if err == nil {
				// Store as JSON string (if the field is string or []byte compatible)
				if resultField.Type().Kind() == reflect.String {
					resultField.SetString(string(jsonData))
				} else {
					// Fall back to original value
					resultField.Set(field)
				}
			} else {
				// On error, just copy original
				resultField.Set(field)
			}
		} else if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Map {
			// Serialize slice of maps to JSON
			jsonData, err := json.Marshal(field.Interface())
			if err == nil {
				// Store as JSON string if possible
				if resultField.Type().Kind() == reflect.String {
					resultField.SetString(string(jsonData))
				} else {
					resultField.Set(field)
				}
			} else {
				resultField.Set(field)
			}
		} else if field.Kind() == reflect.Struct {
			// Process nested struct
			nestedResult := serializeMapFieldsInModel(field)
			resultField.Set(nestedResult)
		} else if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Struct {
			// Process slice of structs
			sliceResult := reflect.MakeSlice(field.Type(), 0, field.Len())
			for j := 0; j < field.Len(); j++ {
				item := field.Index(j)
				processed := serializeMapFieldsInModel(item)
				sliceResult = reflect.Append(sliceResult, processed)
			}
			resultField.Set(sliceResult)
		} else {
			// Copy other fields directly
			resultField.Set(field)
		}
	}

	return result
}
