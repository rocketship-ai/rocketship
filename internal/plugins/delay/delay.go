package delay

import (
	"context"
	"fmt"
)

type DelayPlugin struct {
	Name   string      `json:"name" yaml:"name"`
	Plugin string      `json:"plugin" yaml:"plugin"`
	Config DelayConfig `json:"config" yaml:"config"`
}

type DelayConfig struct {
	Duration string `json:"duration" yaml:"duration"`
}

func (dp *DelayPlugin) GetType() string {
	return "delay"
}

func (dp *DelayPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	// Dummy activity to satisfy the interface. Delay will just be a workflow sleep.
	fmt.Println("Dummy activity called")
	return nil, nil
}
