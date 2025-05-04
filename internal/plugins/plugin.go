package plugins

import (
	"context"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

type Plugin interface {
	Execute(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	Validate(params map[string]interface{}) error
}

// PluginRegistry manages a collection of plugins
type PluginRegistry struct {
	plugins map[string]Plugin
	mu         sync.RWMutex
}

func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
	}
}

func (r *PluginRegistry) Register(name string, plugin Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[name] = plugin
}

func (r *PluginRegistry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	plugin, exists := r.plugins[name]
	return plugin, exists
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
