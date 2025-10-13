package plugins

import (
	"context"
	"fmt"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

type Plugin interface {
	GetType() string
	Activity(ctx context.Context, p map[string]interface{}) (interface{}, error)
}

// Global plugin registry
var (
	registry          = make(map[string]Plugin)
	registryMu        sync.RWMutex
	registeredPlugins []Plugin
)

// RegisterPlugin registers a plugin in the global registry
func RegisterPlugin(plugin Plugin) {
	registryMu.Lock()
	defer registryMu.Unlock()

	pluginType := plugin.GetType()
	if _, exists := registry[pluginType]; exists {
		panic(fmt.Sprintf("plugin %s is already registered", pluginType))
	}

	registry[pluginType] = plugin
	registeredPlugins = append(registeredPlugins, plugin)
}

// GetPlugin retrieves a plugin by type from the registry
func GetPlugin(pluginType string) (Plugin, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	plugin, exists := registry[pluginType]
	return plugin, exists
}

// GetRegisteredPlugins returns all registered plugins
func GetRegisteredPlugins() []Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()

	// Return a copy to prevent external modification
	plugins := make([]Plugin, len(registeredPlugins))
	copy(plugins, registeredPlugins)
	return plugins
}

// RegisterWithTemporal registers a plugin with Temporal worker
func RegisterWithTemporal(w worker.Worker, c Plugin) {
	w.RegisterActivityWithOptions(
		c.Activity,
		activity.RegisterOptions{Name: c.GetType()},
	)
}

// RegisterAllWithTemporal registers all plugins in the registry with Temporal worker
func RegisterAllWithTemporal(w worker.Worker) {
	plugins := GetRegisteredPlugins()
	for _, plugin := range plugins {
		RegisterWithTemporal(w, plugin)
	}
}
