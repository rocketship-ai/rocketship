package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/client"
)

type Engine struct {
	generated.UnimplementedEngineServer
	temporal        client.Client
	runs            map[string]*RunInfo
	mu              sync.RWMutex
	authConfig      authConfig
	cleanupWg       sync.WaitGroup // Tracks active suite cleanup workflows
	runStore        RunStore
	requireOrgScope bool
}

type RunStore interface {
	InsertRun(ctx context.Context, run persistence.RunRecord) (persistence.RunRecord, error)
	UpdateRun(ctx context.Context, update persistence.RunUpdate) (persistence.RunRecord, error)
	GetRun(ctx context.Context, orgID uuid.UUID, runID string) (persistence.RunRecord, error)
	ListRuns(ctx context.Context, orgID uuid.UUID, limit int) ([]persistence.RunRecord, error)
	// Run details
	InsertRunTest(ctx context.Context, rt persistence.RunTest) (persistence.RunTest, error)
	UpdateRunTestByWorkflowID(ctx context.Context, workflowID, status string, errorMsg *string, endedAt time.Time, durationMs int64) error
	ListRunTests(ctx context.Context, runID string) ([]persistence.RunTest, error)
	InsertRunLog(ctx context.Context, log persistence.RunLog) (persistence.RunLog, error)
	ListRunLogs(ctx context.Context, runID string, limit int) ([]persistence.RunLog, error)
}

type RunInfo struct {
	ID                 string
	Name               string
	Status             string
	StartedAt          time.Time
	EndedAt            time.Time
	Tests              map[string]*TestInfo // Test's WorkflowID : TestInfo
	Logs               []LogLine
	Context            *RunContext
	SuiteCleanup       *dsl.CleanupSpec
	SuiteGlobals       map[string]string
	SuiteInitCompleted bool
	SuiteInitFailed    bool
	SuiteCleanupRan    bool
	Vars               map[string]interface{}
	SuiteOpenAPI       *dsl.OpenAPISuiteConfig
	OrganizationID     uuid.UUID
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
