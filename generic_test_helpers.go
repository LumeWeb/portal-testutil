// Package testutil provides utilities for testing service components within the Portal ecosystem.
package testutil

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// NewTestLogger creates a configured test logger with standardized settings.
//
// This function returns a properly configured Portal core.Logger suitable for
// use in tests, with log level set to WarnLevel to reduce noise. The logger
// can be passed to any code that expects a core.Logger.
//
// Example:
//
//	logger := NewTestLogger()
//	service := &MyService{logger: logger}
func NewTestLogger() *core.Logger {
	// Create a new logger with nil config (matches how test contexts create loggers)
	logger := core.NewLogger(nil)

	// Set the log level to warn to reduce noise in tests
	logger.Level().SetLevel(zap.WarnLevel)

	return logger
}

// ServiceInitializer is an interface for services that need explicit initialization.
//
// Services can optionally implement this interface to receive test dependencies
// in a type-safe way without requiring exported fields. This is especially useful
// for production services that use unexported fields but need to be properly
// initialized in tests.
//
// Example:
//
//	type MyService struct {
//	    ctx    context.Context // unexported field
//	    db     *gorm.DB        // unexported field
//	    logger *core.Logger    // unexported field
//	}
//
//	// Implement ServiceInitializer for testing
//	func (s *MyService) InitForTest(tc *DBTestContext, db *gorm.DB, logger *core.Logger) {
//	    s.ctx = tc
//	    s.db = db
//	    s.logger = logger
//	}
type ServiceInitializer interface {
	InitForTest(tc *DBTestContext, db *gorm.DB, logger *core.Logger)
}

// CreateAndRegisterService creates and registers a service with common dependencies
// using generics for type safety and better developer experience.
//
// This function will:
// 1. Create a new instance of the service type specified by the generic parameter T
// 2. Initialize it with common dependencies (context, DB, logger)
// 3. Register it with the test context under the specified serviceID
// 4. Return the typed service instance
//
// The function supports two initialization methods:
// - If the service implements ServiceInitializer interface, it uses that
// - Otherwise, it attempts to set fields via reflection
//
// Example:
//
//	// Create and register a UserService (type-safe)
//	userSvc := CreateAndRegisterService[*UserService](tc, "user_service")
//
//	// Use the service directly with proper typing
//	user, err := userSvc.GetUser(1)
func CreateAndRegisterService[T any](tc *DBTestContext, serviceID string) T {
	// Create a new instance of the service using reflection
	serviceType := reflect.TypeOf((*T)(nil)).Elem()

	// Create instance based on whether it's a pointer or value type
	var svc T
	if serviceType.Kind() == reflect.Ptr {
		// Create a new pointer instance
		svc = reflect.New(serviceType.Elem()).Interface().(T)
	} else {
		// Create a new value instance
		svc = reflect.New(serviceType).Elem().Interface().(T)
	}

	// First try ServiceInitializer interface
	if initializer, ok := any(svc).(ServiceInitializer); ok {
		// Use the interface method if implemented
		initializer.InitForTest(tc, tc.DB(), NewTestLogger())
	} else {
		// Fall back to reflection approach, trying both naming conventions
		SetCommonServiceFields(tc, svc)
	}

	// Register the service
	tc.RegisterService(serviceID, svc)

	return svc
}

// SetCommonServiceFields sets standard fields on a service using reflection.
//
// This function attempts to set common service dependencies by looking for fields
// with common naming patterns and setting them with the appropriate values. It
// supports both exported (capitalized) and unexported (lowercase) field names.
//
// The function looks for:
// - Context fields: "ctx", "Ctx", "context", "Context"
// - Database fields: "db", "Db", "database", "Database"
// - Logger fields: "logger", "Logger", "log", "Log"
//
// Note: This can only set exported (capitalized) fields due to Go's reflection
// limitations. For unexported fields, implement the ServiceInitializer interface.
//
// Example:
//
//	service := &MyService{}
//	SetCommonServiceFields(tc, service)
func SetCommonServiceFields(tc *DBTestContext, svc any) {
	// Use reflection to discover and set fields
	val := reflect.ValueOf(svc)

	// Handle both pointer and value types
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Only proceed if we have a struct
	if val.Kind() != reflect.Struct {
		return
	}

	// Try both naming conventions for context field
	for _, ctxName := range []string{"ctx", "Ctx", "context", "Context"} {
		if ctxField := val.FieldByName(ctxName); ctxField.IsValid() && ctxField.CanSet() {
			ctxField.Set(reflect.ValueOf(tc))
			break
		}
	}

	// Try both naming conventions for database field
	for _, dbName := range []string{"db", "Db", "database", "Database"} {
		if dbField := val.FieldByName(dbName); dbField.IsValid() && dbField.CanSet() {
			dbField.Set(reflect.ValueOf(tc.DB()))
			break
		}
	}

	// Try both naming conventions for logger field
	for _, loggerName := range []string{"logger", "Logger", "log", "Log"} {
		if loggerField := val.FieldByName(loggerName); loggerField.IsValid() && loggerField.CanSet() {
			logger := NewTestLogger()
			loggerField.Set(reflect.ValueOf(logger))
			break
		}
	}
}

// GetService retrieves and casts a service to the correct type.
//
// This function provides type safety when retrieving services from the test context.
// It performs a type assertion to the specified generic type T and handles error cases
// gracefully by returning a zero value of type T if the service is not found or
// cannot be cast to the expected type.
//
// Example:
//
//	// Get a previously registered service with type safety
//	userService := GetService[*UserService](tc, "user_service")
//
//	// Use the service without need for type assertions
//	user, err := userService.GetUser(1)
func GetService[T any](tc *DBTestContext, serviceID string) T {
	service := tc.Service(serviceID)
	if service == nil {
		var zero T
		return zero
	}

	// Attempt to cast to the requested type
	if typedService, ok := service.(T); ok {
		return typedService
	}

	// Return zero value if cast fails
	var zero T
	return zero
}

// TestSuite provides a reusable test suite structure with typed service access.
//
// This generic struct offers a convenient way to organize tests for a specific service
// with full type safety. It holds a reference to the test context, the service ID, and
// a fully typed reference to the service under test.
//
// The generic parameter T specifies the type of the service being tested, which
// ensures that you can interact with the service without type assertions.
type TestSuite[T any] struct {
	// TestCtx is the database test context for this suite
	TestCtx *DBTestContext

	// ServiceID is the identifier used to register the service
	ServiceID string

	// ServiceUnderTest is a type-safe reference to the service being tested
	ServiceUnderTest T
}

// NewTestSuite creates a test suite with the service under test.
//
// This function creates a new DBTestContext, creates and registers a service of
// type T with the specified ID, and returns a TestSuite that provides convenient
// access to both the context and the service.
//
// Example:
//
//	// Create a typed test suite for a UserService
//	suite := NewTestSuite[*UserService](t, "user_service")
//	defer suite.Teardown()
//
//	// Use the service directly with proper typing
//	user, err := suite.ServiceUnderTest.GetUser(1)
func NewTestSuite[T any](t *testing.T, serviceID string) *TestSuite[T] {
	tc := NewDBTestContext(t)
	svc := CreateAndRegisterService[T](tc, serviceID)

	return &TestSuite[T]{
		TestCtx:          tc,
		ServiceID:        serviceID,
		ServiceUnderTest: svc,
	}
}

// RegisterMock registers a mock for a dependency in the test suite.
//
// This method provides a convenient way to register mock dependencies that the
// service under test might need. The mocks are registered in the underlying
// DBTestContext and can be retrieved later using GetService.
//
// Example:
//
//	// Create a mock dependency
//	mockAuthService := new(MockAuthService)
//	mockAuthService.On("ValidateToken", "valid-token").Return(true, nil)
//
//	// Register it with the test suite
//	suite.RegisterMock("auth_service", mockAuthService)
func (s *TestSuite[T]) RegisterMock(serviceID string, mock any) {
	s.TestCtx.RegisterService(serviceID, mock)
}

// Teardown cleans up the test suite by tearing down the underlying test context.
//
// This method should be called in a defer statement after creating a test suite
// to ensure proper cleanup of resources.
//
// Example:
//
//	suite := NewTestSuite[*UserService](t, "user_service")
//	defer suite.Teardown()
func (s *TestSuite[T]) Teardown() {
	if s.TestCtx != nil {
		s.TestCtx.Teardown()
	}
}

// ServiceTransactionOptions configures the transaction expectations for common
// service transaction patterns that involve finding, creating, and updating records.
//
// This struct provides a simplified way to set up SQL expectations for the most
// common transaction patterns found in services, without having to manually
// configure each step of the transaction.
type ServiceTransactionOptions struct {
	// TableName is the name of the table to operate on
	TableName string

	// FindID is the ID to use for the find operation (0 to skip find)
	FindID uint

	// CreateID is the ID to use for the create operation (0 to skip create)
	CreateID uint

	// ExpectUpdate indicates whether to expect an update operation
	ExpectUpdate bool

	// RowBuilder is a function that returns the rows to return for the find operation
	RowBuilder func() *sqlmock.Rows
}

// ExpectServiceTransaction sets up expectations for common transaction patterns.
//
// This function simplifies setting up SQL expectations for common service transactions
// that involve finding, creating, and updating records. Instead of manually configuring
// each step of the transaction, you can use this function to set up all expectations
// at once with a single function call.
//
// Example:
//
//	// Set up expectations for a transaction that finds, creates, and updates a record
//	ExpectServiceTransaction(tc, ServiceTransactionOptions{
//	    TableName:    "users",
//	    FindID:       1,            // Expect a find by ID 1
//	    CreateID:     2,            // Expect creation of record with ID 2
//	    ExpectUpdate: true,         // Expect an update operation
//	    RowBuilder:   func() *sqlmock.Rows {
//	        return UserRowBuilder() // Build rows to return for find
//	    },
//	})
func ExpectServiceTransaction(tc *DBTestContext, opts ServiceTransactionOptions) {
	// Add configurable transaction expectations
	if opts.FindID > 0 && opts.RowBuilder != nil {
		tc.ForTable(opts.TableName).ExpectFind().ByID(opts.FindID).ReturnRows(opts.RowBuilder())
	}

	if opts.CreateID > 0 {
		tc.ForTable(opts.TableName).ExpectCreate(opts.CreateID)
	}

	if opts.ExpectUpdate {
		tc.ForTable(opts.TableName).ExpectUpdate()
	}
}

// RegisterModelWithRelationships registers a model and its relationships.
//
// This function uses generics to provide a type-safe way to register a model and
// automatically discover and register its related models. It analyzes the fields
// of the model to find struct-typed fields and slice fields of struct types, which
// are likely to be relationships.
//
// Parameters:
//   - tc: The test context to register the models with
//   - additionalModels: Any additional models to register manually
//
// Example:
//
//	// Register User model and automatically discover and register Profile and Post models
//	RegisterModelWithRelationships[User](tc)
//
//	// Register User model and explicitly register additional models
//	RegisterModelWithRelationships[User](tc, &Comment{}, &Tag{})
func RegisterModelWithRelationships[T any](tc *DBTestContext, additionalModels ...any) {
	// Create a zero value of the model type
	var model T

	// Register the primary model
	tc.RegisterModel(new(T))

	// Register additional models if provided
	if len(additionalModels) > 0 {
		tc.RegisterModels(additionalModels...)
	}

	// Auto-discover relationships using reflection
	modelType := reflect.TypeOf(model)

	// Handle pointer types
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Only process structs
	if modelType.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// Skip embedded fields
		if field.Anonymous {
			continue
		}

		// Look for struct fields that might be relationships
		fieldType := field.Type

		// Handle pointer types
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		// If the field is a struct and not a built-in type, register it
		if fieldType.Kind() == reflect.Struct && !isBuiltinType(fieldType) {
			// Create an instance of the related type
			relatedInstance := reflect.New(fieldType).Interface()
			tc.RegisterModel(relatedInstance)
		}

		// Handle slice of structs (one-to-many relationships)
		if fieldType.Kind() == reflect.Slice {
			elemType := fieldType.Elem()
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}

			if elemType.Kind() == reflect.Struct && !isBuiltinType(elemType) {
				// Create an instance of the related slice element type
				relatedInstance := reflect.New(elemType).Interface()
				tc.RegisterModel(relatedInstance)
			}
		}
	}
}

// isBuiltinType checks if a type is a Go built-in type or common stdlib type
// that shouldn't be registered as a model.
//
// This function is used internally by RegisterModelWithRelationships to determine
// whether a field type should be considered a model relationship or simply a primitive
// type that doesn't need to be registered.
func isBuiltinType(t reflect.Type) bool {
	// Check for common built-in types that shouldn't be registered
	switch t.String() {
	case "string", "int", "int64", "uint", "uint64", "bool", "float64",
		"time.Time", "*time.Time", "[]byte", "map[string]interface {}":
		return true
	}

	// Check package path for stdlib packages
	pkgPath := t.PkgPath()
	if pkgPath == "" || // built-in types have empty package path
		pkgPath == "time" ||
		pkgPath == "encoding/json" {
		return true
	}

	return false
}
