package plugins

import (
	"testing"

	"go.temporal.io/sdk/workflow"
)

type MockPlugin struct {
	ActivityFunc func(ctx workflow.Context, params map[string]interface{}) (map[string]interface{}, error)
	GetTypeFunc  func() string
}

func (m *MockPlugin) Activity(ctx workflow.Context, p map[string]interface{}) (interface{}, error) {
	return m.ActivityFunc(ctx, p)
}

func (m *MockPlugin) GetType() string {
	return m.GetTypeFunc()
}

func TestPluginRegistry(t *testing.T) {
	registry := NewPluginRegistry()

	if registry == nil {
		t.Fatal("Expected registry to be created, got nil")
	}

	mockPlugin := &MockPlugin{
		ActivityFunc: func(ctx workflow.Context, params map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		},
		GetTypeFunc: func() string {
			return "test.plugin"
		},
	}

	registry.Register("test.plugin", mockPlugin)

	plugin, exists := registry.Get("test.plugin")
	if !exists {
		t.Fatal("Expected plugin to exist in registry")
	}

	if plugin == nil {
		t.Fatal("Expected plugin to be returned, got nil")
	}

	_, exists = registry.Get("non.existent")
	if exists {
		t.Error("Expected non-existent plugin to not exist in registry")
	}
}
