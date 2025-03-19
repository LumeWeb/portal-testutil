package testutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// ExampleModelValidation demonstrates how to use the model validation utilities
func ExampleModelValidation() {
	// This is just an example and won't actually run

	// Define a simple model
	type User struct {
		gorm.Model
		Username string
		Email    string
		Age      int
	}

	// Define a validator function
	validateUser := func(model interface{}) error {
		user, ok := model.(*User)
		if !ok {
			return errors.New("invalid model type")
		}

		if user.Username == "" {
			return errors.New("username is required")
		}

		if user.Email == "" {
			return errors.New("email is required")
		}

		if user.Age < 18 {
			return errors.New("user must be at least 18 years old")
		}

		return nil
	}

	// Example test cases
	_ = []ModelTestCase{
		{
			Name: "Valid user",
			Model: &User{
				Username: "testuser",
				Email:    "test@example.com",
				Age:      25,
			},
			Validator:   validateUser,
			ExpectError: false,
		},
		{
			Name: "Missing username",
			Model: &User{
				Email: "test@example.com",
				Age:   25,
			},
			Validator:     validateUser,
			ExpectError:   true,
			ErrorContains: "username is required",
		},
		{
			Name: "Missing email",
			Model: &User{
				Username: "testuser",
				Age:      25,
			},
			Validator:     validateUser,
			ExpectError:   true,
			ErrorContains: "email is required",
		},
		{
			Name: "Underage user",
			Model: &User{
				Username: "testuser",
				Email:    "test@example.com",
				Age:      16,
			},
			Validator:     validateUser,
			ExpectError:   true,
			ErrorContains: "must be at least 18",
		},
	}

	// Run the tests
	// RunModelValidationTests(t, testCases)
}

// TestModelCRUDTester demonstrates how to use the ModelCRUDTester
func TestModelCRUDTester(t *testing.T) {
	// Skip this test as it's just an example
	t.Skip("This is just an example test")

	// Define a simple model
	type User struct {
		gorm.Model
		Username string
		Email    string
	}

	// Create a test context
	testCtx := NewDBTestContext(t)
	defer testCtx.Teardown()

	// Create a model CRUD tester
	tester := NewModelCRUDTester(t, testCtx, "users")

	// Test creating a user
	user := &User{
		Username: "testuser",
		Email:    "test@example.com",
	}

	// Create a row builder for the user
	builder := NewModelRowBuilder("username", "email")
	builder.AddModelRow(1, "testuser", "test@example.com")

	// Test CRUD operations
	tester.TestCreate(user, 1)

	// Test finding the user
	foundUser := &User{}
	tester.TestFind(1, foundUser, builder)

	// Test updating the user
	user.Email = "updated@example.com"
	tester.TestUpdate(user)

	// Test deleting the user
	tester.TestDelete(user)

	// Test listing users
	var users []User
	builder = NewModelRowBuilder("username", "email")
	builder.AddModelRow(1, "user1", "user1@example.com")
	builder.AddModelRow(2, "user2", "user2@example.com")

	tester.TestList(&users, nil, builder, 2)

	// Verify the results
	assert.Len(t, users, 2)
}

// ExampleModelCRUDTester demonstrates how to use the ModelCRUDTester in a real test
func ExampleModelCRUDTester() {
	// This is just an example and won't actually run

	// Define a model
	type Product struct {
		gorm.Model
		Name  string
		Price float64
	}

	// In your test:
	// t := &testing.T{}
	// testCtx := NewDBTestContext(t)
	// defer testCtx.Teardown()

	// Create a model CRUD tester
	// tester := NewModelCRUDTester(t, testCtx, "products")

	// Test creating a product
	// product := &Product{Name: "Test Product", Price: 19.99}
	// tester.TestCreate(product, 1)

	// Create a row builder for the product
	// builder := NewModelRowBuilder("name", "price")
	// builder.AddModelRow(1, "Test Product", 19.99)

	// Test finding the product
	// foundProduct := &Product{}
	// tester.TestFind(1, foundProduct, builder)

	// Test updating the product
	// product.Price = 24.99
	// tester.TestUpdate(product)

	// Test deleting the product
	// tester.TestDelete(product)
}
