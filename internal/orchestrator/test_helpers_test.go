package orchestrator

import "go.temporal.io/sdk/client"

func newTestEngineWithClient(c client.Client) *Engine {
	return NewEngine(c, NewMemoryRunStore())
}
