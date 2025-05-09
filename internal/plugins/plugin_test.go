package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPlugin is a mock implementation of the Plugin interface
type MockPlugin struct {
	mock.Mock
}

func (m *MockPlugin) GetType() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, p)
	return args.Get(0), args.Error(1)
}

func TestRegistry(t *testing.T) {
	registry := NewPluginRegistry()

	// Test Register and Get
	mockPlugin := new(MockPlugin)
	mockPlugin.On("GetType").Return("test")

	registry.Register("test", mockPlugin)

	plugin, exists := registry.Get("test")
	assert.True(t, exists)
	assert.Equal(t, mockPlugin, plugin)

	// Test Get non-existent plugin
	_, exists = registry.Get("nonexistent")
	assert.False(t, exists)
}
