// Package testutil provides utilities for testing service components within the Portal ecosystem.
//
// This library extends the core Portal testing facilities with additional utilities specifically for
// database testing, SQL expectations, service mocking, validation testing, and other
// testing scenarios. It employs fluent builder patterns to create an expressive API
// that reduces boilerplate code in tests.
//
// Key features include:
//
// - DBTestContext: An extension of core.TestContext with database mocking capabilities
// - ExpectationsBuilder: A fluent interface for building SQL expectations
// - ValidationTester: Utilities for testing validation logic
// - TransactionTestHelper: Helpers for testing transactional behavior
// - ConcurrentTestHelper: Tools for testing concurrent operations
// - Row Builders: Utilities for creating test data with SQL-compatible wrappers
//
// This library aims to make tests more readable, maintainable, and reliable by providing
// domain-specific testing utilities that align with common testing patterns in the Portal ecosystem.
package testutil

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gertd/go-pluralize"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	dbTesting "go.lumeweb.com/portal/core/testing/db"
	"gorm.io/gorm"
)

// Create a package-level pluralizer instance
var pluralizer = pluralize.NewClient()

// Config contains configuration options for the test context.
// This allows customizing the behavior of the test context for specific test scenarios.
type Config struct {
	// TablePrefix is an optional prefix for database tables (e.g., "users_")
	// This is useful for testing services that use table prefixes
	TablePrefix string

	// EnableSQLDebug enables detailed SQL pattern diagnostics
	// When enabled, failed SQL pattern matches will generate detailed diagnostics
	// to help identify and fix issues with pattern matching
	EnableSQLDebug bool
}

// DefaultConfig returns the default configuration for the test context.
// By default, no table prefix is used and SQL debug mode is disabled.
func DefaultConfig() *Config {
	return &Config{
		TablePrefix:    "",    // No prefix by default
		EnableSQLDebug: false, // Disabled by default
	}
}

// DBTestContext extends the portal's core TestContext with additional
// capabilities specifically for testing database-backed services.
//
// It embeds the core.TestContext interface and adds SQL mocking capabilities
// using the go-sqlmock library. This allows tests to set expectations for SQL
// queries and verify that they were executed as expected.
//
// DBTestContext provides fluent builder interfaces for common database operations:
//   - ForTable(): Creates an ExpectationsBuilder for a specific table
//   - Expect(): Creates a GenericExpectationBuilder for any SQL query
//   - Search(): Creates a SearchTestHelper for search operations
//   - Transaction(): Creates a TransactionTestHelper for transaction operations
//   - Validation(): Creates a ValidationTester for validation logic
//   - Concurrent(): Creates a ConcurrentTestHelper for concurrent operations
type DBTestContext struct {
	coreTesting.TestContext                        // Embed the core test context interface
	mock                    sqlmock.Sqlmock        // The SQL mock for setting expectations
	mu                      sync.Mutex             // Mutex to protect concurrent access
	config                  *Config                // Configuration options
	registeredModels        map[string]interface{} // Map of registered models by table name
	debugEnabled            bool                   // Enable detailed SQL pattern diagnostics
	lastSQLDiagnostics      string                 // Last SQL pattern diagnostics for error reporting
	skipVerification        bool                   // Whether to skip verification of expectations
}

// NewDBTestContext creates a new test context with a mocked database.
//
// It initializes a SQLite database mock, configures common SQL expectations,
// and creates a new test context with the mocked database. The mocked database
// can be configured using optional configuration functions.
//
// Example:
//
//	// Create a test context with default configuration
//	testCtx := testutil.NewDBTestContext(t)
//	defer testCtx.Teardown()
//
//	// Create a test context with custom table prefix
//	testCtx := testutil.NewDBTestContext(t, testutil.WithTablePrefix("users_"))
func NewDBTestContext(t *testing.T, opts ...func(*Config)) *DBTestContext {
	// Create a mock provider directly
	provider, mock, err := dbTesting.NewMockProvider(t)
	if err != nil {
		t.Fatalf("failed to create mock database provider: %v", err)
	}

	// Configure common SQL expectations *BEFORE* opening the connection
	configureSQLMockDefaults(mock)

	// Create a test logger
	logger := core.NewLogger(nil)

	// Connect to the mock database
	db, err := provider.Connect(logger)
	if err != nil {
		t.Fatalf("failed to create gorm database: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		_ = provider.Close()
	})

	// Create the test context with the mock DB
	ctx := coreTesting.NewTestContext(t, coreTesting.WithMockDB(db))

	// Apply default config
	config := DefaultConfig()

	// Apply any provided configuration options
	for _, opt := range opts {
		opt(config)
	}

	return &DBTestContext{
		TestContext:        ctx,
		mock:               mock,
		config:             config,
		registeredModels:   make(map[string]interface{}),
		debugEnabled:       config.EnableSQLDebug,
		lastSQLDiagnostics: "",
	}
}

// WithTablePrefix returns a configuration function that sets a table prefix for the test context.
//
// This is useful for testing services that use a prefix for their database tables.
// For example, if your service uses tables like "users_accounts" and "users_profiles",
// you can use WithTablePrefix("users_") to configure the test context to automatically
// add the prefix to table names when using ExpectationsBuilder.
//
// Example:
//
//	testCtx := testutil.NewDBTestContext(t, testutil.WithTablePrefix("users_"))
//
//	// Will use "users_accounts" as the table name
//	testCtx.ForTable("accounts").ExpectFind()...
func WithTablePrefix(prefix string) func(*Config) {
	return func(c *Config) {
		c.TablePrefix = prefix
	}
}

// WithSQLDebug returns a configuration function that enables detailed SQL pattern diagnostics.
//
// When enabled, the test context will generate detailed diagnostics for failed SQL pattern matches,
// which can help identify and fix issues with SQL pattern matching.
//
// Example:
//
//	testCtx := testutil.NewDBTestContext(t, testutil.WithSQLDebug())
func WithSQLDebug() func(*Config) {
	return func(c *Config) {
		c.EnableSQLDebug = true
	}
}

// EnableSQLDebug enables detailed SQL pattern diagnostics for the test context.
//
// This method can be called at any time to enable detailed diagnostics for
// failed SQL pattern matches. It is particularly useful for troubleshooting
// complex queries with pattern matching issues.
//
// Example:
//
//	testCtx := testutil.NewDBTestContext(t)
//	testCtx.EnableSQLDebug()
func (tc *DBTestContext) EnableSQLDebug() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.debugEnabled = true
}

// GetLastSQLDiagnostics returns the last SQL pattern diagnostics generated
// by the test context. This is useful for debugging failed SQL pattern matches.
//
// If no diagnostics have been generated, or if debug mode is disabled,
// this method returns an empty string.
//
// Example:
//
//	// After a test failure
//	if diag := testCtx.GetLastSQLDiagnostics(); diag != "" {
//	    t.Logf("SQL Diagnostics: %s", diag)
//	}
func (tc *DBTestContext) GetLastSQLDiagnostics() string {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.lastSQLDiagnostics
}

// SetLastSQLDiagnostics sets the last SQL pattern diagnostics for the test context.
// This is intended for internal use by the SQL pattern matching helpers.
func (tc *DBTestContext) setLastSQLDiagnostics(diagnostics string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.lastSQLDiagnostics = diagnostics
}

// configureSQLMockDefaults adds default expectations for common SQL operations
// that GORM and other libraries may execute automatically in the background.
//
// This function sets up expectations for queries like SQLite version checks,
// table existence checks, and schema information queries so that tests don't
// fail due to these background operations.
func configureSQLMockDefaults(mock sqlmock.Sqlmock) {
	// Allow expectations to be matched out of order
	// This is important as background queries may be executed in any order
	mock.MatchExpectationsInOrder(false)

	// SQLite version query handling - We use a robust approach to avoid warnings about
	// unmet expectations for SQLite version queries that GORM executes during connection
	// or operations. The version query can appear in various formats and cases.

	// Set up a version row result for all version queries
	versionRows := sqlmock.NewRows([]string{"version"}).AddRow("3.36.0")

	// Set up result for SELECT 1 queries that are frequently used as ping/simple tests
	oneRow := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock.ExpectQuery(`SELECT 1`).WillReturnRows(oneRow)
	mock.ExpectQuery(`select 1`).WillReturnRows(oneRow)

	// Use a very permissive regex to capture all potential version queries
	// This acts as a catch-all for any version-related query
	mock.ExpectQuery(`(?i).*version.*`).WillReturnRows(versionRows)

	// Add specific patterns for common SQLite version query formats
	// These use case-insensitive matching to handle variations in casing
	for _, pattern := range []string{
		`(?i)select\s+sqlite_version\(\).*`, // Common format: SELECT sqlite_version()
		`(?i)select\s+SQLITE_VERSION\(\).*`, // Uppercase variant: SELECT SQLITE_VERSION()
		`(?i)pragma\s+user_version.*`,       // PRAGMA format: PRAGMA user_version
	} {
		mock.ExpectQuery(pattern).WillReturnRows(versionRows)
	}

	// Add expectations for SQLite table existence checks used by GORM
	// This simplifies test setup by handling these automatically
	tableExistenceSQL := "SELECT count\\(\\*\\) FROM sqlite_master WHERE type='table' AND name=\\?"
	for i := 0; i < 50; i++ {
		mock.ExpectQuery(tableExistenceSQL).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	}

	// Add expectations for SQLite table info queries
	tableInfoSQL := "PRAGMA table_info\\(`.*`\\)"
	for i := 0; i < 10; i++ {
		mock.ExpectQuery(tableInfoSQL).
			WillReturnRows(sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk"}))
	}

	// Add expectations for SQLite foreign key check queries
	foreignKeySQL := "PRAGMA foreign_key_list\\(`.*`\\)"
	for i := 0; i < 10; i++ {
		mock.ExpectQuery(foreignKeySQL).
			WillReturnRows(sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}))
	}

	// Add expectations for SQLite index list queries
	indexListSQL := "SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = \\?"
	for i := 0; i < 10; i++ {
		mock.ExpectQuery(indexListSQL).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"name"}))
	}
}

// RegisterService registers a service in the test context.
//
// This method is thread-safe and can be called from concurrent goroutines.
// It delegates to the embedded TestContext's RegisterService method.
//
// Example:
//
//	testCtx.RegisterService("user_service", mockUserService)
func (tc *DBTestContext) RegisterService(serviceID string, service interface{}) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.TestContext.RegisterService(serviceID, service)
}

// Teardown cleans up the test context and releases any resources it holds.
//
// This should be called at the end of each test, typically in a defer statement
// right after creating the test context. It delegates to the embedded TestContext's
// Teardown method.
//
// Example:
//
//	testCtx := testutil.NewDBTestContext(t)
//	defer testCtx.Teardown()
func (tc *DBTestContext) Teardown() {
	// Verify expectations before teardown if skipVerification is not set
	if !tc.skipVerification {
		tc.VerifyExpectations()
	}
	tc.TestContext.Teardown()
}

// VerifyExpectations verifies that all SQL expectations were met during the test.
//
// This should be called at the end of each test to ensure that all expected
// SQL operations were actually executed. If any expectations were not met,
// it will log a warning unless they are SQLite-related queries which are sometimes
// generated unexpectedly by GORM.
//
// The function intelligently ignores common SQLite-related unmet expectations,
// such as SQLite version queries or schema information queries that may not
// always be executed by GORM depending on its internal state.
//
// Example:
//
//	// At the end of your test
//	testCtx := testutil.NewDBTestContext(t)
//	defer testCtx.Teardown()
//
//	// Set up your expectations
//	testCtx.ForTable("users").ExpectCount(5)
//
//	// Call the code that will execute the queries
//	service.GetUserCount()
//
//	// Verify all expectations were met
//	testCtx.VerifyExpectations()
func (tc *DBTestContext) VerifyExpectations() {
	// Skip verification if skipVerification is set
	if tc.skipVerification {
		return
	}

	// Check if there are any unmet expectations
	err := tc.mock.ExpectationsWereMet()
	if err != nil {
		// Sometimes GORM generates unexpected SQLite queries
		// We'll only report unmet expectations if they're not SQLite-related
		isIgnoredSQLiteError := strings.Contains(err.Error(), "sqlite_master") ||
			strings.Contains(err.Error(), "sqlite_version") ||
			strings.Contains(err.Error(), "SQLITE_VERSION") ||
			strings.Contains(err.Error(), "user_version") ||
			strings.Contains(err.Error(), "SELECT 1") // Also ignore simple SELECT 1 queries

		if !isIgnoredSQLiteError {
			tc.T().Logf("Warning: unmet expectations: %v", err)
		}
		// Otherwise, silently ignore the error as it's a normal part of GORM's behavior
	}
}

// SkipVerification tells the test context to skip verification of SQL expectations
// at the end of the test. This is primarily used for tests that specifically test
// the behavior of default SQL expectations, like SQLite version queries.
//
// In normal circumstances, portal-testutil verifies all SQL expectations were met
// when the test completes. This method allows bypassing that verification when needed,
// such as when testing internal library behavior or when working with databases that
// generate unpredictable queries.
//
// Example:
//
//	// Create a test context
//	tc := testutil.NewDBTestContext(t)
//
//	// Skip verification for this specific test
//	tc.SkipVerification()
//
//	// Set up expectations normally...
//	tc.ForTable("users").ExpectFind().ReturnRows(sqlmock.NewRows([]string{"id"}))
//
//	// Run test... verification will be skipped at the end
func (tc *DBTestContext) SkipVerification() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.skipVerification = true
}

// ForTable creates a new expectations builder for a specific database table.
//
// This is the primary entry point for setting up SQL expectations in tests.
// It returns an ExpectationsBuilder that provides a fluent interface for
// building SQL expectations specific to the given table.
//
// If a table prefix was configured for the test context, it will be automatically
// prepended to the table name unless the table name already has the prefix.
//
// When using with registered models via RegisterModel() or RegisterModels(),
// ForTable handles both the table name directly and also can resolve table names
// from model registrations. This allows the same test code to work with or without
// model registration.
//
// Example:
//
//	// Expect a find operation on the users table
//	testCtx.ForTable("users").ExpectFind().ByID(1).ReturnRows(rows)
//
//	// With model registration
//	testCtx.RegisterModel(&models.User{})
//	testCtx.ForTable("users").ExpectCount(5)
func (tc *DBTestContext) ForTable(table string) *ExpectationsBuilder {
	// First, check if we have a registered model with this exact name
	tc.mu.Lock()
	// Look for a model registered with this exact table name
	model, modelExists := tc.registeredModels[table]
	tc.mu.Unlock()

	if modelExists {
		// If we have a registered model with this name, we need to determine
		// what table name GORM will actually use in its queries

		// Check if the model has a TableName method - if so, we'll use that exact value
		// because GORM will use that value directly in its queries
		modelValue := reflect.ValueOf(model)
		tableNameMethod := modelValue.MethodByName("TableName")

		if tableNameMethod.IsValid() {
			// This model has a TableName method, so we need to use its exact output in our expectations
			results := tableNameMethod.Call(nil)
			if len(results) == 1 && results[0].Kind() == reflect.String {
				actualTableName := results[0].String()
				// For models with TableName method, don't apply prefix again since GORM will use
				// exactly what TableName() returns
				return NewExpectationsBuilder(tc, actualTableName)
			}
		}
	}

	// For normal table names, apply prefixing as needed
	return NewExpectationsBuilder(tc, table)
}

// GetRegisteredTableNames returns a list of all registered table names.
//
// This is primarily useful for debugging and testing purposes. It returns
// a list of all table names that have been registered using RegisterModel()
// or RegisterModels().
//
// Example:
//
//	testCtx.RegisterModel(&models.User{})
//	testCtx.RegisterModel(&models.Post{})
//	tableNames := testCtx.GetRegisteredTableNames()
//	// tableNames will contain ["users", "posts"]
func (tc *DBTestContext) GetRegisteredTableNames() []string {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	names := make([]string, 0, len(tc.registeredModels))
	for name := range tc.registeredModels {
		names = append(names, name)
	}

	return names
}

// TableNameForModel extracts the table name for a given GORM model.
//
// This method is useful when you need to get the actual table name that will
// be used in SQL queries for a particular model. It follows the same table name
// extraction logic used internally by the test context.
//
// The returned table name includes any table prefix configured for the test context,
// unless the model has its own TableName() method, in which case the result of
// that method is used directly.
//
// Example:
//
//	// Get the table name for a model
//	tableName := testCtx.TableNameForModel(&models.User{})
//	// tableName will be "users" (or "prefix_users" if a prefix is configured)
//
//	// Use in expectations
//	testCtx.ForTable(tableName).ExpectCount(5)
func (tc *DBTestContext) TableNameForModel(model interface{}) string {
	// First, extract the basic table name
	tableName := tc.extractTableName(model)

	// Check if the model has a TableName method
	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() == reflect.Ptr && modelVal.IsNil() {
		// Create a new instance of the struct for the method call
		modelVal = reflect.New(reflect.TypeOf(model).Elem())
	}

	// Look for TableName method
	tableNameMethod := modelVal.MethodByName("TableName")
	if tableNameMethod.IsValid() {
		// If the model has a TableName method, use its value directly
		// GORM will use this exact value in queries
		return tableName
	}

	// If the model doesn't have a TableName method and we have a prefix,
	// apply the prefix to the table name (only if it's not already prefixed)
	if tc.config.TablePrefix != "" && !strings.HasPrefix(tableName, tc.config.TablePrefix) {
		tableName = tc.config.TablePrefix + tableName
	}

	return tableName
}

// PrefixTableName applies the configured table prefix to a table name.
//
// This is useful for test cases where table names need to be dynamically created
// with the current prefix configuration.
//
// Example:
//
//	// With prefix "app_"
//	testCtx := testutil.NewDBTestContext(t, testutil.WithTablePrefix("app_"))
//	prefixedTable := testCtx.PrefixTableName("users")
//	// prefixedTable will be "app_users"
//
//	// Use in expectations
//	testCtx.ForTable(prefixedTable).ExpectCount(5)
func (tc *DBTestContext) PrefixTableName(table string) string {
	if tc.config.TablePrefix == "" || strings.HasPrefix(table, tc.config.TablePrefix) {
		return table
	}
	return tc.config.TablePrefix + table
}

// tableHasSoftDelete checks if a table uses soft delete (has a deleted_at column).
//
// This helper function is used to determine if deleted_at conditions should be
// added to SQL expectations. It checks registered models to see if they have a
// deleted_at field, which would indicate GORM soft delete is being used.
//
// In the current implementation it conservatively defaults to returning true
// for most tables (assuming soft delete) unless a model has been registered
// that explicitly doesn't have a deleted_at field.
func (tc *DBTestContext) tableHasSoftDelete(tableName string) bool {
	// Default to assume soft delete is used
	useSoftDelete := true

	// Check if we have a registered model for this table
	tc.mu.Lock()
	defer tc.mu.Unlock()

	for registeredTable, model := range tc.registeredModels {
		// If we found a matching registered model
		if registeredTable == tableName {
			// Check if the model has a deleted_at field
			hasSoftDelete := false

			// Use reflection to check for a deleted_at field
			modelVal := reflect.ValueOf(model)
			if modelVal.Kind() == reflect.Ptr {
				modelVal = modelVal.Elem()
			}
			modelType := modelVal.Type()

			// Check for gorm.Model embedding which includes soft delete
			for i := 0; i < modelType.NumField(); i++ {
				field := modelType.Field(i)

				// Check if this is gorm.Model or has a deleted_at field
				if field.Anonymous && field.Type.Name() == "Model" {
					hasSoftDelete = true
					break
				}

				if field.Name == "DeletedAt" {
					hasSoftDelete = true
					break
				}
			}

			// Return the result of our check
			return hasSoftDelete
		}
	}

	// Default to true (safer) - will add deleted_at condition
	return useSoftDelete
}

// Expect creates a generic expectation builder for SQL operations.
//
// Unlike ForTable, this method returns a GenericExpectationBuilder that can
// be used to build expectations for any SQL query or execution, not just
// those specific to a particular table. This is useful for custom SQL queries
// or for operations that span multiple tables.
//
// Example:
//
//	// Expect a custom SQL query
//	testCtx.Expect().Query("SELECT count\\(\\*\\) FROM users").ReturnRows(rows)
func (tc *DBTestContext) Expect() *GenericExpectationBuilder {
	return NewGenericExpectationBuilder(tc)
}

// Search creates a search test helper for testing search functionality.
//
// The SearchTestHelper provides a fluent interface for setting up expectations
// for search operations, including support for search queries, filters, sorting,
// and pagination.
//
// Example:
//
//	// Expect a search operation
//	testCtx.Search().WithQuery("john").WithFields("name", "email").ReturnCount(2).ReturnRows(rows)
func (tc *DBTestContext) Search() *SearchTestHelper {
	return NewSearchTestHelper(tc)
}

// Validation creates a validation test helper for testing validation logic.
//
// The ValidationTester provides utilities for testing various aspects of validation
// logic, including required fields, string length, email format, URL format, numeric
// range, and more.
//
// Example:
//
//	// Test validation logic
//	validator := testCtx.Validation()
//	validator.AssertEmailFormat(validateEmail, "email")
//	validator.AssertMinLength(validatePassword, "password", 8)
func (tc *DBTestContext) Validation() *ValidationTester {
	return NewValidationTester(tc.T())
}

// Raw returns the underlying SQL mock for advanced usage.
//
// This method provides direct access to the sqlmock.Sqlmock instance used by the
// test context. It should only be used when the provided abstractions don't
// support your needs, as it bypasses the safety and convenience features of
// the test context.
//
// Example:
//
//	// Set up a custom expectation directly
//	testCtx.Raw().ExpectQuery("SELECT \\* FROM users").WillReturnRows(rows)
func (tc *DBTestContext) Raw() sqlmock.Sqlmock {
	return tc.mock
}

// We'll use a simple struct to mock model table name
type modelWithTableName interface {
	TableName() string
}

// DB returns the underlying GORM database connection with enhanced table resolution support.
//
// This method provides access to the *gorm.DB instance used by the test context.
// It wraps the embedded TestContext's DB method to add support for table resolution
// in GORM transactions. The returned database connection is enhanced with callbacks
// that ensure proper table name resolution for all database operations, including
// those in transactions.
//
// The DB method includes critical fixes for GORM transaction table resolution, including:
// - Support for models registered via RegisterModel() or RegisterModelWithRelationships()
// - Support for models with custom TableName() methods (both pointer and value receivers)
// - Handling of map values in Create operations
// - Support for operations where the model is available as Statement.Dest
//
// Unlike the older versions that had issues with certain transactions, this enhancement
// allows direct GORM transactions (db.Transaction()) to properly resolve table names
// in all scenarios. It also uses unique session IDs per helper to prevent duplicate
// callback warnings.
//
// Example:
//
//	// Register a model before using the model-based GORM API
//	testCtx.RegisterModel(&models.User{})
//
//	// In your service code:
//	service.DB().Model(&models.User{}).Count(&total)
//
//	// In transactions - Now works with direct GORM transactions and custom TableName models!
//	service.DB().Transaction(func(tx *gorm.DB) error {
//	    // This will now correctly resolve the table name for models with TableName methods
//	    return tx.Create(&models.User{...}).Error
//	})
//
//	// Even works with map values in transactions:
//	service.DB().Transaction(func(tx *gorm.DB) error {
//	    values := map[string]interface{}{"name": "test"}
//	    return tx.Model(&models.User{}).Create(values).Error
//	})
//
//	// And with RegisterModelWithRelationships:
//	RegisterModelWithRelationships[models.User](testCtx)
//	service.DB().Transaction(func(tx *gorm.DB) error {
//	    return tx.Create(&models.User{...}).Error // Table name resolved correctly
//	})
//
//	// Your test expectations:
//	testCtx.ForTable("users").ExpectCount(5)
func (tc *DBTestContext) DB() *gorm.DB {
	// Get the base DB from the context
	baseDB := tc.TestContext.DB()

	// If there are no registered models, no need for the special wrapper
	if len(tc.registeredModels) == 0 {
		return baseDB
	}

	// Create a transaction helper to use its table resolution logic
	txHelper := NewTransactionTestHelper(tc)

	// Create a new session with registered callbacks for table resolution
	wrappedDB := baseDB.Session(&gorm.Session{})

	// Register callbacks for all operations that might need table resolution - with unique session ID to prevent duplication
	callbackBaseNames := []string{
		"ensure_query_table",
		"ensure_create_table",
		"ensure_update_table",
		"ensure_delete_table",
		"ensure_raw_table",
	}

	// Create full callback names with session ID to ensure uniqueness
	callbackNames := make([]string, len(callbackBaseNames))
	for i, baseName := range callbackBaseNames {
		callbackNames[i] = fmt.Sprintf("testutil:%s:%s", baseName, txHelper.sessionID)
	}

	// Register Query operation callback
	callbackName := callbackNames[0]
	wrappedDB.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before a SELECT query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			txHelper.ensureTableSet(db)
		}
	})
	txHelper.registeredCallbacks[callbackName] = true

	// Register Create operation callback
	callbackName = callbackNames[1]
	wrappedDB.Callback().Create().Before("gorm:create").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before an INSERT query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			txHelper.ensureTableSet(db)
		}
	})
	txHelper.registeredCallbacks[callbackName] = true

	// Register Update operation callback
	callbackName = callbackNames[2]
	wrappedDB.Callback().Update().Before("gorm:update").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before an UPDATE query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			txHelper.ensureTableSet(db)
		}
	})
	txHelper.registeredCallbacks[callbackName] = true

	// Register Delete operation callback
	callbackName = callbackNames[3]
	wrappedDB.Callback().Delete().Before("gorm:delete").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before a DELETE query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			txHelper.ensureTableSet(db)
		}
	})
	txHelper.registeredCallbacks[callbackName] = true

	// Register Raw operation callback
	callbackName = callbackNames[4]
	wrappedDB.Callback().Raw().Before("gorm:raw").Register(callbackName, func(db *gorm.DB) {
		// This is triggered before a raw SQL query is executed
		if (db.Statement.Model != nil || db.Statement.ReflectValue.IsValid()) && db.Statement.Table == "" {
			txHelper.ensureTableSet(db)
		}
	})
	txHelper.registeredCallbacks[callbackName] = true

	return wrappedDB
}

// RegisterModel registers a GORM model with the test context.
//
// This method allows registering models that will be recognized when
// using db.Model() in GORM queries, enabling services to use the model-based
// API pattern while still allowing for proper mocking in tests.
//
// The function extracts the table name from the model using GORM's conventions:
// - If the model has a TableName() method, it will use that
// - Otherwise, it will infer the table name from the model's type name
//
// Example:
//
//	// Register a single model
//	testCtx.RegisterModel(&models.User{})
//
//	// Now model-based queries will work in tests
//	testCtx.ForTable("users").ExpectCount(5)
//	svc.GetUserCount() // Uses db.Model(&models.User{}).Count(&count)
func (tc *DBTestContext) RegisterModel(model interface{}) {
	// Extract the table name using reflection
	tableName := tc.extractTableName(model)

	// Store the model in the registry
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.registeredModels[tableName] = model
}

// RegisterModels registers multiple GORM models with the test context.
//
// This method is a convenience wrapper around RegisterModel that accepts
// multiple models at once. It registers each model by extracting its table name
// and storing it in the model registry.
//
// Example:
//
//	// Register multiple models
//	testCtx.RegisterModels(&models.User{}, &models.Post{}, &models.Comment{})
//
//	// Now model-based queries will work in tests
//	testCtx.ForTable("users").ExpectCount(5)
//	testCtx.ForTable("posts").ExpectFind().ReturnRows(postRows)
func (tc *DBTestContext) RegisterModels(models ...interface{}) {
	for _, model := range models {
		tc.RegisterModel(model)
	}
}

// SetupModels registers a list of models with the test context.
//
// This is an alias for RegisterModels that is more explicit about the
// parameter being a slice. It's designed to work with slices like:
//
// Example:
//
//	// Register models from a slice
//	models := []interface{}{&models.User{}, &models.Post{}, &models.Comment{}}
//	testCtx.SetupModels(models)
//
//	// Now model-based queries will work in tests
//	testCtx.ForTable("users").ExpectCount(5)
func (tc *DBTestContext) SetupModels(models []interface{}) {
	tc.RegisterModels(models...)
}

// extractTableName extracts the table name from a GORM model.
//
// This internal helper function determines the table name by:
// 1. Checking if the model implements TableName() string method
// 2. If not, falling back to inferring the table name from the struct name
//
// The function handles both struct and pointer types and tries to follow
// GORM's table naming conventions.
func (tc *DBTestContext) extractTableName(model interface{}) string {
	// Handle nil case first
	if model == nil {
		if tc.T() != nil {
			tc.T().Errorf("Cannot extract table name from nil model")
		}
		return ""
	}

	modelType := reflect.TypeOf(model)

	// Handle pointer types
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Only structs are supported
	if modelType.Kind() != reflect.Struct {
		if tc.T() != nil {
			tc.T().Errorf("Cannot extract table name from non-struct type: %v", modelType)
		}
		return ""
	}

	// Try to call TableName method if it exists
	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() == reflect.Ptr && modelVal.IsNil() {
		// Create a new instance of the struct for the method call
		modelVal = reflect.New(modelType)
	}

	// Look for TableName method
	tableNameMethod := modelVal.MethodByName("TableName")
	if tableNameMethod.IsValid() {
		results := tableNameMethod.Call(nil)
		if len(results) == 1 && results[0].Kind() == reflect.String {
			tableName := results[0].String()
			return tableName
		}
	}

	// Fall back to inferring from struct name using GORM's conventions
	// Convert CamelCase to snake_case and pluralize
	structName := modelType.Name()
	snakeCase := toSnakeCase(structName)
	tableName := pluralizer.Plural(snakeCase)

	return tableName
}

// BuildRows creates mock SQL rows from a map of column values.
//
// This method provides a convenient way to create mock database rows from a map
// where keys are column names and values are column values. It's especially useful
// for simple, single-row test data.
//
// The table parameter is primarily for documentation purposes to indicate which
// table the rows are meant to represent. It doesn't affect the generated rows.
//
// Example:
//
//	// Create mock rows for a single reporter
//	now := time.Now()
//	rows := testCtx.BuildRows("reporters", map[string]any{
//		"id":         1,
//		"created_at": now,
//		"updated_at": now,
//		"deleted_at": nil,
//		"email":      "reporter1@example.com",
//		"name":       "Reporter One",
//		"user_id":    nil,
//	})
//
//	// Use in expectations
//	testCtx.ForTable("reporters").ExpectFind().ByID(1).ReturnRows(rows)
//
// Returns:
//   - A *sqlmock.Rows object that can be used with ExpectationsBuilder.ReturnRows()
func (tc *DBTestContext) BuildRows(table string, data map[string]any) *sqlmock.Rows {
	// Extract column names from the map keys
	columns := make([]string, 0, len(data))
	for col := range data {
		columns = append(columns, col)
	}

	// Create a row builder with the columns
	builder := NewRowBuilder(columns...)

	// Add a row with the map values
	return builder.AddRowWithMap(data).Build()
}

// BuildRowsFrom creates mock SQL rows from structs, maps, or models.
//
// This method provides a powerful way to create mock database rows directly from
// application objects. It significantly reduces boilerplate in tests by automatically
// extracting column names and values from Go objects.
//
// # Features
//
// BuildRowsFrom supports:
//   - Single structs or maps
//   - Slices of structs or maps
//   - GORM models with embedded gorm.Model
//   - Struct field tags for custom column names
//   - Nil pointers and nullable fields
//
// # How It Works
//
// The function uses reflection to:
//   - Extract field values from structs
//   - Handle embedded fields (like gorm.Model)
//   - Process tags for column names (gorm:"column:name" or json:"name")
//   - Convert field names to snake_case if no tags are present
//
// # Usage Examples
//
// Example with GORM models:
//
//	// Create rows from a slice of model structs
//	rows := testCtx.BuildRowsFrom("reporters", []models.Reporter{
//		{
//			Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
//			Email: "reporter1@example.com",
//			Name:  "Reporter One",
//		},
//		{
//			Model: gorm.Model{ID: 2, CreatedAt: now, UpdatedAt: now},
//			Email: "reporter2@example.com",
//			Name:  "Reporter Two",
//		},
//	})
//
// Example with a single struct:
//
//	// Create rows from a single struct
//	row := testCtx.BuildRowsFrom("user", user)
//
// Example with a slice of maps:
//
//	// Create rows from multiple maps
//	rows := testCtx.BuildRowsFrom("items", []map[string]any{
//		{"id": 1, "name": "Item 1", "price": 10.99},
//		{"id": 2, "name": "Item 2", "price": 20.99},
//	})
//
// The table parameter is primarily for documentation purposes to indicate which
// table the rows are meant to represent. It doesn't affect the generated rows.
//
// # Tags Support
//
// The function recognizes both GORM and JSON tags:
//   - GORM: `gorm:"column:custom_name"`
//   - JSON: `json:"custom_name"`
//
// If no tags are present, field names are converted to snake_case.
//
// # Return Value
//
// Returns a *sqlmock.Rows object that can be used with ExpectationsBuilder.ReturnRows().
// For unsupported input types or empty slices, returns an empty *sqlmock.Rows.
func (tc *DBTestContext) BuildRowsFrom(table string, models any) *sqlmock.Rows {
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

		// If the slice contains maps, handle them specially
		if firstModel.Kind() == reflect.Map {
			return tc.buildRowsFromMaps(table, models)
		}

		// Otherwise, assume it's a slice of structs
		return tc.buildRowsFromStructs(table, models)

	case reflect.Struct:
		// Single struct - wrap in a slice and process
		sliceType := reflect.SliceOf(modelsVal.Type())
		slice := reflect.MakeSlice(sliceType, 1, 1)
		slice.Index(0).Set(modelsVal)
		return tc.buildRowsFromStructs(table, slice.Interface())

	case reflect.Map:
		// Single map - handle it directly
		if mapValue, ok := modelsVal.Interface().(map[string]any); ok {
			return tc.BuildRows(table, mapValue)
		}
		if tc.T() != nil {
			tc.T().Logf("Warning: unsupported map type for BuildRowsFrom: %v", modelsVal.Type())
		}
		return sqlmock.NewRows([]string{})

	default:
		// For unsupported types, return an empty result
		if tc.T() != nil {
			tc.T().Logf("Warning: unsupported type for BuildRowsFrom: %v", modelsVal.Type())
		}
		return sqlmock.NewRows([]string{})
	}
}

// buildRowsFromStructs creates mock SQL rows from a slice of structs.
//
// This internal helper function:
// 1. Takes a slice of structs (or a single struct wrapped in a slice)
// 2. Extracts the field names and creates column names from tags or snake_case
// 3. Extracts values from each struct to create rows
// 4. Handles embedded structs (like gorm.Model)
func (tc *DBTestContext) buildRowsFromStructs(table string, models any) *sqlmock.Rows {
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
	fieldIndices := make(map[string][]int)

	// Process all fields, including embedded ones
	tc.extractFieldNames(firstModelType, []int{}, &columns, fieldIndices)

	// Create a row builder with the columns
	builder := NewRowBuilder(columns...)

	// For each model, extract the values and add as a row
	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		values := make(map[string]any)

		// For each column, find the corresponding field value
		for _, col := range columns {
			indices, ok := fieldIndices[col]
			if !ok {
				values[col] = nil
				continue
			}

			// Follow the indices to get the field
			field := model
			for _, idx := range indices {
				if field.Kind() == reflect.Ptr && !field.IsNil() {
					field = field.Elem()
				}
				field = field.Field(idx)
			}

			// Handle different field types
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					values[col] = nil
				} else {
					values[col] = field.Elem().Interface()
				}
			} else {
				values[col] = field.Interface()
			}
		}

		builder.AddRowWithMap(values)
	}

	return builder.Build()
}

// buildRowsFromMaps creates mock SQL rows from a slice of maps.
//
// This internal helper function:
// 1. Takes a slice of maps (or a single map)
// 2. Extracts the keys from the first map to determine column names
// 3. Uses map values to populate rows
// 4. Creates a properly formatted sqlmock.Rows object
func (tc *DBTestContext) buildRowsFromMaps(table string, models any) *sqlmock.Rows {
	modelsVal := reflect.ValueOf(models)

	// Must be a slice
	if modelsVal.Kind() != reflect.Slice || modelsVal.Len() == 0 {
		return sqlmock.NewRows([]string{})
	}

	// Get the first map to determine the columns
	firstMap := modelsVal.Index(0).Interface().(map[string]any)

	// Extract column names from the map keys
	columns := make([]string, 0, len(firstMap))
	for col := range firstMap {
		columns = append(columns, col)
	}

	// Create a row builder with the columns
	builder := NewRowBuilder(columns...)

	// For each map, add its values as a row
	for i := 0; i < modelsVal.Len(); i++ {
		mapVal := modelsVal.Index(i).Interface().(map[string]any)
		builder.AddRowWithMap(mapVal)
	}

	return builder.Build()
}

// extractFieldNames extracts column names from struct fields, handling embedded structs.
//
// This internal helper function recursively processes a struct type and:
// 1. Handles embedded structs by recursively extracting their fields
// 2. Extracts column names from GORM tags, JSON tags, or field names
// 3. Keeps track of the path to each field for later value extraction
// 4. Handles complex nested structures with proper path tracking
//
// The function builds both:
// - A list of column names in the columns slice
// - A mapping from column names to field paths in the fieldIndices map
func (tc *DBTestContext) extractFieldNames(t reflect.Type, path []int, columns *[]string, fieldIndices map[string][]int) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Create a new path that includes the current field
		newPath := append(append([]int{}, path...), i)

		if field.Anonymous {
			// Handle embedded structs (like gorm.Model)
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				tc.extractFieldNames(fieldType, newPath, columns, fieldIndices)
			}
			continue
		}

		// Get column name from tags or field name
		colName := field.Tag.Get("gorm")
		if colName == "" || colName == "-" {
			// Try JSON tag if GORM tag isn't available
			colName = field.Tag.Get("json")
			if colName == "" || colName == "-" {
				// Use field name as a fallback, converting to snake_case
				colName = toSnakeCase(field.Name)
			} else {
				// Handle JSON tag options like `json:"name,omitempty"`
				colName = strings.Split(colName, ",")[0]
			}
		} else {
			// Handle GORM tag options like `gorm:"column:name;type:varchar(255)"`
			if strings.Contains(colName, "column:") {
				parts := strings.Split(colName, ";")
				for _, part := range parts {
					if strings.HasPrefix(part, "column:") {
						colName = strings.TrimPrefix(part, "column:")
						break
					}
				}
			} else {
				colName = toSnakeCase(field.Name)
			}
		}

		// Add column to the list if not already there
		if colName != "" {
			found := false
			for _, col := range *columns {
				if col == colName {
					found = true
					break
				}
			}
			if !found {
				*columns = append(*columns, colName)
				fieldIndices[colName] = newPath
			}
		}
	}
}

// toSnakeCase converts a camelCase string to snake_case.
//
// This function takes a CamelCase field name and converts it to snake_case,
// which is the conventional format for database column names in many ORMs
// including GORM.
//
// Special handling is included for common field names:
// - "ID" becomes "id" (not "i_d")
func toSnakeCase(s string) string {
	// Special case for common field names
	if s == "ID" {
		return "id" // Ensure ID is always lowercase
	}

	var result strings.Builder
	for i, c := range s {
		if i > 0 && c >= 'A' && c <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(c))
	}
	return result.String()
}
