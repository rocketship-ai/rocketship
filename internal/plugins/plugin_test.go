package plugins

import (
	"context"

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
