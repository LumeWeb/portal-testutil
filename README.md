# Portal Testing Library

This package provides a comprehensive testing framework for database-backed services in the portal ecosystem. It is designed to simplify testing by providing a fluent interface for building SQL expectations and a registry pattern for mock services.

## Architecture

The library properly extends the core Portal testing facilities, providing specialized utilities for database testing, SQL expectations, and service mocking. It follows these design principles:

1. **Core Integration**: Built on top of Portal's core testing framework through direct embedding
2. **Fluent Interfaces**: Builder patterns for expressive test setups 
3. **Minimal Boilerplate**: Reduces common testing boilerplate
4. **Extensibility**: Easily extendable for specific testing needs

## Recent Enhancements

### ExpectCreate and ExpectCreateError with Automatic Transaction Handling

The library now provides `ExpectCreate` and `ExpectCreateError` methods that automatically handle GORM's transaction behavior. These methods offer a more intuitive API for testing GORM's Create operations:

Key features:
- Automatically handles GORM's transaction behavior (Begin/Commit/Rollback)
- More semantic API that matches GORM's terminology
- **Optional automatic transaction handling** - can be disabled by passing `false` as a second parameter
  - `ExpectCreate(1, false)` - disables auto transaction handling
  - `ExpectCreateError(err, false)` - disables auto transaction handling

**Example usage:**

```go
func TestUserService_CreateUser(t *testing.T) {
    // Create test context
    tc := testutil.NewDBTestContext(t)
    defer tc.Teardown()
    
    // Set up create expectation with automatic transaction handling
    tc.ForTable("users").ExpectCreate(1)
    
    // Create the service with the mocked DB
    service := NewUserService(tc.DB())
    
    // Call the service method that creates a user
    userID, err := service.CreateUser("johndoe", "john@example.com")
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, uint(1), userID)
    
    // Verify expectations
    tc.VerifyExpectations()
}

func TestUserService_CreateUser_Error(t *testing.T) {
    // Create test context
    tc := testutil.NewDBTestContext(t)
    defer tc.Teardown()
    
    // Set up create expectation with error and automatic transaction handling
    expectedErr := errors.New("duplicate email")
    tc.ForTable("users").ExpectCreateError(expectedErr)
    
    // Create the service with the mocked DB
    service := NewUserService(tc.DB())
    
    // Call the service method that creates a user
    _, err := service.CreateUser("johndoe", "john@example.com")
    
    // Assertions
    assert.Error(t, err)
    assert.Equal(t, expectedErr, err)
    
    // Verify expectations
    tc.VerifyExpectations()
}

// For cases where you need manual control over transactions
func TestUserService_CreateUser_ManualTransaction(t *testing.T) {
    tc := testutil.NewDBTestContext(t)
    defer tc.Teardown()
    
    // Disable automatic transaction handling
    tc.mock.ExpectBegin()
    tc.ForTable("users").ExpectCreate(1, false) // Pass false to disable auto handling
    tc.mock.ExpectCommit()
    
    // Test service as usual
    service := NewUserService(tc.DB())
    userID, err := service.CreateUser("johndoe", "john@example.com")
    
    assert.NoError(t, err)
    assert.Equal(t, uint(1), userID)
    tc.VerifyExpectations()
}
```

### Transaction Table Resolution for Registered Models

The transaction testing utilities have been enhanced to support registered models in GORM transactions. This solves the "Table not set" error that can occur in transaction operations even after registering models with `RegisterModels()`.

Key improvements:
- Automatically resolves table names for models used within transactions
- Ensures proper table association even with custom TableName() methods
- Handles all transaction operations (Create, Update, Delete, Query)
- Works transparently with existing transaction test API

**Example usage:**

```go
func TestUserService_CreateUserInTransaction(t *testing.T) {
    // Create test context
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Register models
    testCtx.RegisterModel(&models.User{})
    
    // Set up transaction expectations
    testCtx.ForTable("users").
        ExpectCreate().
        WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), nil, "johndoe", "john@example.com", "active").
        ReturnID(1)
    
    // Use the transaction helper with a registered model
    err := testCtx.Transaction().ExecuteInTransaction(func(tx *gorm.DB) error {
        // Create a new user within transaction
        user := &models.User{
            Username: "johndoe",
            Email:    "john@example.com",
            Status:   "active",
        }
        
        // This would fail without the transaction wrapper
        // Now it correctly resolves the table name from the registered model
        result := tx.Create(user)
        if result.Error != nil {
            return result.Error
        }
        
        return nil
    })
    
    // Assertions
    assert.NoError(t, err)
    
    // Verify all expectations were met
    testCtx.VerifyExpectations()
}
```

The implementation adds a transaction wrapper that automatically ensures proper table resolution via GORM callbacks. This solution is completely transparent to your test code and service implementations.

### Specialized Handlers for Complex GORM Queries

The library now provides specialized handlers for complex GORM query patterns that can be challenging to test:

- **HandleStandardFirstRows()**: Precise pattern matching for standard GORM First() queries
  ```go
  // This will correctly match the SQL GORM generates for First() with a simple condition
  tc.ForTable("users").
      ExpectFind().
      Where("username = ?", "johndoe").
      HandleStandardFirstRows(userRows)
  ```

- **HandleDeletedNotNullRows()**: Special handling for queries that explicitly retrieve soft-deleted records
  ```go
  // This will match GORM's Unscoped() queries that look for deleted records
  tc.ForTable("users").
      ExpectFind().
      Where("username = ? AND deleted_at IS NOT NULL", "deleted_user").
      HandleDeletedNotNullRows(deletedUserRows)
  ```

- **HandleDeletedAtRows()**: For precise handling of GORM's complex deleted_at conditions
  ```go
  // This handles the special case of explicit deleted_at IS NULL conditions
  tc.ForTable("users").
      ExpectFind().
      Where("deleted_at IS NULL").
      WithDeletedAt().
      HandleDeletedAtRows(userRows)
  ```

- **Automatic First() Detection**: The library now better detects GORM's First() patterns even without an explicit First() call
- **Complex WHERE Clause Support**: Better handling of parenthesized expressions and multiple conditions
- **Improved Soft Delete Handling**: Better detection and handling of GORM's soft delete conditions

### GORM Model Registration Support

The library now supports registering GORM models for testing, which allows you to use GORM's model-based API in your service code while still being able to use the test framework's high-level expectation builders:

- **Model Registration API**: Register models with `RegisterModel(&models.User{})` or `SetupModels([]interface{}{&models.User{}, &models.Post{}})`
- **Table Name Extraction**: Table names are automatically extracted from models using GORM's conventions 
- **TableName() Support**: Models with custom TableName() methods are properly handled
- **Soft Delete Detection**: Models with GORM's soft delete (DeletedAt field) are automatically detected and handled
- **Flexible SQL Pattern Matching**: Tests will correctly match GORM's query variations without being brittle
- **Table Name Resolution**: The test context will properly recognize model references in GORM calls like `db.Model(&models.User{}).Count(&count)`
- **Compatibility with ForTable()**: After registering models, you can still use `testCtx.ForTable("users").ExpectCount(5)`

**Example Usage:**

```go
func TestUserService_CountActiveUsers(t *testing.T) {
    // Create the test context
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Register a model - this allows test framework to recognize model references
    testCtx.RegisterModel(&models.User{})
    
    // Set up the expectation - table name "users" will be resolved automatically
    testCtx.ForTable("users").
        ExpectCount().
        Where("status = ?", "active").
        ReturnCount(10)
    
    // Create the service with the mocked DB
    service := NewUserService(testCtx.DB())
    
    // Service can use model-based GORM API
    // This would fail without model registration!
    count, err := service.CountActiveUsers() // Uses db.Model(&models.User{}).Where(...).Count(&count)
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, int64(10), count)
    
    // Verify all expectations were met
    testCtx.VerifyExpectations()
}
```

With model registration, you can:
- Write tests with the high-level, expressive ExpectCount and ExpectFind APIs
- Support service code that uses the `db.Model(&someModel)` pattern
- Avoid brittle regex-based SQL pattern matching
- Have proper table prefix support with models

**Helper Methods Added:**
- `RegisterModel(model interface{})` - Register a single model 
- `RegisterModels(models ...interface{})` - Register multiple models
- `SetupModels(models []interface{})` - Register a slice of models
- `TableNameForModel(model interface{})` - Get the table name for a model
- `PrefixTableName(table string)` - Apply the configured table prefix to a table name
- `GetRegisteredTableNames()` - Get a list of all registered table names

### Enhanced Count Query Support

The library now provides a more flexible interface for setting up count expectations with improved ORM compatibility:

- **Basic count expectations** work the same as before: `testCtx.ForTable("users").ExpectCount(5)`
- **Count with WHERE conditions**: `testCtx.ForTable("users").ExpectCount().Where("status = ?", "active").ReturnCount(3)`
- **Error handling for count queries**: `testCtx.ForTable("users").ExpectCount().ReturnError(fmt.Errorf("database error"))`
- **Combined conditions and errors**: `testCtx.ForTable("users").ExpectCount().Where("region = ?", "unknown").ReturnError(errors.New("not found"))`
- **ORM compatibility by default**: All count queries now return data in the format expected by most ORMs (with a `count(*)` column name)
- **Backward compatibility option**: For older code, use `WithColumnName("count")` to retain the old column name format

**Key improvements:**
- By default, count queries now use the column name `count(*)` that most ORMs expect, eliminating scan errors
- Fixed the scan errors when using Model().Count() query patterns
- Added options to specify custom column names for specialized use cases
- Comprehensive test coverage for both compatibility modes

**Example usage in a test:**

```go
func TestUserService_CountActiveUsers(t *testing.T) {
    // Create the test context
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Set up the expectation with a WHERE condition
    // Works with both Model().Count() and Table().Count() patterns
    testCtx.ForTable("users").
        ExpectCount().
        Where("status = ?", "active").
        ReturnCount(10)
    
    // Create the service with the mocked DB
    service := NewUserService(testCtx.DB())
    
    // Call the method that executes a count query
    count, err := service.CountActiveUsers()
    
    // Assert the results match the expectation
    assert.NoError(t, err)
    assert.Equal(t, int64(10), count)
    
    // Verify all expectations were met
    testCtx.VerifyExpectations()
}

// For backward compatibility with older code
func TestLegacyCode_CountActiveUsers(t *testing.T) {
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Use WithColumnName option to specify "count" instead of "count(*)"
    testCtx.ForTable("users").
        ExpectCount(WithColumnName("count")).
        Where("status = ?", "active").
        ReturnCount(10)
    
    // Service that uses older SQL driver or custom query that expects "count" column
    service := NewLegacyService(testCtx.DB())
    count, err := service.CountActiveUsers()
    
    assert.NoError(t, err)
    assert.Equal(t, int64(10), count)
    testCtx.VerifyExpectations()
}

func TestUserService_CountActiveUsers_Error(t *testing.T) {
    // Create the test context
    testCtx := testutil.NewDBTestContext(t)
    defer testCtx.Teardown()
    
    // Set up the expectation with an error
    expectedErr := errors.New("database connection error")
    testCtx.ForTable("users").
        ExpectCount().
        Where("status = ?", "active").
        ReturnError(expectedErr)
    
    // Create the service with the mocked DB
    service := NewUserService(testCtx.DB())
    
    // Call the method that executes a count query
    count, err := service.CountActiveUsers()
    
    // Assert the error was returned
    assert.Error(t, err)
    assert.Equal(t, expectedErr, err)
    assert.Equal(t, int64(0), count)
    
    // Verify all expectations were met
    testCtx.VerifyExpectations()
}
```

### Improved SQLite Version Query Handling

The library now handles SQLite version queries more robustly, preventing false warnings about unmet expectations in test output. This enhancement:

- Automatically handles any format of SQLite version query
- Suppresses warnings for unmet version query expectations
- Supports case-insensitive matching for version queries

You don't need to do anything differently - your tests will now run with fewer spurious warnings.

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

// 6. Count with WHERE condition
testCtx.ForTable("items")
    .ExpectCount()
    .Where("status = ?", "active")
    .ReturnCount(3)

// 7. Count with WHERE condition and error
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

Enhanced transaction testing capabilities:

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
    
    // Set up expectation for find query
    testCtx.ForTable("users")
        .ExpectFind()
        .Where("(username LIKE ? OR email LIKE ?) AND status = ?")
        .ReturnRows(userRows)
    
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
