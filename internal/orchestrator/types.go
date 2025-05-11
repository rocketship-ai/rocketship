package orchestrator

import (
	"sync"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"go.temporal.io/sdk/client"
)

type Engine struct {
	generated.UnimplementedEngineServer
	temporal client.Client
	runs     map[string]*RunInfo
	mu       sync.RWMutex
}
type RunInfo struct {
	ID        string
	Name      string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
	Tests     map[string]*TestInfo // Test's WorkflowID : TestInfo
	Logs      []LogLine
}

type LogLine struct {
	Msg   string
	Color string
	Bold  bool
}

type TestInfo struct {
	WorkflowID string
	Name       string
	Status     string
	StartedAt  time.Time
	EndedAt    time.Time
	RunID      string
}
