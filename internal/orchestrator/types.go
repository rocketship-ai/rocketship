package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
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
	GetRunTestByWorkflowID(ctx context.Context, workflowID string) (persistence.RunTest, error)
	ListRunTests(ctx context.Context, runID string) ([]persistence.RunTest, error)
	InsertRunLog(ctx context.Context, log persistence.RunLog) (persistence.RunLog, error)
	ListRunLogs(ctx context.Context, runID string, limit int) ([]persistence.RunLog, error)
	// Step operations
	UpsertRunStep(ctx context.Context, step persistence.RunStep) (persistence.RunStep, error)
	UpdateRunTestStepCounts(ctx context.Context, runTestID uuid.UUID) error
	ListRunSteps(ctx context.Context, runTestID uuid.UUID) ([]persistence.RunStep, error)
	// Project lookup for run association
	FindProjectByRepoAndPathScope(ctx context.Context, orgID uuid.UUID, repoURL string, pathScope []string) (persistence.Project, bool, error)
	// Suite/test lookup for run linking
	GetSuiteByName(ctx context.Context, projectID uuid.UUID, name, sourceRef string) (persistence.Suite, bool, error)
	ListTestsBySuite(ctx context.Context, suiteID uuid.UUID) ([]persistence.Test, error)
	UpdateSuiteLastRun(ctx context.Context, suiteID uuid.UUID, runID, status string, runAt time.Time) error
	UpdateTestLastRun(ctx context.Context, testID uuid.UUID, runID, status string, runAt time.Time, durationMs int64) error
	// Environment lookup for run execution
	GetEnvironmentBySlug(ctx context.Context, projectID uuid.UUID, slug string) (persistence.ProjectEnvironment, error)
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
	// Project/suite/test linking for DB persistence
	ProjectID uuid.UUID            // Resolved project ID (copied from record for convenience)
	SuiteID   uuid.UUID            // Resolved suite ID
	TestIDs   map[string]uuid.UUID // Test name (lowercase) -> discovered test ID
	// Environment secrets from project environment (for template resolution)
	EnvSecrets map[string]string
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
	TestID     uuid.UUID // Resolved discovered test ID (for last_run updates)
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
