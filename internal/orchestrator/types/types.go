package types

import "time"

// RunContext contains metadata about the test run execution context
type RunContext struct {
	ProjectID    string            `json:"project_id"`
	Source       string            `json:"source"`       // "cli-local", "ci-branch", "webhook", etc.
	Branch       string            `json:"branch"`       // Git branch
	CommitSHA    string            `json:"commit_sha"`   // Git commit SHA
	Trigger      string            `json:"trigger"`      // "manual", "webhook", "schedule"
	ScheduleName string            `json:"schedule_name"` // If triggered by schedule
	Metadata     map[string]string `json:"metadata"`     // Additional metadata
}

// RunInfo stores information about test runs
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

// TestStatus represents the status of an individual test
type TestStatus struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// TestInfo contains information about a test execution
type TestInfo struct {
	WorkflowID string
	Name       string
	Status     string
	StartedAt  time.Time
	EndedAt    time.Time
	RunID      string
}

// LogLine represents a single log entry
type LogLine struct {
	Msg      string
	Color    string
	Bold     bool
	TestName string
	StepName string
}

// TestStatusCounts tracks counts of test statuses
type TestStatusCounts struct {
	Total    int
	Passed   int
	Failed   int
	TimedOut int
	Pending  int
}