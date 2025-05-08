package delay

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

type DelayPlugin struct {
	Name   string      `json:"name" yaml:"name"`
	Plugin string      `json:"plugin" yaml:"plugin"`
	Config DelayConfig `json:"config" yaml:"config"`
}

type DelayConfig struct {
	Duration time.Duration `json:"duration" yaml:"duration"`
}

func (p *DelayPlugin) GetType() string {
	return "delay"
}

func (p *DelayPlugin) Activity(ctx workflow.Context, params map[string]interface{}) (map[string]interface{}, error) {
	// Dummy activity to satisfy the interface. Delay will just be a workflow sleep.
	fmt.Println("Dummy activity called")
	return nil, nil
}
