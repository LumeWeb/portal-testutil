package testutil

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// This file contains examples demonstrating how to use the BuildRows and BuildRowsFrom functions
// to create mock SQL rows for testing.

// ExampleReporter is an example model used to demonstrate the BuildRows functionality
type ExampleReporter struct {
	gorm.Model
	Email     string
	Name      string
	UserID    *uint
	IsActive  bool
	CreatedBy string
}

// ExampleBuildRows_FromMap demonstrates how to create mock rows from a map
func ExampleBuildRows_FromMap(testCtx *DBTestContext) {
	// Get the current time for the timestamps
	now := time.Now()

	// Create mock rows from a map
	reporterRows := testCtx.BuildRows("reporters", map[string]any{
		"id":         1,
		"created_at": now,
		"updated_at": now,
		"deleted_at": nil,
		"email":      "reporter1@example.com",
		"name":       "Reporter One",
		"user_id":    nil,
	})

	// Use the rows in an expectation
	testCtx.ForTable("reporters").ExpectFind().ReturnRows(reporterRows)

	// Example of creating multiple rows in one call by using BuildRowsFrom with a slice of maps
	allReporterRows := testCtx.BuildRowsFrom("reporters", []map[string]any{
		{
			"id":         1,
			"created_at": now,
			"updated_at": now,
			"email":      "reporter1@example.com",
			"name":       "Reporter One",
		},
		{
			"id":         2,
			"created_at": now,
			"updated_at": now,
			"email":      "reporter2@example.com",
			"name":       "Reporter Two",
		},
	})

	// Use in an expectation
	testCtx.ForTable("reporters").ExpectFind().ReturnRows(allReporterRows)
}

// ExampleBuildRowsFrom_WithStructs demonstrates how to create mock rows from structs
func ExampleBuildRowsFrom_WithStructs(testCtx *DBTestContext) {
	// Get the current time for the timestamps
	now := time.Now()

	// Create rows from a slice of structs
	reporterRows := testCtx.BuildRowsFrom("reporters", []ExampleReporter{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email:     "reporter1@example.com",
			Name:      "Reporter One",
			IsActive:  true,
			CreatedBy: "admin",
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email:     "reporter2@example.com",
			Name:      "Reporter Two",
			IsActive:  true,
			CreatedBy: "admin",
		},
	})

	// Use the rows in an expectation
	testCtx.ForTable("reporters").ExpectFind().ReturnRows(reporterRows)

	// You can also use a single struct
	singleReporterRow := testCtx.BuildRowsFrom("reporters", ExampleReporter{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email:     "reporter1@example.com",
		Name:      "Reporter One",
		IsActive:  true,
		CreatedBy: "admin",
	})

	// Use in an expectation
	testCtx.ForTable("reporters").ExpectFind().ByID(1).ReturnRows(singleReporterRow)
}

// ExampleReporterWithTags demonstrates a model with custom column name tags
type ExampleReporterWithTags struct {
	gorm.Model
	Email     string `gorm:"column:email_address"`
	Name      string `gorm:"column:full_name"`
	IsActive  bool   `gorm:"column:is_enabled"`
	CreatedBy string `gorm:"column:creator"`
}

// ExampleBuildRowsFrom_WithTags demonstrates how column names from tags are used
func ExampleBuildRowsFrom_WithTags(testCtx *DBTestContext) {
	// Get the current time for the timestamps
	now := time.Now()

	// Create a test model with tags that specify custom column names
	reporter := ExampleReporterWithTags{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email:     "reporter@example.com",
		Name:      "Test Reporter",
		IsActive:  true,
		CreatedBy: "admin",
	}

	// Build rows - the column names will be taken from the gorm tags
	rows := testCtx.BuildRowsFrom("reporters", reporter)

	// This will create rows with columns:
	// - id, created_at, updated_at, deleted_at (from gorm.Model)
	// - email_address (from Email field's gorm tag)
	// - full_name (from Name field's gorm tag)
	// - is_enabled (from IsActive field's gorm tag)
	// - creator (from CreatedBy field's gorm tag)

	// Use in an expectation
	testCtx.ForTable("reporters").ExpectFind().ByID(1).ReturnRows(rows)
}

// DemonstrateAllApproaches shows all BuildRows approaches in one example
func DemonstrateAllApproaches() {
	// In a real test, you would have a test context
	// testCtx := testutil.NewDBTestContext(t)
	var testCtx *DBTestContext // This is just for the example

	now := time.Now()

	// Approach 1: Using a map (best for a single row)
	mapRows := testCtx.BuildRows("reporters", map[string]any{
		"id":         1,
		"created_at": now,
		"updated_at": now,
		"email":      "reporter1@example.com",
		"name":       "Reporter One",
	})
	fmt.Println("Map rows created:", mapRows != nil)

	// Approach 2: Using a struct (best when working with models)
	structRows := testCtx.BuildRowsFrom("reporters", ExampleReporter{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Email: "reporter1@example.com",
		Name:  "Reporter One",
	})
	fmt.Println("Struct rows created:", structRows != nil)

	// Approach 3: Using a slice of structs (best for multiple rows)
	structSliceRows := testCtx.BuildRowsFrom("reporters", []ExampleReporter{
		{
			Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
			Email: "reporter1@example.com",
			Name:  "Reporter One",
		},
		{
			Model: gorm.Model{ID: 2, CreatedAt: now, UpdatedAt: now},
			Email: "reporter2@example.com",
			Name:  "Reporter Two",
		},
	})
	fmt.Println("Struct slice rows created:", structSliceRows != nil)

	// Approach 4: Using a slice of maps
	mapSliceRows := testCtx.BuildRowsFrom("reporters", []map[string]any{
		{
			"id":    1,
			"email": "reporter1@example.com",
			"name":  "Reporter One",
		},
		{
			"id":    2,
			"email": "reporter2@example.com",
			"name":  "Reporter Two",
		},
	})
	fmt.Println("Map slice rows created:", mapSliceRows != nil)
}
