package orchestrator

import (
	"sync"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"go.temporal.io/sdk/client"
)

type Engine struct {
	generated.UnimplementedEngineServer
	temporal   client.Client
	runs       map[string]*RunInfo
	mu         sync.RWMutex
	authConfig authConfig
}
type RunInfo struct {
	ID        string
	Name      string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
	Tests     map[string]*TestInfo // Test's WorkflowID : TestInfo
	Logs      []LogLine
	Context   *RunContext
}

type LogLine struct {
	Msg      string
	Color    string
	Bold     bool
	TestName string
	StepName string
}

type TestInfo struct {
	WorkflowID string
	Name       string
	Status     string
	StartedAt  time.Time
	EndedAt    time.Time
	RunID      string
}

// TestStatusCounts represents the count of tests in different states
type TestStatusCounts struct {
	Total    int
	Passed   int
	Failed   int
	TimedOut int
	Pending  int
}

// Enhanced data structures for persistence
type EnhancedRunInfo struct {
	*RunInfo
	Context   *RunContext
	SuiteName string
}

type RunContext struct {
	ProjectID    string
	Source       string
	Branch       string
	CommitSHA    string
	Trigger      string
	ScheduleName string
	Metadata     map[string]string
}

// Temporal search attribute keys (must be registered in Temporal cluster)
const (
	SearchAttrProjectID    = "ProjectId"
	SearchAttrSuiteName    = "SuiteName"
	SearchAttrSource       = "Source"
	SearchAttrBranch       = "Branch"
	SearchAttrCommitSHA    = "CommitSHA"
	SearchAttrTrigger      = "Trigger"
	SearchAttrScheduleName = "ScheduleName"
	SearchAttrStatus       = "Status"
	SearchAttrStartTime    = "StartTime"
	SearchAttrEndTime      = "EndTime"
	SearchAttrDurationMs   = "DurationMs"
	SearchAttrTotalTests   = "TotalTests"
	SearchAttrPassedTests  = "PassedTests"
	SearchAttrFailedTests  = "FailedTests"
	SearchAttrTimeoutTests = "TimeoutTests"
)
