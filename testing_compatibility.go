package testutil

import (
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestContextCompat is a compatibility wrapper for core.TestContext
type TestContextCompat struct {
	coreTesting.TestContext
}

// GetService is a compatibility layer for applications expecting GetService method
func (tc *TestContextCompat) GetService(serviceID string) core.Service {
	// Call the existing Service method
	service := tc.Service(serviceID)
	if service != nil {
		if serviceItem, ok := service.(core.Service); ok {
			return serviceItem
		}
	}
	return nil
}

// WrapTestContext wraps a core.TestContext with compatibility methods
func WrapTestContext(ctx coreTesting.TestContext) *TestContextCompat {
	return &TestContextCompat{
		TestContext: ctx,
	}
}
