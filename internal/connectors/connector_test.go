package connectors

import (
	"context"
	"testing"
)

type MockConnector struct {
	ExecuteFunc func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
}

func (m *MockConnector) Execute(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	return m.ExecuteFunc(ctx, params)
}

func (m *MockConnector) Validate(params map[string]interface{}) error {
	return nil
}

func TestConnectorRegistry(t *testing.T) {
	registry := NewConnectorRegistry()
	
	if registry == nil {
		t.Fatal("Expected registry to be created, got nil")
	}
	
	mockConnector := &MockConnector{
		ExecuteFunc: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		},
	}
	
	registry.Register("test.connector", mockConnector)
	
	connector, exists := registry.Get("test.connector")
	if !exists {
		t.Fatal("Expected connector to exist in registry")
	}
	
	if connector == nil {
		t.Fatal("Expected connector to be returned, got nil")
	}
	
	result, err := connector.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if result["result"] != "success" {
		t.Errorf("Expected result 'success', got '%v'", result["result"])
	}
	
	_, exists = registry.Get("non.existent")
	if exists {
		t.Error("Expected non-existent connector to not exist in registry")
	}
}
