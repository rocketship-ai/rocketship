package orchestrator

import "go.temporal.io/sdk/client"

// NewEngine creates a new Engine instance backed by the provided Temporal client.
func NewEngine(c client.Client, store RunStore, requireOrgScope bool) *Engine {
	if store == nil {
		panic("run store is required")
	}
	return &Engine{
		temporal:        c,
		runs:            make(map[string]*RunInfo),
		authConfig:      authConfig{},
		runStore:        store,
		requireOrgScope: requireOrgScope,
	}
}
