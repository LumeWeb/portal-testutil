package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal/core"
)

// ExampleMockServiceDemo demonstrates how to use the MockServiceBuilder
func ExampleMockServiceDemo() {
	// This is just an example and won't actually run

	// Register a mock factory
	RegisterMockFactory("example_service", func() core.Service {
		return &MockServiceExample{}
	})

	// In your test:
	// t := &testing.T{}
	// tc := NewDBTestContext(t)

	// Method 1: Using WithMockBuilder
	// tc.WithMockBuilder("example_service", func(builder *MockServiceBuilder) {
	//     builder.On("SomeMethod", "arg1", "arg2").Return("result", nil)
	//     builder.On("OtherMethod", mock.Anything).Return(nil)
	// })

	// Method 2: Creating a builder directly
	// builder, _ := CreateMockBuilder("example_service")
	// builder.On("SomeMethod", "arg1", "arg2").Return("result", nil)
	// builder.Register(tc, "example_service")

	// Method 3: Using the existing WithService helper
	// tc.WithService("example_service", func(m *mock.Mock) {
	//     m.On("SomeMethod", "arg1", "arg2").Return("result", nil)
	// })
}

// MockServiceExample is an example mock service implementation
type MockServiceExample struct {
	mockObj mock.Mock
}

// ID implements the Service interface
func (m *MockServiceExample) ID() string {
	return "example_service"
}

// Initialize implements the Service interface
func (m *MockServiceExample) Initialize() error {
	args := m.mockObj.Called()
	return args.Error(0)
}

// SomeMethod is an example method
func (m *MockServiceExample) SomeMethod(arg1, arg2 string) (string, error) {
	args := m.mockObj.Called(arg1, arg2)
	return args.String(0), args.Error(1)
}

// OtherMethod is another example method
func (m *MockServiceExample) OtherMethod(arg interface{}) error {
	args := m.mockObj.Called(arg)
	return args.Error(0)
}

// Mock implements the ServiceMockConfigurable interface
func (m *MockServiceExample) Mock() *mock.Mock {
	return &m.mockObj
}

// MockServiceBuilderExample demonstrates how to use the MockServiceBuilder in a real test
func MockServiceBuilderExample(t *testing.T) {
	// Skip this test as it's just an example
	t.Skip("This is just an example test")

	// Register a mock factory
	RegisterMockFactory("example_service", func() core.Service {
		return &MockServiceExample{}
	})

	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create a mock service builder
	builder, exists := CreateMockBuilder("example_service")
	assert.True(t, exists)

	// Configure the mock
	builder.On("SomeMethod", "hello", "world").Return("hello world", nil)

	// Register the mock service
	builder.Register(tc, "example_service")

	// Get the service from the context
	service := tc.Service("example_service")
	assert.NotNil(t, service)

	// Use the service
	mockService := service.(*MockServiceExample)
	result, err := mockService.SomeMethod("hello", "world")

	// Verify the result
	assert.NoError(t, err)
	assert.Equal(t, "hello world", result)

	// Verify expectations
	mockService.mockObj.AssertExpectations(t)
}
