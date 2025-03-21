// Package testutil provides utilities for testing service components within the Portal ecosystem.
package testutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// # Relationship Preloading Support
//
// These utilities provide enhanced relationship handling for GORM models in test environments.
// When testing with GORM models that have relationships (has-many, has-one, belongs-to), you'll
// often encounter an issue where the relationship fields remain empty in test results, even though
// they were populated in your test data.
//
// This is because:
// 1. Standard SQL mocking serializes models to basic SQL rows
// 2. GORM's relationship loading requires explicit Preload() calls
// 3. When testing, these Preload() calls require separate mock expectations
//
// ReturnModelsWithPreload solves this by:
// 1. Setting up the basic SQL rows using BuildRowsWithRelations
// 2. Registering a GORM callback that automatically injects relationship data
// 3. Eliminating the need for explicit Preload() calls in the test or separate mock expectations
//
// This makes testing with relationship models much simpler and more intuitive.
//
// # Example Usage
//
//	// Create test data with relationships
//	parentWithChildren := Parent{
//	    ID:   1,
//	    Name: "Parent",
//	    Children: []Child{
//	        {ID: 1, Name: "Child 1", ParentID: 1},
//	        {ID: 2, Name: "Child 2", ParentID: 1},
//	    },
//	}
//
//	// Set up expectation with relationship preloading
//	tc.ForTable("parents").
//	    ExpectFind().
//	    ReturnModelsWithPreload([]Parent{parentWithChildren})
//
//	// Execute query - NO explicit Preload needed!
//	var results []Parent
//	err := service.DB().Find(&results).Error
//
//	// Children will be populated automatically
//	assert.Len(t, results[0].Children, 2)
//	assert.Equal(t, "Child 1", results[0].Children[0].Name)
//
// Compare this with standard ReturnModels behavior, which would require:
//
//	// Set up the main query
//	tc.ForTable("parents").
//	    ExpectFind().
//	    ReturnModels([]Parent{parentWithChildren})
//
//	// Set up a SEPARATE expectation for the Preload query
//	tc.Raw().ExpectQuery("SELECT (.+) FROM `children` WHERE `children`.`parent_id` = ?").
//	    WithArgs(1).
//	    WillReturnRows(tc.BuildRowsFrom("children", parentWithChildren.Children))
//
//	// Execute query - MUST use Preload
//	var results []Parent
//	err := service.DB().Preload("Children").Find(&results).Error

// ReturnModelsWithPreload is an enhanced version of ReturnModels that automatically populates
// relationship fields in GORM query results.
//
// # Problem Solved
//
// When testing GORM models with relationships, you'll often encounter a frustrating issue:
// even though your test data includes populated relationship fields (like Children slices),
// these fields come back empty in your test results. This happens because:
//
// 1. SQL mocking only sets up rows for the main query, not relationship queries
// 2. GORM's relationship loading requires explicit Preload() calls
// 3. Each Preload() needs its own separate mock expectation
//
// # How ReturnModelsWithPreload Works
//
// This method solves the problem by:
//
// 1. Setting up the basic SQL rows using BuildRowsWithRelations
// 2. Extracting relationship data from your test models
// 3. Registering a GORM callback that injects this relationship data after the main query executes
// 4. Returning the completely populated models with all relationship fields intact
//
// The key advantage is that your test code doesn't need to use explicit Preload() calls
// or set up multiple expectations. Everything works with a single expectation.
//
// # Usage Example
//
//	// Create test data with relationships
//	parentWithChildren := models.Parent{
//	    ID: 1,
//	    Name: "Parent",
//	    Children: []models.Child{
//	        {ID: 1, Name: "Child 1", ParentID: 1},
//	        {ID: 2, Name: "Child 2", ParentID: 1},
//	    },
//	}
//
//	// Set up a single expectation with relationship support
//	tc.ForTable("parents").
//	    ExpectFind().
//	    ReturnModelsWithPreload([]models.Parent{parentWithChildren})
//
//	// Execute query - NO explicit Preload needed!
//	var results []models.Parent
//	err := service.DB().Find(&results).Error  // No Preload("Children") required!
//
//	// Relationships are automatically populated
//	assert.Equal(t, "Child 1", results[0].Children[0].Name)
//	assert.Equal(t, "Child 2", results[0].Children[1].Name)
//
// # Supported Relationships
//
// This method supports all GORM relationship types:
// - Has-Many relationships (slices of related models)
// - Has-One relationships (single related model)
// - Belongs-To relationships
// - Nested relationships (relationships of relationships)
//
// # Important Note
//
// While this method handles most relationship scenarios automatically, very complex cases like
// polymorphic relationships or manually constructed many-to-many relationships might require
// additional setup. For these cases, you can still use the traditional approach of setting up
// multiple expectations with the raw mock.
func (f *FindExpectationBuilder) ReturnModelsWithPreload(models any) *ExpectationsBuilder {
	// Use BuildRowsWithRelations to get the basic SQL rows
	rows := f.builder.tc.BuildRowsWithRelations(f.builder.table, models)

	// Extract relationship data from the models to attach to our custom hook
	relationshipData := extractRelationshipData(models)

	// Set up the find expectation with the rows
	expectationBuilder := f.ReturnRows(rows)

	// Generate a unique ID for this callback to prevent registration conflicts
	// If multiple tests use this feature, we don't want callback name collisions
	uniqueID := fmt.Sprintf("_%p", models)
	callbackName := "testutil:simulate_preload" + uniqueID

	// Register a custom hook to process the relationship data after the query is executed
	// This simulates what GORM would do with real Preload calls
	db := f.builder.tc.DB()

	// Register a custom callback to inject relationships after the query completes
	// This is the key part that makes relationship loading work
	db.Callback().Query().After("gorm:query").Register(callbackName, func(d *gorm.DB) {
		// Only process if there are results and we have relationship data
		dest := d.Statement.Dest
		if dest == nil || len(relationshipData) == 0 {
			return
		}

		// Apply the relationship data to the query results
		injectRelationships(dest, relationshipData)
	})

	return expectationBuilder
}

// ReturnModelsWithPreload for search expectations
func (s *SearchExpectationBuilder) ReturnModelsWithPreload(models any) *ExpectationsBuilder {
	// Use BuildRowsWithRelations to get the basic SQL rows
	rows := s.builder.tc.BuildRowsWithRelations(s.builder.table, models)

	// Extract relationship data from the models
	relationshipData := extractRelationshipData(models)

	// Set up the expectation with the rows
	expectationBuilder := s.ReturnRows(rows)

	// Generate a unique ID for this callback
	uniqueID := fmt.Sprintf("_%p", models)
	callbackName := "testutil:simulate_search_preload" + uniqueID

	// Register a custom hook to process the relationship data
	db := s.builder.tc.DB()
	db.Callback().Query().After("gorm:query").Register(callbackName, func(d *gorm.DB) {
		// Only process if there are results and we have relationship data
		dest := d.Statement.Dest
		if dest == nil || len(relationshipData) == 0 {
			return
		}

		// Inject the relationship data
		injectRelationships(dest, relationshipData)
	})

	return expectationBuilder
}

// extractRelationshipData extracts relationship fields from models into a map
// to be used for later relationship injection.
//
// This function uses reflection to walk through the provided models and identify
// relationship fields (structs or slices of structs). It then serializes these
// relationship fields to JSON and stores them in a map with keys that follow the pattern:
// "ModelType_ID_FieldName" -> JSONData
//
// For example, a Post model with ID 1 and a Comments field would generate a key:
// "Post_1_Comments" -> [{"ID":1,"Content":"Comment 1"},{"ID":2,"Content":"Comment 2"}]
//
// This map is later used by injectRelationships to restore the relationship data
// into query results.
func extractRelationshipData(models any) map[string]interface{} {
	data := make(map[string]interface{})

	// Use reflection to extract relationship fields from the models
	val := reflect.ValueOf(models)

	// Handle different types of input
	if val.Kind() == reflect.Slice {
		// For each model in the slice
		for i := 0; i < val.Len(); i++ {
			model := val.Index(i)
			extractModelRelationships(model, data)
		}
	} else if val.Kind() == reflect.Struct || (val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Struct) {
		// Single model
		extractModelRelationships(val, data)
	}

	return data
}

// extractModelRelationships extracts relationship fields from a single model.
//
// This is a helper function called by extractRelationshipData that processes each
// individual model. It identifies relationship fields by examining the struct's fields,
// looking for:
//
// - Struct fields that aren't basic types (like time.Time)
// - Slice fields containing structs (has-many relationships)
// - Pointer fields pointing to structs (nullable relationships)
//
// When a relationship field is found, it serializes the field value to JSON and
// stores it in the provided map with a key that uniquely identifies the relationship:
// "ModelType_ID_FieldName" -> JSONData
func extractModelRelationships(model reflect.Value, data map[string]interface{}) {
	// Handle pointer models
	if model.Kind() == reflect.Ptr {
		model = model.Elem()
	}

	// Only process structs
	if model.Kind() != reflect.Struct {
		return
	}

	// Get the model's primary key (ID)
	idField := model.FieldByName("ID")
	if !idField.IsValid() {
		return
	}

	modelID := fmt.Sprintf("%v", idField.Interface())
	modelType := model.Type().Name()

	// For each field in the model
	for i := 0; i < model.NumField(); i++ {
		field := model.Field(i)
		fieldType := model.Type().Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Check if this is a relationship field (struct or slice of structs)
		isRelationship := false

		// Check for slice types (has-many relationships)
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Struct {
			isRelationship = true
		} else if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Ptr {
			elemType := field.Type().Elem().Elem()
			if elemType.Kind() == reflect.Struct {
				isRelationship = true
			}
		} else if field.Kind() == reflect.Struct {
			// Skip common struct types that aren't relationships
			if field.Type().String() != "time.Time" && !strings.HasPrefix(field.Type().String(), "gorm.") {
				isRelationship = true
			}
		} else if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			// Pointer to struct
			if field.Type().Elem().String() != "time.Time" && !strings.HasPrefix(field.Type().Elem().String(), "gorm.") {
				isRelationship = true
			}
		}

		if isRelationship {
			// Serialize the relationship field to JSON
			jsonData, err := json.Marshal(field.Interface())
			if err == nil {
				// Store in our map as modelType_ID_fieldName -> jsonData
				key := fmt.Sprintf("%s_%s_%s", modelType, modelID, fieldType.Name)
				data[key] = jsonData
			}
		}
	}
}

// injectRelationships injects the extracted relationship data into the query results.
//
// This function uses reflection to walk through the query results and populate relationship
// fields with the data previously extracted by extractRelationshipData. For each model
// in the results, it:
//
// 1. Identifies the model type and ID
// 2. For each field in the model, checks if there's matching relationship data
// 3. If data exists, deserializes the JSON data into the field
// 4. Handles different field types (structs, slices, pointers) appropriately
//
// This effectively simulates what GORM's Preload functionality would do, but without
// requiring additional SQL queries or mock expectations.
func injectRelationships(dest interface{}, relationshipData map[string]interface{}) {
	// Use reflection to inject the relationships
	destVal := reflect.ValueOf(dest)

	// Handle different types of destinations
	if destVal.Kind() == reflect.Ptr {
		// Pointer to something (single value or slice)
		destVal = destVal.Elem()
	}

	// Handle slices of models
	if destVal.Kind() == reflect.Slice {
		// For each model in the slice
		for i := 0; i < destVal.Len(); i++ {
			item := destVal.Index(i)
			injectModelRelationships(item, relationshipData)
		}
	} else if destVal.Kind() == reflect.Struct {
		// Single model
		injectModelRelationships(destVal, relationshipData)
	}
}

// injectModelRelationships injects relationship data into a single model.
//
// This is a helper function called by injectRelationships that focuses on a single model.
// It identifies the model's type and ID, then checks if there's relationship data available
// for any of its fields. If a match is found, it deserializes the data and sets the field value.
//
// The function handles several types of relationship fields:
// - Slices of models (has-many relationships)
// - Structs (has-one/belongs-to relationships)
// - Pointers to structs (nullable relationships)
//
// The key matching pattern is: "ModelType_ID_FieldName" -> JSONData
func injectModelRelationships(model reflect.Value, relationshipData map[string]interface{}) {
	// Handle pointer models
	if model.Kind() == reflect.Ptr {
		model = model.Elem()
	}

	// Only process structs
	if model.Kind() != reflect.Struct {
		return
	}

	// Get the model's ID
	idField := model.FieldByName("ID")
	if !idField.IsValid() {
		return
	}

	modelID := fmt.Sprintf("%v", idField.Interface())
	modelType := model.Type().Name()

	// For each field in the model
	for i := 0; i < model.NumField(); i++ {
		field := model.Field(i)
		fieldType := model.Type().Field(i)

		// Skip unexported or unaddressable fields
		if !field.CanSet() {
			continue
		}

		// Calculate the key for this field
		key := fmt.Sprintf("%s_%s_%s", modelType, modelID, fieldType.Name)

		// Check if we have data for this relationship
		if jsonData, exists := relationshipData[key]; exists {
			// Found relationship data, deserialize and inject it
			jsonBytes, ok := jsonData.([]byte)
			if !ok {
				continue
			}

			// Create a new value of the appropriate type to deserialize into
			var newVal reflect.Value

			// Handle different field types
			if field.Kind() == reflect.Slice {
				// Create a new slice of the appropriate type
				newVal = reflect.New(field.Type())
				err := json.Unmarshal(jsonBytes, newVal.Interface())
				if err == nil && !newVal.Elem().IsNil() {
					field.Set(newVal.Elem())
				}
			} else if field.Kind() == reflect.Struct {
				// Create a new struct of the appropriate type
				newVal = reflect.New(field.Type())
				err := json.Unmarshal(jsonBytes, newVal.Interface())
				if err == nil {
					field.Set(newVal.Elem())
				}
			} else if field.Kind() == reflect.Ptr {
				// Create a new pointer to a struct
				newVal = reflect.New(field.Type().Elem())
				err := json.Unmarshal(jsonBytes, newVal.Interface())
				if err == nil {
					// Set the pointer field to the new value
					field.Set(newVal)
				}
			}
		}
	}
}
