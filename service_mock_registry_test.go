package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal/core"
)

// MockService implements a mock service for testing
type MockService struct {
	mockObj mock.Mock
}

// Mock returns the underlying mock object
func (m *MockService) Mock() *mock.Mock {
	return &m.mockObj
}

// ID implements the Service interface
func (m *MockService) ID() string {
	args := m.mockObj.Called()
	return args.String(0)
}

// Initialize implements the Service interface
func (m *MockService) Initialize() error {
	args := m.mockObj.Called()
	return args.Error(0)
}

// CreateTestMockService creates a mock service for testing
func CreateTestMockService() core.Service {
	return &MockService{}
}

func TestServiceMockRegistry_RegisterMockFactory(t *testing.T) {
	// Create a new registry
	registry := NewServiceMockRegistry()

	// Register a mock factory
	serviceID := "test-service"
	registry.RegisterMockFactory(serviceID, CreateTestMockService)

	// Verify the factory was registered
	factory, exists := registry.factories[serviceID]
	assert.True(t, exists)
	assert.NotNil(t, factory)
}

func TestServiceMockRegistry_CreateMock(t *testing.T) {
	// Create a new registry
	registry := NewServiceMockRegistry()

	// Register a mock factory
	serviceID := "test-service"
	registry.RegisterMockFactory(serviceID, CreateTestMockService)

	// Create a mock service
	service, exists := registry.CreateMock(serviceID)

	// Verify the mock was created
	assert.True(t, exists)
	assert.NotNil(t, service)
	assert.IsType(t, &MockService{}, service)

	// Try to create a mock for a non-existent service
	service, exists = registry.CreateMock("non-existent")
	assert.False(t, exists)
	assert.Nil(t, service)
}

func TestGlobalServiceRegistry(t *testing.T) {
	// Clear the global registry to start fresh
	GlobalServiceRegistry = NewServiceMockRegistry()

	// Register a mock factory in the global registry
	serviceID := "global-test-service"
	RegisterMockFactory(serviceID, CreateTestMockService)

	// Verify the factory was registered
	factory, exists := GlobalServiceRegistry.factories[serviceID]
	assert.True(t, exists)
	assert.NotNil(t, factory)

	// Create a mock service
	service, exists := GlobalServiceRegistry.CreateMock(serviceID)
	assert.True(t, exists)
	assert.NotNil(t, service)
}

func TestMockServiceBuilder(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create a mock service
	mockService := &MockService{}

	// Create a builder
	builder := NewMockServiceBuilder(mockService)

	// Configure the mock
	builder.On("ID").Return("test-service")

	// Register the mock
	builder.Register(tc, "test-service")

	// Verify the builder methods
	assert.Equal(t, mockService, builder.GetMock())
	assert.Equal(t, &mockService.mockObj, builder.GetMockObject())
}

func TestSetupMock(t *testing.T) {
	// Clear the global registry to start fresh
	GlobalServiceRegistry = NewServiceMockRegistry()

	// Register a mock factory
	serviceID := "setup-test-service"
	RegisterMockFactory(serviceID, CreateTestMockService)

	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Setup the mock
	SetupMock(tc, serviceID, func(m *mock.Mock) {
		m.On("ID").Return(serviceID)
	})

	// Get the registered service without error handling
	registeredService := tc.Service(serviceID)
	assert.NotNil(t, registeredService)
	mockService, ok := registeredService.(*MockService)
	assert.True(t, ok)

	// Call the mocked method
	result := mockService.ID()
	assert.Equal(t, serviceID, result)
}

func TestWithService(t *testing.T) {
	// Clear the global registry to start fresh
	GlobalServiceRegistry = NewServiceMockRegistry()

	// Register a mock factory
	serviceID := "with-test-service"
	RegisterMockFactory(serviceID, CreateTestMockService)

	// Create a test context with the WithService helper
	tc := NewDBTestContext(t, WithTablePrefix("test_"))
	defer tc.Teardown()

	// Apply the WithService helper
	WithService(serviceID, func(m *mock.Mock) {
		m.On("ID").Return(serviceID)
	})(tc)

	// Get the registered service without error handling
	registeredService := tc.Service(serviceID)
	assert.NotNil(t, registeredService)
	mockService, ok := registeredService.(*MockService)
	assert.True(t, ok)

	// Call the mocked method
	result := mockService.ID()
	assert.Equal(t, serviceID, result)
}

func TestSetupRegisteredMocks(t *testing.T) {
	// Clear the global registry to start fresh
	GlobalServiceRegistry = NewServiceMockRegistry()

	// Register multiple mock factories
	RegisterMockFactory("service-1", CreateTestMockService)
	RegisterMockFactory("service-2", CreateTestMockService)

	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Setup all registered mocks
	SetupRegisteredMocks(tc)

	// Verify both services were registered without error handling
	service1 := tc.Service("service-1")
	assert.NotNil(t, service1)
	assert.IsType(t, &MockService{}, service1)

	service2 := tc.Service("service-2")
	assert.NotNil(t, service2)
	assert.IsType(t, &MockService{}, service2)
}

func TestCreateMockBuilder(t *testing.T) {
	// Clear the global registry to start fresh
	GlobalServiceRegistry = NewServiceMockRegistry()

	// Register a mock factory
	serviceID := "builder-test-service"
	RegisterMockFactory(serviceID, CreateTestMockService)

	// Create a mock builder
	builder, exists := CreateMockBuilder(serviceID)

	// Verify the builder was created
	assert.True(t, exists)
	assert.NotNil(t, builder)
	assert.IsType(t, &MockService{}, builder.GetMock())

	// Try to create a builder for a non-existent service
	builder, exists = CreateMockBuilder("non-existent")
	assert.False(t, exists)
	assert.Nil(t, builder)
}

func TestWithMockBuilder(t *testing.T) {
	// Clear the global registry to start fresh
	GlobalServiceRegistry = NewServiceMockRegistry()

	// Register a mock factory
	serviceID := "builder-helper-test-service"
	RegisterMockFactory(serviceID, CreateTestMockService)

	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Apply the WithMockBuilder helper
	WithMockBuilder(serviceID, func(builder *MockServiceBuilder) {
		builder.On("ID").Return(serviceID)
	})(tc)

	// Get the registered service without error handling
	registeredService := tc.Service(serviceID)
	assert.NotNil(t, registeredService)
	mockService, ok := registeredService.(*MockService)
	assert.True(t, ok)

	// Call the mocked method
	result := mockService.ID()
	assert.Equal(t, serviceID, result)
}
