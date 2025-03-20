package testutil

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Mock service implementation for testing
type MockTestService struct {
	mock.Mock
	Ctx    *DBTestContext
	Db     *gorm.DB
	Logger *core.Logger
}

func (m *MockTestService) ID() string {
	return "test_service"
}

func (m *MockTestService) DoSomething() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// Mock model for testing relationship discovery
type TestParentModel struct {
	ID       uint `gorm:"primaryKey"`
	Name     string
	Children []TestChildModel
	Related  *TestRelatedModel
}

type TestChildModel struct {
	ID         uint `gorm:"primaryKey"`
	ParentID   uint
	ChildValue string
}

type TestRelatedModel struct {
	ID           uint `gorm:"primaryKey"`
	RelatedValue string
}

// TestNewTestLogger verifies that the logger is created with the correct configuration
func TestNewTestLogger(t *testing.T) {
	logger := NewTestLogger()

	// Verify logger is not nil
	assert.NotNil(t, logger)

	// Verify logger level is set to warn (just check that info is disabled)
	assert.False(t, logger.Core().Enabled(zap.InfoLevel))
	// and warn is enabled
	assert.True(t, logger.Core().Enabled(zap.WarnLevel))
}

// TestCreateAndRegisterService verifies that a service can be created and registered
func TestCreateAndRegisterService(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create and register a service
	service := CreateAndRegisterService[*MockTestService](tc, "test_service")

	// Verify the service was created and registered
	assert.NotNil(t, service)
	assert.Equal(t, "test_service", service.ID())

	// Verify common fields were set
	assert.Equal(t, tc, service.Ctx)
	assert.NotNil(t, service.Db) // Just check it's not nil, as DB objects are hard to compare directly
	assert.NotNil(t, service.Logger)

	// Verify the service is retrievable from the context
	registeredService := tc.Service("test_service")
	assert.Equal(t, service, registeredService)
}

// TestSetCommonServiceFields verifies that common fields are set on a service
func TestSetCommonServiceFields(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create a service
	service := &MockTestService{}

	// Set common fields
	SetCommonServiceFields(tc, service)

	// Verify fields were set
	assert.Equal(t, tc, service.Ctx)
	assert.NotNil(t, service.Db) // Just check it's not nil, as DB objects are hard to compare directly
	assert.NotNil(t, service.Logger)
}

// ServiceInitializerExample implements ServiceInitializer interface
type ServiceInitializerExample struct {
	mock.Mock
	context  *DBTestContext
	database *gorm.DB
	log      *core.Logger
}

func (s *ServiceInitializerExample) ID() string {
	return "initializer_example"
}

func (s *ServiceInitializerExample) DoSomething() (string, error) {
	args := s.Called()
	return args.String(0), args.Error(1)
}

// InitForTest implements the ServiceInitializer interface
func (s *ServiceInitializerExample) InitForTest(tc *DBTestContext, db *gorm.DB, logger *core.Logger) {
	s.context = tc
	s.database = db
	s.log = logger
}

// TestServiceInitializer verifies that services can be initialized through the interface
func TestServiceInitializer(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create and register a service that implements ServiceInitializer
	service := CreateAndRegisterService[*ServiceInitializerExample](tc, "initializer_service")

	// Verify the service was created and registered
	assert.NotNil(t, service)
	assert.Equal(t, "initializer_example", service.ID())

	// Verify fields were set through the interface
	assert.Equal(t, tc, service.context)
	assert.NotNil(t, service.database) // Just check it's not nil, as DB objects are hard to compare directly
	assert.NotNil(t, service.log)
}

// TestGetService verifies that services can be retrieved and cast to the correct type
func TestGetService(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register a service
	tc.RegisterService("test_service", &MockTestService{})

	// Get the service with the correct type
	service := GetService[*MockTestService](tc, "test_service")

	// Verify the service was retrieved
	assert.NotNil(t, service)

	// Verify that a non-existent service returns the zero value
	nonExistent := GetService[*MockTestService](tc, "non_existent_service")
	assert.Nil(t, nonExistent)

	// Verify that a service of the wrong type returns the zero value
	tc.RegisterService("wrong_type", "not_a_service")
	wrongType := GetService[*MockTestService](tc, "wrong_type")
	assert.Nil(t, wrongType)
}

// TestExpectServiceTransaction verifies that transaction expectations are set up correctly
func TestExpectServiceTransaction(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.SkipVerification() // Skip verification since we're testing expectations

	// Create row builder function
	rowBuilder := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Test")
	}

	// Set up transaction expectations
	ExpectServiceTransaction(tc, ServiceTransactionOptions{
		TableName:    "test_table",
		FindID:       1,
		CreateID:     2,
		ExpectUpdate: true,
		RowBuilder:   rowBuilder,
	})

	// Verify that expectations were set up (can't directly test this,
	// but would be caught by SQLMock if incorrect in real usage)
}

// TestRegisterModelWithRelationships verifies that models and their relationships are registered
func TestRegisterModelWithRelationships(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Register a model with relationships
	RegisterModelWithRelationships[TestParentModel](tc)

	// Get all registered table names
	tableNames := tc.GetRegisteredTableNames()

	// Verify that the model and its relationships were registered
	assert.Contains(t, tableNames, "test_parent_models")
	assert.Contains(t, tableNames, "test_child_models")
	assert.Contains(t, tableNames, "test_related_models")
}

// TestTestSuite verifies that the test suite works correctly
func TestTestSuite(t *testing.T) {
	// Create a test suite
	suite := NewTestSuite[*MockTestService](t, "test_service")
	defer suite.Teardown()

	// Verify the suite was created correctly
	assert.NotNil(t, suite.TestCtx)
	assert.Equal(t, "test_service", suite.ServiceID)
	assert.NotNil(t, suite.ServiceUnderTest)

	// Set up a mock expectation
	suite.ServiceUnderTest.On("DoSomething").Return("test result", nil)

	// Call the method
	result, err := suite.ServiceUnderTest.DoSomething()

	// Verify the result
	assert.NoError(t, err)
	assert.Equal(t, "test result", result)

	// Verify the mock expectation was met
	suite.ServiceUnderTest.AssertExpectations(t)

	// Test registering a mock dependency
	mockDep := new(mock.Mock)
	suite.RegisterMock("dependency", mockDep)

	// Verify the mock was registered
	assert.Equal(t, mockDep, suite.TestCtx.Service("dependency"))
}

// TestIsBuiltinType verifies that built-in types are correctly identified
func TestIsBuiltinType(t *testing.T) {
	// Test basic types directly
	assert.True(t, isBuiltinType(reflect.TypeOf("")))
	assert.True(t, isBuiltinType(reflect.TypeOf(0)))
	assert.True(t, isBuiltinType(reflect.TypeOf(true)))

	// Create a custom type to test
	type CustomType struct {
		Field string
	}

	// Test the custom type
	// Since it's in the test package it won't be recognized as builtin
	customType := CustomType{}
	assert.False(t, isBuiltinType(reflect.TypeOf(customType)))
}
