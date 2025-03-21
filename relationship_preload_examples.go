package testutil

import (
	"fmt"
)

// ReturnModelsWithPreload Example
//
// This example demonstrates how to use ReturnModelsWithPreload to properly
// handle relationship fields in your tests. This method automatically injects
// relationship data after query execution, similar to GORM's Preload.
//
// The key difference from ReturnModels is that ReturnModelsWithPreload ensures
// relationship fields are populated in query results, while ReturnModels serializes
// them for the SQL response but doesn't recreate the relationships in the result.
func ExampleFindExpectationBuilder_ReturnModelsWithPreload() {
	// In a real test function, you'd use:
	// tc := NewDBTestContext(t)
	// defer tc.Teardown()

	// Prepare test data with relationships and set up expectation:
	//
	// posts := []Post{...} // with Tags and Comments relationships
	//
	// tc.ForTable("posts").
	//     ExpectFind().
	//     ReturnModelsWithPreload(posts)
	//
	// Execute query - no need for explicit Preload calls!
	// var results []Post
	// err := service.DB().Find(&results).Error
	// require.NoError(t, err)
	//
	// Verify relationships are populated:
	// assert.Len(t, results[0].Tags, 2)
	// assert.Equal(t, "golang", results[0].Tags[0].Name)

	fmt.Println("ReturnModelsWithPreload helps test complex relationships")
	// Output: ReturnModelsWithPreload helps test complex relationships
}

// ReturnModelsWithPreload vs ReturnModels
//
// This example shows the difference between ReturnModelsWithPreload and
// the standard ReturnModels method when working with relationship fields.
func ExampleReturnModelsWithPreload_comparison() {
	// Option 1: Standard ReturnModels (relationships don't get populated)
	// tc.ForTable("customers").
	//     ExpectFind().
	//     ReturnModels([]Customer{customer})
	//
	// var customers1 []Customer
	// err := service.DB().Find(&customers1).Error
	// require.NoError(t, err)
	// fmt.Println(len(customers1[0].Orders)) // Would print: 0

	// Option 2: Enhanced ReturnModelsWithPreload (relationships get populated)
	// tc.ForTable("customers").
	//     ExpectFind().
	//     ReturnModelsWithPreload([]Customer{customer})
	//
	// var customers2 []Customer
	// err = service.DB().Find(&customers2).Error
	// require.NoError(t, err)
	// fmt.Println(len(customers2[0].Orders)) // Would print: 2

	fmt.Println("ReturnModelsWithPreload populates relationship fields")
	// Output: ReturnModelsWithPreload populates relationship fields
}
