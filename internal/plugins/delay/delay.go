package delay

import (
	"context"
)

func (dp *DelayPlugin) GetType() string {
	return "delay"
}

func (dp *DelayPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	// Dummy activity to satisfy the interface. Delay will just be a workflow sleep.
	return nil, nil
}
