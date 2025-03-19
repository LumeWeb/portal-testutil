package testutil

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal/core"
)

// ServiceMockFactory is a function that creates a mock service
type ServiceMockFactory func() core.Service

// ServiceMockConfigurable defines methods a mock should implement to be configurable
type ServiceMockConfigurable interface {
	Mock() *mock.Mock
}

// ServiceMockRegistry manages mock service registrations
type ServiceMockRegistry struct {
	factories map[string]ServiceMockFactory
	mu        sync.RWMutex
}

// NewServiceMockRegistry creates a new service mock registry
func NewServiceMockRegistry() *ServiceMockRegistry {
	return &ServiceMockRegistry{
		factories: make(map[string]ServiceMockFactory),
	}
}

// RegisterMockFactory registers a factory function for creating mock services
func (r *ServiceMockRegistry) RegisterMockFactory(serviceID string, factory ServiceMockFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[serviceID] = factory
}

// CreateMock creates a mock service using the registered factory
func (r *ServiceMockRegistry) CreateMock(serviceID string) (core.Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[serviceID]
	if !exists {
		return nil, false
	}

	return factory(), true
}

// GlobalServiceRegistry is the global service mock registry
var GlobalServiceRegistry = NewServiceMockRegistry()

// RegisterMockFactory registers a mock factory in the global registry
func RegisterMockFactory(serviceID string, factory ServiceMockFactory) {
	GlobalServiceRegistry.RegisterMockFactory(serviceID, factory)
}

// MockServiceBuilder helps configure mock services with expectations
type MockServiceBuilder struct {
	mock    core.Service
	mockObj *mock.Mock
}

// NewMockServiceBuilder creates a new builder for the given mock service
func NewMockServiceBuilder(mockService core.Service) *MockServiceBuilder {
	// Extract the mock object if the service implements the right interface
	if configurable, ok := mockService.(ServiceMockConfigurable); ok {
		return &MockServiceBuilder{
			mock:    mockService,
			mockObj: configurable.Mock(),
		}
	}
	return &MockServiceBuilder{mock: mockService}
}

// On sets up an expectation on the mock
func (b *MockServiceBuilder) On(methodName string, args ...interface{}) *mock.Call {
	if b.mockObj == nil {
		panic("Mock service doesn't implement Mock() method")
	}
	return b.mockObj.On(methodName, args...)
}

// Register registers the mock service with the test context
func (b *MockServiceBuilder) Register(tc *DBTestContext, serviceID string) *MockServiceBuilder {
	tc.RegisterService(serviceID, b.mock)
	return b
}

// GetMock returns the underlying mock service
func (b *MockServiceBuilder) GetMock() core.Service {
	return b.mock
}

// GetMockObject returns the underlying mock object
func (b *MockServiceBuilder) GetMockObject() *mock.Mock {
	return b.mockObj
}

// SetupMock creates and configures a mock service
func SetupMock(tc *DBTestContext, serviceID string, configurator func(*mock.Mock)) {
	// Create the mock service
	mockService, exists := GlobalServiceRegistry.CreateMock(serviceID)
	if !exists {
		tc.T().Fatalf("No mock factory registered for service ID: %s", serviceID)
		return
	}

	// Configure the mock if a configurator was provided
	if configurator != nil {
		if mockObj, ok := mockService.(ServiceMockConfigurable); ok {
			configurator(mockObj.Mock())
		} else {
			tc.T().Fatalf("Mock service for %s doesn't implement Mock() method", serviceID)
		}
	}

	// Register the service in the context
	tc.RegisterService(serviceID, mockService)
}

// WithService is a helper function to set up a mock service in a test
func WithService(serviceID string, configurator func(*mock.Mock)) func(*DBTestContext) {
	return func(tc *DBTestContext) {
		SetupMock(tc, serviceID, configurator)
	}
}

// SetupRegisteredMocks sets up all registered mock services
func SetupRegisteredMocks(tc *DBTestContext) {
	GlobalServiceRegistry.mu.RLock()
	defer GlobalServiceRegistry.mu.RUnlock()

	// Create and register each registered mock service
	for serviceID, factory := range GlobalServiceRegistry.factories {
		mockService := factory()
		tc.RegisterService(serviceID, mockService)
	}
}

// CreateMockBuilder creates a mock service builder using the registered factory
func CreateMockBuilder(serviceID string) (*MockServiceBuilder, bool) {
	mockService, exists := GlobalServiceRegistry.CreateMock(serviceID)
	if !exists {
		return nil, false
	}

	return NewMockServiceBuilder(mockService), true
}

// WithMockBuilder is a helper function to set up a mock service with a builder
func WithMockBuilder(serviceID string, configurator func(*MockServiceBuilder)) func(*DBTestContext) {
	return func(tc *DBTestContext) {
		builder, exists := CreateMockBuilder(serviceID)
		if !exists {
			tc.T().Fatalf("No mock factory registered for service ID: %s", serviceID)
			return
		}

		if configurator != nil {
			configurator(builder)
		}

		builder.Register(tc, serviceID)
	}
}

// Legacy EmailTestHelper has been replaced by MailerTestHelper
// See mailer_test_helper.go for the new implementation

// TemplateTestHelper provides utilities for testing templates
type TemplateTestHelper struct {
	tempDir string
	t       *testing.T
}

// NewTemplateTestHelper creates a new template test helper
func NewTemplateTestHelper(t *testing.T) *TemplateTestHelper {
	tempDir, err := os.MkdirTemp("", "template_test")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &TemplateTestHelper{
		tempDir: tempDir,
		t:       t,
	}
}

// CreateTemplate creates a template file with the given content
func (h *TemplateTestHelper) CreateTemplate(name, content string) error {
	return os.WriteFile(filepath.Join(h.tempDir, name), []byte(content), 0644)
}

// GetTemplateDir returns the template directory
func (h *TemplateTestHelper) GetTemplateDir() string {
	return h.tempDir
}

// RenderTemplate renders a template with the given data
func (h *TemplateTestHelper) RenderTemplate(name string, data interface{}) (string, error) {
	tmpl, err := template.ParseFiles(filepath.Join(h.tempDir, name))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// AssertTemplateRendersTo asserts that a template renders to the expected output
func (h *TemplateTestHelper) AssertTemplateRendersTo(name string, data interface{}, expected string) {
	output, err := h.RenderTemplate(name, data)
	require.NoError(h.t, err)
	assert.Equal(h.t, expected, output)
}

// CreateTemplateWithFuncs creates a template with custom functions
func (h *TemplateTestHelper) CreateTemplateWithFuncs(name, content string, funcMap template.FuncMap) error {
	tmpl, err := template.New(name).Funcs(funcMap).Parse(content)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil) // Execute with nil data to verify syntax
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(h.tempDir, name), []byte(content), 0644)
}

// LoadAndRenderTemplate loads a template from the template directory and renders it
func (h *TemplateTestHelper) LoadAndRenderTemplate(name string, data interface{}, funcMap template.FuncMap) (string, error) {
	content, err := os.ReadFile(filepath.Join(h.tempDir, name))
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(string(content))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
