package connectors

import (
	"context"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

type Connector interface {
	Execute(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	Validate(params map[string]interface{}) error
}

// ConnectorRegistry manages a collection of connectors
type ConnectorRegistry struct {
	connectors map[string]Connector
	mu         sync.RWMutex
}

func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{
		connectors: make(map[string]Connector),
	}
}

func (r *ConnectorRegistry) Register(name string, connector Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[name] = connector
}

func (r *ConnectorRegistry) Get(name string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	connector, exists := r.connectors[name]
	return connector, exists
}

type TemporalConnector interface {
	Name() string
	Activity(ctx context.Context, p map[string]interface{}) (interface{}, error)
}

func RegisterWithTemporal(w worker.Worker, c TemporalConnector) {
	w.RegisterActivityWithOptions(
		c.Activity,
		activity.RegisterOptions{Name: c.Name()},
	)
}
