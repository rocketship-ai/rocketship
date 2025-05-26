package delay

import (
	"context"

	"github.com/rocketship-ai/rocketship/internal/plugins"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&DelayPlugin{})
}

func (dp *DelayPlugin) GetType() string {
	return "delay"
}

func (dp *DelayPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	// Dummy activity to satisfy the interface. Delay will just be a workflow sleep.
	return nil, nil
}
