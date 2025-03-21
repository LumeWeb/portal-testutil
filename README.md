# Portal Testing Library

This package provides a comprehensive testing framework for database-backed services in the portal ecosystem. It is designed to simplify testing by providing a fluent interface for building SQL expectations and a registry pattern for mock services.

## What's New in v0.2.13

### Enhanced Relationship Support with BuildRowsWithRelations

A new method `BuildRowsWithRelations` has been added to properly handle GORM relationships in tests and solve the "unsupported data type: &map[]" error that commonly occurs when testing models with relationships:

```go
// Set up test data with relationships
parents := []models.Parent{
    {
        Model: gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
        Name:  "Parent 1",
        Children: []models.Child{
            {
                Model:    gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
                Name:     "Child 1",
                ParentID: 1,
            },
        },
    },
}

// Use enhanced builder with relationship support
rows := tc.BuildRowsWithRelations("parents", parents)

// Use in test expectations
tc.Raw().ExpectQuery("SELECT (.+) FROM `parents`").WillReturnRows(rows)

// Now the query will execute without "unsupported data type: &map[]" errors
var results []models.Parent
tc.DB().Find(&results)
```

This enhancement:
- Serializes relationship structs to JSON column values
- Handles belongs-to, has-one, and has-many relationships
- Maintains proper foreign key references
- Works with nested relationship structures
- Avoids the common "unsupported data type: &map[]" error
- For advanced usage, includes an SQLRelationshipScanner for full relationship reconstruction

## What's New in v0.2.12

### Explicit Argument Matching with WithArgs

You can now explicitly specify the arguments to match in SQL queries with the new `WithArgs` method:

```go
// Match exact arguments in parameterized queries
testCtx.ForTable("users")
    .ExpectCount()
    .Where("status = ?")          // FIRST set up the SQL pattern with placeholders
    .WithArgs("active")           // THEN specify the args to match (in same order)
    .ReturnCount(10)

// Especially useful with queryutil.Filter or other WHERE clause generators
testCtx.ForTable("products")
    .ExpectFind()
    .Where("category = ? AND price >= ?")    // SQL pattern with placeholders
    .WithArgs("electronics", 199.99)         // Arguments in same order as placeholders
    .ReturnModels(productModels)
```

**Important usage notes:**
1. You **MUST** call `Where()` before `WithArgs()` - the `WithArgs` method only sets the arguments to match and does not create any SQL pattern
2. The arguments passed to `WithArgs()` must match the order of placeholders in the `Where()` pattern
3. Common mistake: Using `WithArgs()` without first setting up the pattern with `Where()`

This feature makes it easier to test code that generates SQL queries with parameters, such as filter functions, search utilities, or dynamic query builders.

## What's New in v0.2.11

### Direct Model Return Support

You can now directly pass model structs to expectation builders without manually converting them to rows:

```go
// Old approach (still supported):
rows := testCtx.BuildRowsFrom("users", userModels)
testCtx.ForTable("users").ExpectFind().ReturnRows(rows)

// New approach with ReturnModels:
testCtx.ForTable("users").ExpectFind().ReturnModels(userModels)

// Also works with search expectations:
testCtx.ForTable("products").ExpectSearch("phone").ReturnModels(productModels)

// And custom queries:
testCtx.Expect().Query("SELECT \\* FROM users").ReturnModels("users", userModels)
```

This feature makes database testing even more concise and intuitive, eliminating the manual step of converting model structs to rows.

## Architecture

The library properly extends the core Portal testing facilities, providing specialized utilities for database testing, SQL expectations, and service mocking. It follows these design principles:

1. **Core Integration**: Built on top of Portal's core testing framework through direct embedding
2. **Fluent Interfaces**: Builder patterns for expressive test setups 
3. **Minimal Boilerplate**: Reduces common testing boilerplate
4. **Extensibility**: Easily extendable for specific testing needs
5. **Type Safety**: Uses Go generics for improved type safety and developer experience

## Generic Testing Utilities

The library now provides generic test utilities for improved type safety and developer experience:

### Type-Safe Service Testing

```go
// Create a test context
tc := NewDBTestContext(t)
defer tc.Teardown()

// Create and register a service with type safety
service := CreateAndRegisterService[*UserService](tc, "user_service")

// Access the service directly with proper typing
result, err := service.GetUser(1)

// Or get the service from the context with type safety
userService := GetService[*UserService](tc, "user_service")
```

### Service Initialization Flexibility

Services can receive test dependencies in two ways:

1. **Reflection-based field setting** (works with exported fields)
```go
type TestService struct {
    Ctx    *DBTestContext
    Db     *gorm.DB
    Logger *core.Logger
}
```

2. **Interface-based initialization** (works with any field naming)
```go
type MyService struct {
    ctx    *DBTestContext // unexported fields
    db     *gorm.DB
    logger *core.Logger
}

// Implement ServiceInitializer for testing
func (s *MyService) InitForTest(tc *DBTestContext, db *gorm.DB, logger *core.Logger) {
    s.ctx = tc
    s.db = db
    s.logger = logger
}
```

### Automatic Model Relationship Registration

```go
// Register models with auto-discovery of relationships
RegisterModelWithRelationships[UserModel](tc)

// Models related to UserModel (e.g., Profile, Post) are automatically registered
```

### Transaction Pattern Helpers

```go
// Simplified transaction expectation setup
ExpectServiceTransaction(tc, ServiceTransactionOptions{
    TableName:    "users",
    FindID:       1,
    CreateID:     2,
    ExpectUpdate: true,
    RowBuilder:   func() *sqlmock.Rows { return UserRowBuilder() },
})
```

### TestSuite for Organized Testing

```go
// Create a typed test suite
suite := NewTestSuite[*UserService](t, "user_service")
defer suite.Teardown()

// Register mocks
mockDep := new(MockDependency)
mockDep.On("GetData").Return("test data", nil)
suite.RegisterMock("dependency_service", mockDep)

// Test the service using the type-safe reference
result, err := suite.ServiceUnderTest.DoSomething()
```

### Standardized Logger Creation

```go
// Create a standardized test logger
logger := NewTestLogger() // Returns a properly configured core.Logger
```

## Benefits

- **Type Safety**: Compile-time checking of service types
- **Developer Experience**: Enhanced IDE code completion and refactoring support
- **Flexible Initialization**: Support for both exported and unexported fields
- **Reduced Boilerplate**: Less setup code in tests
- **Automatic Relationship Discovery**: Models and their relationships registered automatically
- **Standardized Patterns**: Consistent approach to service testing

## Enhanced Testing Features

The test utilities in this package provide sophisticated support for many GORM features, including:

### Advanced GORM Model Support

`BuildRowsFrom` has advanced support for GORM models:

```go
// A model with relationships and gorm.Model (which includes DeletedAt)
type Case struct {
    gorm.Model            // Includes ID, CreatedAt, UpdatedAt, and DeletedAt
    ReferenceNumber string
    ReporterID      uint
    Reporter        Reporter `gorm:"foreignKey:ReporterID"` // Relationship field
    Messages        []Message `gorm:"foreignKey:CaseID"`    // Has-many relationship
}

// Build mock rows directly from your model:
rows := testCtx.BuildRowsFrom("cases", []models.Case{
    {
        Model:           gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
        ReferenceNumber: "CASE-123",
        ReporterID:      1,
        Reporter:        models.Reporter{Model: gorm.Model{ID: 1}, Name: "John"},
        Messages:        []models.Message{{Content: "Test"}},
    },
})

// Use in SQL expectations
testCtx.ForTable("cases").ExpectFind().ReturnRows(rows)

// Or more directly with ReturnModels (new in v0.2.11):
testCtx.ForTable("cases").ExpectFind().ReturnModels(caseModels)
```

### Transaction Support for Complex Models

The transaction utilities handle complex model scenarios, including models with both relationships and hooks:

```go
// A model with both hooks and relationships
type BugModel struct {
    gorm.Model
    Name       string
    RelatedID  uint
    Related    *RelationshipOnlyModel `gorm:"foreignKey:RelatedID"`
}

// Has a lifecycle hook
func (m *BugModel) BeforeCreate(tx *gorm.DB) error {
    return nil
}

// Register and use in transactions
testCtx.RegisterModel(&BugModel{})
err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    return tx.Create(&BugModel{Name: "test", RelatedID: 1}).Error
})
```

### Map Values and Custom Table Names

Transaction handling supports both map values and models with custom TableName() methods:

```go
// Register a model with relationships
testCtx.RegisterModel(&models.Communication{})

// Works with struct models
err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    return tx.Create(&models.Communication{
        CaseID:    1,
        Content:   "test content",
        Direction: "incoming",
    }).Error
})

// Works with map values
err = testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    values := map[string]interface{}{"name": "test"}
    return tx.Model(&MyModel{}).Create(values).Error
})

// Works with custom TableName() methods
err = testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    return tx.Create(&MyModel{Name: "test"}).Error
})
```

## Feature Overview

### Transaction Testing

```go
// Execute operations in a transaction with proper table resolution
err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    // Table name is properly resolved for registered models
    return tx.Create(&MyModel{Name: "test"}).Error
})

// Force rollback regardless of result
err := testCtx.Transaction().WithRollbackOnly(func(tx *gorm.DB) error {
    return tx.First(&user, 1).Error
})

// Force commit regardless of result
err := testCtx.Transaction().WithCommitOnly(func(tx *gorm.DB) error {
    return tx.Create(&model).Error
})
```

### Model Registration

```go
// Register a single model
testCtx.RegisterModel(&models.User{})

// Register multiple models
testCtx.RegisterModels(&models.Post{}, &models.Comment{})

// Generic registration with relationship auto-discovery
RegisterModelWithRelationships[UserModel](testCtx)
```

### Expectation Building

```go
// Simple create expectation with auto-transaction handling
testCtx.ForTable("users").ExpectCreate(1)

// Create with error simulation
testCtx.ForTable("users").ExpectCreateError(errors.New("duplicate key"))

// Find by ID with returned rows
testCtx.ForTable("users")
    .ExpectFind()
    .ByID(1)
    .ReturnRows(userRows)

// Count with conditions
testCtx.ForTable("users")
    .ExpectCount()
    .Where("status = ?", "active")
    .ReturnCount(10)

// First() query with GORM's pattern
testCtx.ForTable("users")
    .ExpectFind()
    .Where("username = ?", "johndoe")
    .HandleStandardFirstRows(userRows)
```

### Row Building

```go
// Build rows from structs
userRows := testCtx.BuildRowsFrom("users", []models.User{
    {
        Model:    gorm.Model{ID: 1},
        Username: "johndoe", 
        Email:    "john@example.com",
    },
    {
        Model:    gorm.Model{ID: 2},
        Username: "janedoe",
        Email:    "jane@example.com", 
    },
})

// Or build manually
rows := testutil.NewModelRowBuilder("name", "email")
    .AddModelRow(1, "John Doe", "john@example.com")
    .AddModelRow(2, "Jane Smith", "jane@example.com")
    .Build()
```

## Key Components

### 1. DBTestContext

`DBTestContext` extends the portal's core test context by embedding it and adding database testing capabilities:

```go
// Create a test context with default configuration
testCtx := testutil.NewDBTestContext(t)
defer testCtx.Teardown()

// Or with custom configuration
testCtx := testutil.NewDBTestContext(t, 
    testutil.WithTablePrefix("my_prefix_"))

// Register a service
testCtx.RegisterService("my_service", myMockService)

// Register GORM models for model-based API support
testCtx.RegisterModel(&models.User{})
testCtx.RegisterModels(&models.Post{}, &models.Comment{})

// Get the table name for a model (useful in expectations)
tableName := testCtx.TableNameForModel(&models.User{}) // "users"

// Apply table prefix to a name
prefixedName := testCtx.PrefixTableName("users") // "my_prefix_users"

// Verify expectations at the end of the test
testCtx.VerifyExpectations()
```

### 2. ExpectationsBuilder

`ExpectationsBuilder` provides a fluent interface for building SQL expectations:

```go
// Set up expectations for a transaction
testCtx.ForTable("users")
    .ExpectTransaction()
    .Insert(1)
    .Commit()

// Set up expectations for a create operation (with auto-transaction handling)
testCtx.ForTable("users")
    .ExpectCreate(1)

// Set up expectations for a create operation (with manual transaction handling)
tc.mock.ExpectBegin()
testCtx.ForTable("users")
    .ExpectCreate(1, false) // Pass false to disable auto handling
tc.mock.ExpectCommit()

// Set up expectations for a create error (with auto-transaction handling)
testCtx.ForTable("users")
    .ExpectCreateError(errors.New("duplicate key"))

// Set up expectations for a create error (with manual transaction handling)
tc.mock.ExpectBegin()
testCtx.ForTable("users")
    .ExpectCreateError(errors.New("duplicate key"), false) // Pass false to disable auto handling
tc.mock.ExpectRollback()

// Set up expectations for a find operation
testCtx.ForTable("items")
    .ExpectFind()
    .ByID(1)
    .ReturnRows(rows)
    
// Find query with explicit arguments (new in v0.2.12)
testCtx.ForTable("users")
    .ExpectFind()
    .Where("email = ? AND status = ?")
    .WithArgs("john@example.com", "active") // Explicit argument matching
    .ReturnModels(userModels)

// ExpectCount - Several ways to use it:

// 1. Simple count expectation (original style, now GORM-compatible by default)
testCtx.ForTable("items")
    .ExpectCount(5)

// 2. Simple count with custom column name (for backward compatibility)
testCtx.ForTable("items")
    .ExpectCount(5, WithColumnName("count"))

// 3. Count with builder and ReturnCount
testCtx.ForTable("items")
    .ExpectCount()
    .ReturnCount(10)

// 4. Count with custom column name 
testCtx.ForTable("items")
    .ExpectCount(WithColumnName("cnt"))
    .ReturnCount(10)

// 5. Count with error simulation
testCtx.ForTable("items")
    .ExpectCount()
    .ReturnError(fmt.Errorf("database error"))

// 6. Count with WHERE condition using internal args (legacy approach)
testCtx.ForTable("items")
    .ExpectCount()
    .Where("status = ?", "active")
    .ReturnCount(3)

// 7. Count with WHERE condition and explicit args (new in v0.2.12)
testCtx.ForTable("items")
    .ExpectCount()
    .Where("status = ?")
    .WithArgs("active") // Explicitly specify the argument to match
    .ReturnCount(3)

// 8. Count with WHERE condition and error
testCtx.ForTable("items")
    .ExpectCount()
    .Where("region = ?", "unknown")
    .ReturnError(errors.New("region not found"))
```

### 3. Validation Testing

`ValidationTester` provides utilities for testing validation logic:

```go
// Get validation helper 
validator := testCtx.Validation()

// Test required fields
validator.AssertRequiredField(validateUsername, "username")

// Test string length
validator.AssertMinLength(validatePassword, "password", 8)
validator.AssertMaxLength(validateUsername, "username", 50)

// Test email format
validator.AssertEmailFormat(validateEmail, "email")

// Test URL format
validator.AssertURLFormat(validateWebsite, "website")

// Test numeric range
validator.AssertNumericRange(validateAge, "age", 18, 120)
```

### 4. Transaction Testing

Enhanced transaction testing capabilities with robust table name resolution:

```go
// Use the transaction helper for automatic commit/rollback
err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    // Perform operations within a transaction
    return nil
})

// Force rollback even if successful
err := testCtx.Transaction().WithRollbackOnly(func(tx *gorm.DB) error {
    // Operation will be rolled back regardless of return value
    return nil
})

// Force commit even if error
err := testCtx.Transaction().WithCommitOnly(func(tx *gorm.DB) error {
    // Operation will be committed regardless of return value
    return nil
})

// New in v0.2.1: Works with models that have custom TableName() methods 
// regardless of how they were registered:
testCtx.RegisterModel(&MyModel{}) // Standard registration
RegisterModelWithRelationships[MyModel](testCtx) // Generic registration

// Table name will be correctly resolved in all scenarios
err = testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    return tx.Create(&MyModel{Name: "test"}).Error
})

// New in v0.2.1: Works with map values in Create operations
err = testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    values := map[string]interface{}{"name": "test"}
    return tx.Model(&MyModel{}).Create(values).Error
})

// New in v0.2.2: Works with complex models that have relationships
testCtx.RegisterModel(&models.Communication{}) // Model with relationships to other models

// Works correctly with complex models without requiring explicit table setting
err = testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    return tx.Create(&models.Communication{
        CaseID:    1,
        Content:   "test content",
        Direction: "incoming",
    }).Error
})

// New in v0.2.2: Works with all CRUD operations on complex models
err = testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
    // Create, Find, Update, and Delete all work correctly
    // with complex models that have relationships
    
    // 1. Create
    comm := &models.Communication{...}
    if err := tx.Create(comm).Error; err != nil {
        return err
    }
    
    // 2. Find
    var found models.Communication
    if err := tx.First(&found, comm.ID).Error; err != nil {
        return err
    }
    
    // 3. Update
    if err := tx.Model(&models.Communication{}).
        Where("id = ?", comm.ID).
        Update("content", "updated").Error; err != nil {
        return err
    }
    
    // 4. Delete - soft delete correctly handled
    if err := tx.Delete(&models.Communication{}, comm.ID).Error; err != nil {
        return err
    }
    
    return nil
})

// New in v0.2.1: No more callback warning messages in test output
// thanks to unique session IDs and precise callback tracking
```

### 5. Concurrent Testing

Testing concurrent operations:

```go
// Run concurrent operations
errors := testCtx.Concurrent().RunWithConcurrency(10, func(id int) error {
    // Perform concurrent operations
    return nil
})

// Run with specific timeout
errors := testCtx.Concurrent().RunWithConcurrencyAndTimeout(
    10, 30*time.Second, func(id int) error {
    // Perform concurrent operations with longer timeout
    return nil
})

// Run parallel database queries
errors := testCtx.Concurrent().RunParallelQueries(5, func(id int, dbCtx *DBTestContext) error {
    // Execute database queries in parallel
    return nil
})
```

### 6. ServiceMockRegistry

`ServiceMockRegistry` provides a registry for mock service implementations:

```go
// Register a mock factory
testutil.RegisterMockFactory("my_service", func() core.Service {
    return new(MockMyService)
})

// Set up a mock service in a test
testutil.WithService("my_service", func(m *mock.Mock) {
    m.On("GetItem", uint(1)).Return(myItem, nil)
})(testCtx)
```

### 7. Row Builders and BuildRows

The library provides multiple ways to create mock SQL rows for tests, ranging from manual creation to automatic building from structs and maps.

#### BuildRows - Simplified Row Creation from Go Objects

The `BuildRows` and `BuildRowsFrom` methods on `DBTestContext` make it easy to create mock SQL rows directly from Go structs or maps:

```go
// Create rows from a map of column values
reporterRows := testCtx.BuildRows("reporters", map[string]interface{}{
    "id":         1,
    "created_at": now,
    "updated_at": now,
    "deleted_at": nil,
    "email":      "reporter1@example.com",
    "name":       "Reporter One",
    "user_id":    nil,
})

// Create rows from a slice of structs with GORM models
reporterRows := testCtx.BuildRowsFrom("reporters", []models.Reporter{
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

// Use the rows in an expectation
testCtx.ForTable("reporters").ExpectFindAll().ReturnRows(reporterRows)
```

Key features:
- Works with maps, structs, or slices of maps/structs
- Automatically handles GORM models and embedded structs
- Special handling for gorm.DeletedAt fields for soft delete functionality
- Intelligent handling of relationship fields (extracts IDs from relation structs)
- Skips has-many relationship fields (slices/arrays) that don't map to direct columns
- Preserves column names from GORM tags (`column:name`)
- Converts field names to snake_case when needed
- Handles nil values properly

#### Manual Row Builders

For more control, you can use the manual row builders directly:

```go
// Create model rows with additional fields
rows := testutil.NewModelRowBuilder("name", "email")
    .AddModelRow(1, "John Doe", "john@example.com")
    .AddModelRow(2, "Jane Smith", "jane@example.com")
    .Build()

// Add rows from maps
rows := testutil.NewRowBuilder("id", "name", "email")
    .AddRowWithMap(map[string]interface{}{
        "id": 1,
        "name": "John Doe",
        "email": "john@example.com",
    })
    .Build()

// Create rows with wrapper for compatibility with SQL interface
rowsWrapper := testutil.NewRowBuilder("id", "name")
    .AddRow(1, "John")
    .BuildWithWrapper()

// Now you can use Next() and Scan() methods like real database rows
for rowsWrapper.Next() {
    var id int
    var name string
    rowsWrapper.Scan(&id, &name)
}
```

The row wrapper's scanning system provides comprehensive type conversion, supporting:

- Basic types: string, []byte, int, int64, uint, float64, bool
- Time values: time.Time, *time.Time
- String conversions for numeric and boolean types
- Reflection-based fallbacks for other types

This makes the mock rows behave almost identically to real database rows, reducing test breakage when refactoring.

### 8. Search Testing

Testing search functionality:

```go
// Set up search expectations
testCtx.Search()
    .WithQuery("john")
    .WithFields("name", "email")
    .WithPageSize(10)
    .WithPage(1)
    .ExpectCount(2)
    .ReturnResults(rows)

// Or use the fluent interface directly with ForTable
testCtx.ForTable("users")
    .ExpectSearch("john")
    .WithFields("name", "email")
    .ReturnRows(rows)
```

### 9. Test Helpers

The package provides multiple test helpers for different testing scenarios:

- `RunServiceTests` - For testing service methods
- `RunTransactionTests` - For testing transaction behavior
- `RunConcurrentTests` - For testing concurrent operations
- `RunTestScenario` - For testing multi-step scenarios

## Example Usage

### Service Method Test

```go
// Run tests with default configuration
testutil.RunServiceTests(t, []testutil.ServiceTestCase{
    {
        Name: "Successfully get an item",
        SetupMock: func(tc *testutil.DBTestContext) {
            // Set up SQL expectations
            tc.ForTable("items")
                .ExpectFind()
                .ByID(1)
                .ReturnRows(
                    testutil.NewModelRowBuilder("name", "description")
                        .AddModelRow(1, "Test Item", "A test item")
                        .Build(),
                )
        },
        ExecuteTest: func(ctx core.Context) (interface{}, error) {
            // Execute the service method
            service := ctx.Service("my_service").(MyService)
            return service.GetItem(1)
        },
        ExpectResult: myExpectedItem,
    },
})
```

### Service Method Test with Model Registration

```go
func TestUserService_SearchUsers(t *testing.T) {
    // Create test context
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Register models - enables model-based GORM API support
    testCtx.RegisterModel(&models.User{})
    
    // Create models to build rows from
    users := []models.User{
        {
            Model:    gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
            Username: "johndoe", 
            Email:    "john@example.com",
            Status:   "active",
        },
        {
            Model:    gorm.Model{ID: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()},
            Username: "janedoe",
            Email:    "jane@example.com", 
            Status:   "active",
        },
    }
    
    // Build rows directly from user models
    userRows := testCtx.BuildRowsFrom("users", users)
    
    // Set up expectation for count query
    testCtx.ForTable("users")
        .ExpectCount()
        .Where("(username LIKE ? OR email LIKE ?) AND status = ?")
        .ReturnCount(2)
    
    // Set up expectation for find query - two equivalent approaches
    // Traditional approach with pre-built rows:
    testCtx.ForTable("users")
        .ExpectFind()
        .Where("(username LIKE ? OR email LIKE ?) AND status = ?")
        .ReturnRows(userRows)
        
    // Or with v0.2.11 direct models approach:
    testCtx.ForTable("users")
        .ExpectFind()
        .Where("(username LIKE ? OR email LIKE ?) AND status = ?")
        .ReturnModels(users)
    
    // Create service
    service := NewUserService(testCtx.DB())
    
    // Execute search method (uses db.Model(&models.User{}) internally)
    results, total, err := service.SearchUsers("doe", "active")
    
    // Assert results
    assert.NoError(t, err)
    assert.Equal(t, int64(2), total)
    assert.Len(t, results, 2)
    assert.Equal(t, "johndoe", results[0].Username)
    assert.Equal(t, "janedoe", results[1].Username)
    
    // Verify all expectations were met
    testCtx.VerifyExpectations()
}
```

### Transaction Test

```go
// Transaction tests
testutil.RunTransactionTests(t, []testutil.TransactionTestCase{
    {
        Name: "Transaction rolls back on error",
        SetupMock: func(tc *testutil.DBTestContext) {
            tc.ForTable("items")
                .ExpectTransaction()
                .InsertError(errors.New("database error"))
                .Rollback()
        },
        ExecuteService: func(ctx core.Context) error {
            service := ctx.Service("my_service").(MyService)
            return service.CreateItem(myItem)
        },
        ExpectError:    true,
        ErrorContains:  "database error",
    },
})
```

### Validation Test

```go
func TestUserValidation(t *testing.T) {
    // Create test context
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Get validation helper
    validator := testCtx.Validation()
    
    // Test email validation
    validator.AssertEmailFormat(user.ValidateEmail, "email")
    
    // Test password validation
    validator.AssertMinLength(user.ValidatePassword, "password", 8)
}
```

### Multi-step Scenario Test

```go
// Run with default configuration
testutil.RunTestScenario(t, testutil.TestScenario{
    Name:        "Create and retrieve an item",
    Description: "Tests creating an item and then retrieving it",
    SetupMock: func(tc *testutil.DBTestContext) {
        // 1. Create item
        tc.ForTable("items")
            .ExpectTransaction()
            .Insert(1)
            .Commit()
            
        // 2. Get item
        tc.ForTable("items")
            .ExpectFind()
            .ByID(1)
            .ReturnRows(
                testutil.NewModelRowBuilder("name", "description")
                    .AddModelRow(1, "Test Item", "A test item")
                    .Build(),
            )
    },
    Steps: []testutil.TestStep{
        {
            Name: "Create a new item",
            ExecuteTest: func(ctx core.Context) (interface{}, error) {
                service := ctx.Service("my_service").(MyService)
                return service.CreateItem(myItem)
            },
            ExpectResult: uint(1),
        },
        {
            Name: "Retrieve the created item",
            ExecuteTest: func(ctx core.Context) (interface{}, error) {
                service := ctx.Service("my_service").(MyService)
                return service.GetItem(1)
            },
            ExpectResult: myExpectedItem,
        },
    },
})
```

## Implementing a Mock Service

To implement a mock service that works with this library:

```go
type MockMyService struct {
    mock.Mock
}

// ID returns the service ID
func (m *MockMyService) ID() string {
    return "my_service"
}

// Config returns the service configuration
func (m *MockMyService) Config() (any, error) {
    return nil, nil
}

// Mock returns the underlying mock object
func (m *MockMyService) Mock() *mock.Mock {
    return &m.Mock
}

// GetItem demonstrates a simple service method
func (m *MockMyService) GetItem(id uint) (interface{}, error) {
    args := m.Called(id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0), args.Error(1)
}

func init() {
    // Register the mock factory
    testutil.RegisterMockFactory("my_service", func() core.Service {
        return new(MockMyService)
    })
}
```

## Troubleshooting

### "Table not set" Errors in Transactions

If you encounter errors like `Table not set, please set it like: db.Model(&user) or db.Table("users")` when using GORM transactions, this is typically related to how GORM resolves table names within transactions.

This issue usually occurs with:

1. Models that have relationships (fields with struct types or slices of structs)
2. Models that have lifecycle hooks (BeforeCreate, BeforeUpdate, etc.) that call other methods
3. Models with more complex structures

**Solutions:**

1. **Register your models with `RegisterModelWithRelationships`:**
   ```go
   // Register both the main model and its relationships
   testutil.RegisterModelWithRelationships[MyModel](testContext)
   ```

2. **Use `testContext.Transaction()` instead of direct GORM transactions:**
   ```go
   // Use this pattern for consistent table resolution
   err := testContext.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
       return tx.Create(&myModel).Error
   })
   ```

3. **If all else fails, explicitly set the table name:**
   ```go
   err := testContext.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
       // Explicitly set the table name
       return tx.Table("my_models").Create(&myModel).Error
   })
   ```

The first approach is strongly recommended as it provides the most reliable solution by fully registering your model and its relationships with the test context.

### Callback Registration Warnings

If you see warnings about duplicate callbacks being registered, it's likely that multiple transaction helpers are being created. As of v0.2.1, the library uses unique session IDs to prevent these warnings, but if you're using an older version, upgrade or follow these practices:

1. Create a single transaction helper per test
2. Reuse the same helper for multiple operations

Example:
```go
// Create a single helper for the test
txHelper := testCtx.Transaction()

// Use it for multiple transaction operations
err1 := txHelper.ExecuteInTransaction(func(tx *gorm.DB) error {
    // First operation
})

err2 := txHelper.ExecuteInTransaction(func(tx *gorm.DB) error {
    // Second operation
})
```

## Mailer Testing

The `MailerTestHelper` provides utilities for testing email functionality. It implements the `core.MailerService` interface and captures emails instead of sending them.

### Usage

```go
// Create test context
tc := NewDBTestContext(t)
defer tc.Teardown()

// Create mailer test helper
mailerHelper := NewMailerTestHelper(t)

// Register test templates
mailerHelper.RegisterTemplate(
    "notification_template",
    "Notification: {{.Subject}}",
    "Hello,\n\n{{.Body}}\n\nRegards,\nThe System"
)

// Register the mailer with the test context
mailerHelper.RegisterWithContext(tc)

// Create your service that uses the mailer
service := NewYourService(tc.Context)

// Test your service method that sends an email
err := service.SendNotification("user@example.com", "Important Update", "This is an important update.")
assert.NoError(t, err)

// Assert that the email was sent
email := mailerHelper.AssertEmailSent("notification_template", "user@example.com")

// Assert the email content
mailerHelper.AssertEmailContent(email, "Notification: Important Update", "This is an important update.")

// Verify that the variables were passed correctly
assert.Equal(t, "Important Update", email.SubjectVars["Subject"])
assert.Equal(t, "This is an important update.", email.BodyVars["Body"])
```
