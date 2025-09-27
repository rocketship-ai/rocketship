package orchestrator

import "go.temporal.io/sdk/client"

// NewEngine creates a new Engine instance backed by the provided Temporal client.
func NewEngine(c client.Client) *Engine {
	return &Engine{
		temporal: c,
		runs:     make(map[string]*RunInfo),
	}
}
