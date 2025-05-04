package plugins

import (
	"context"
	"testing"
)

type MockPlugin struct {
	ExecuteFunc func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
}

func (m *MockPlugin) Execute(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	return m.ExecuteFunc(ctx, params)
}

func (m *MockPlugin) Validate(params map[string]interface{}) error {
	return nil
}

func TestPluginRegistry(t *testing.T) {
	registry := NewPluginRegistry()
	
	if registry == nil {
		t.Fatal("Expected registry to be created, got nil")
	}
	
	mockPlugin := &MockPlugin{
		ExecuteFunc: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
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
	
	result, err := plugin.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if result["result"] != "success" {
		t.Errorf("Expected result 'success', got '%v'", result["result"])
	}
	
	_, exists = registry.Get("non.existent")
	if exists {
		t.Error("Expected non-existent plugin to not exist in registry")
	}
}
