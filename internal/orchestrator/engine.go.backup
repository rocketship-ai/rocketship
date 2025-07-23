package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/rbac"
	"github.com/rocketship-ai/rocketship/internal/tokens"
	"go.temporal.io/sdk/client"
)

func NewEngine(c client.Client, rbacRepo *rbac.Repository) *Engine {
	return &Engine{
		temporal: c,
		rbacRepo: rbacRepo,
		runs:     make(map[string]*RunInfo),
	}
}

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	// Validate input
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if len(req.YamlPayload) == 0 {
		return nil, fmt.Errorf("YAML payload cannot be empty")
	}

	slog.Debug("CreateRun called", "payload_size", len(req.YamlPayload))

	runID, err := generateID()
	if err != nil {
		slog.Error("Failed to generate run ID", "error", err)
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}

	run, err := dsl.ParseYAML(req.YamlPayload)
	if err != nil {
		slog.Error("Failed to parse YAML", "error", err)
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate parsed run has tests
	if len(run.Tests) == 0 {
		return nil, fmt.Errorf("test run must contain at least one test")
	}

	// Extract context from request or auto-detect
	runContext := extractRunContext(req.Context)

	slog.Debug("Starting run",
		"name", run.Name,
		"test_count", len(run.Tests),
		"project_id", runContext.ProjectID,
		"source", runContext.Source,
		"branch", runContext.Branch)

	runInfo := &RunInfo{
		ID:        runID,
		Name:      run.Name,
		Status:    "PENDING",
		StartedAt: time.Now(),
		Tests:     make(map[string]*TestInfo),
		Context:   runContext,
		Logs: []LogLine{
			{
				Msg:   fmt.Sprintf("Starting test run \"%s\"... 🚀 [%s/%s]", run.Name, runContext.ProjectID, runContext.Source),
				Color: "purple",
				Bold:  true,
			},
		},
	}

	for _, test := range run.Tests {
		testID, err := generateID()
		if err != nil {
			log.Printf("[ERROR] Failed to generate test ID: %v", err)
			return nil, fmt.Errorf("failed to generate test ID: %w", err)
		}
		testInfo := &TestInfo{
			WorkflowID: testID,
			Name:       test.Name,
			Status:     "PENDING",
			StartedAt:  time.Now(),
			RunID:      runID,
		}

		// Enhanced workflow options with search attributes (disabled for Phase 1)
		workflowOptions := client.StartWorkflowOptions{
			ID:        testID,
			TaskQueue: "test-workflows",
			// TODO Phase 2: Add search attributes after registering them in Temporal
			// SearchAttributes: map[string]interface{}{
			//     SearchAttrProjectID:    runContext.ProjectID,
			//     SearchAttrSuiteName:    run.Name,
			//     SearchAttrSource:       runContext.Source,
			//     SearchAttrBranch:       runContext.Branch,
			//     SearchAttrCommitSHA:    runContext.CommitSHA,
			//     SearchAttrTrigger:      runContext.Trigger,
			//     SearchAttrScheduleName: runContext.ScheduleName,
			//     SearchAttrStatus:       "PENDING",
			//     SearchAttrStartTime:    time.Now(),
			//     SearchAttrTotalTests:   len(run.Tests),
			//     SearchAttrPassedTests:  0,
			//     SearchAttrFailedTests:  0,
			//     SearchAttrTimeoutTests: 0,
			// },
		}

		slog.Debug("Starting workflow with search attributes",
			"workflow_id", testID,
			"project_id", runContext.ProjectID,
			"suite_name", run.Name,
			"source", runContext.Source)

		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test, run.Vars, runID)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		e.mu.Lock()
		runInfo.Tests[testID] = testInfo
		// add a log line for the start of the test
		runInfo.Logs = append(runInfo.Logs, LogLine{
			Msg:   fmt.Sprintf("Running test: \"%s\"...", test.Name),
			Color: "n/a",
			Bold:  false,
		})
		e.mu.Unlock()

		go e.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())
	}

	e.mu.Lock()
	e.runs[runID] = runInfo
	e.mu.Unlock()

	return &generated.CreateRunResponse{
		RunId: runID,
	}, nil
}

func (e *Engine) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	runID := req.RunId

	// Initial validation and log copy
	e.mu.RLock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}

	logs := make([]LogLine, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	e.mu.RUnlock()

	// Send initial logs
	for _, logMsg := range logs {
		if err := stream.Send(&generated.LogLine{
			Ts:       time.Now().Format(time.RFC3339),
			Msg:      logMsg.Msg,
			Color:    logMsg.Color,
			Bold:     logMsg.Bold,
			TestName: logMsg.TestName,
			StepName: logMsg.StepName,
		}); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastLogIndex := len(logs)

	for {
		select {
		case <-ticker.C:
			// Get new logs and status in a single lock acquisition
			e.mu.RLock()
			runInfo, exists := e.runs[runID]
			if !exists {
				e.mu.RUnlock()
				return fmt.Errorf("run not found: %s", runID)
			}

			var newLogs []LogLine
			status := runInfo.Status

			if len(runInfo.Logs) > lastLogIndex {
				newLogs = make([]LogLine, len(runInfo.Logs)-lastLogIndex)
				copy(newLogs, runInfo.Logs[lastLogIndex:])
				lastLogIndex = len(runInfo.Logs)
			}
			e.mu.RUnlock()

			// Send new logs (outside of lock)
			for _, logMsg := range newLogs {
				if err := stream.Send(&generated.LogLine{
					Ts:       time.Now().Format(time.RFC3339),
					Msg:      logMsg.Msg,
					Color:    logMsg.Color,
					Bold:     logMsg.Bold,
					TestName: logMsg.TestName,
					StepName: logMsg.StepName,
				}); err != nil {
					return err
				}
			}

			// Check if run is finished
			if status == "PASSED" || status == "FAILED" {
				return nil
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (e *Engine) monitorWorkflow(runID, workflowID, workflowRunID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	slog.Debug("Starting to monitor workflow", "workflow_id", workflowID, "run_id", runID)
	workflowRun := e.temporal.GetWorkflow(ctx, workflowID, workflowRunID)

	resultChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in workflow monitoring goroutine: %v", r)
				resultChan <- fmt.Errorf("workflow monitoring panic: %v", r)
			}
		}()

		var result interface{}
		err := workflowRun.Get(ctx, &result)
		resultChan <- err
	}()

	select {
	case err := <-resultChan:
		e.updateTestStatus(runID, workflowID, err)
	case <-ctx.Done():
		log.Printf("[WARN] Monitoring timed out for workflow %s in run %s", workflowID, runID)
		e.updateTestStatus(runID, workflowID, fmt.Errorf("workflow monitoring timeout"))
	}
}

// updateTestStatus updates the test status and handles cleanup atomically
func (e *Engine) updateTestStatus(runID, workflowID string, workflowErr error) {
	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Run not found during status update: %s", runID)
		return
	}

	testInfo, exists := runInfo.Tests[workflowID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Test not found during status update: %s in run %s", workflowID, runID)
		return
	}

	testInfo.EndedAt = time.Now()

	if workflowErr != nil {
		if workflowErr.Error() == "workflow monitoring timeout" {
			testInfo.Status = "TIMEOUT"
			e.mu.Unlock()
			log.Printf("[WARN] Test timed out: %s", testInfo.Name)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" timed out", testInfo.Name), "red", true)
		} else {
			testInfo.Status = "FAILED"
			e.mu.Unlock()
			log.Printf("[ERROR] Test failed: %s - %v", testInfo.Name, workflowErr)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %v", testInfo.Name, workflowErr), "red", true)
		}
	} else {
		testInfo.Status = "PASSED"
		e.mu.Unlock()
		log.Printf("[INFO] Test passed: %s", testInfo.Name)
		e.addLog(runID, fmt.Sprintf("Test: \"%s\" passed", testInfo.Name), "green", true)
	}

	e.checkIfRunFinished(runID)
}

func (e *Engine) addLog(runID, message, color string, bold bool) {
	e.addLogWithContext(runID, message, color, bold, "", "")
}

func (e *Engine) addLogWithContext(runID, message, color string, bold bool, testName, stepName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		log.Printf("[WARN] Run %s not found when trying to add log", runID)
		return
	}

	runInfo.Logs = append(runInfo.Logs, LogLine{
		Msg:      message,
		Color:    color,
		Bold:     bold,
		TestName: testName,
		StepName: stepName,
	})
}

func generateID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Helper function to extract run context from request or detect from environment
func extractRunContext(reqContext *generated.RunContext) *RunContext {
	if reqContext == nil {
		// Auto-detect context from environment
		slog.Debug("No context provided, auto-detecting from environment")
		return &RunContext{
			ProjectID:    detectProjectID(),
			Source:       detectSource(),
			Branch:       detectBranch(),
			CommitSHA:    detectCommitSHA(),
			Trigger:      detectTrigger(),
			ScheduleName: "",
			Metadata:     make(map[string]string),
		}
	}

	slog.Debug("Using provided context",
		"project_id", reqContext.ProjectId,
		"source", reqContext.Source,
		"branch", reqContext.Branch)

	return &RunContext{
		ProjectID:    reqContext.ProjectId,
		Source:       reqContext.Source,
		Branch:       reqContext.Branch,
		CommitSHA:    reqContext.CommitSha,
		Trigger:      reqContext.Trigger,
		ScheduleName: reqContext.ScheduleName,
		Metadata:     reqContext.Metadata,
	}
}

// Auto-detection helper functions
func detectProjectID() string {
	// Try to get from git remote origin
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		// Extract repo name from URL
		if strings.Contains(url, "/") {
			parts := strings.Split(url, "/")
			if len(parts) > 0 {
				repo := parts[len(parts)-1]
				repo = strings.TrimSuffix(repo, ".git")
				if repo != "" {
					slog.Debug("Detected project ID from git remote", "project_id", repo)
					return repo
				}
			}
		}
	}

	slog.Debug("Using default project ID")
	return "default"
}

func detectSource() string {
	// Check for common CI environment variables
	ciEnvVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "BUILDKITE"}
	for _, envVar := range ciEnvVars {
		if cmd := exec.Command("sh", "-c", fmt.Sprintf("echo $%s", envVar)); cmd != nil {
			if output, err := cmd.Output(); err == nil && strings.TrimSpace(string(output)) != "" {
				slog.Debug("Detected CI environment", "source", "ci-branch")
				return "ci-branch"
			}
		}
	}

	slog.Debug("Detected local environment", "source", "cli-local")
	return "cli-local"
}

func detectBranch() string {
	// Try git branch --show-current
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" {
			slog.Debug("Detected git branch", "branch", branch)
			return branch
		}
	}

	slog.Debug("Could not detect git branch")
	return ""
}

func detectCommitSHA() string {
	// Try git rev-parse HEAD
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err == nil {
		commit := strings.TrimSpace(string(output))
		if commit != "" {
			slog.Debug("Detected git commit", "commit", commit[:8])
			return commit
		}
	}

	slog.Debug("Could not detect git commit")
	return ""
}

func detectTrigger() string {
	// If it's CI, it's likely webhook triggered, otherwise manual
	if detectSource() == "ci-branch" {
		return "webhook"
	}
	return "manual"
}

func (e *Engine) checkIfRunFinished(runID string) {
	counts, err := e.getTestStatusCounts(runID)
	if err != nil {
		log.Printf("[ERROR] Failed to get test status counts for run %s: %v", runID, err)
		return
	}

	// Only proceed if all tests are finished (no pending tests)
	if counts.Pending > 0 {
		return
	}

	// Get run name and update status atomically
	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Run not found when checking if finished: %s", runID)
		return
	}

	runName := runInfo.Name

	// Determine final status and update
	if counts.Failed == 0 && counts.TimedOut == 0 {
		// All tests passed
		runInfo.Status = "PASSED"
		runInfo.EndedAt = time.Now()
		e.mu.Unlock()
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. All %d tests passed.", runName, counts.Total), "n/a", true)
	} else {
		// Some tests failed or timed out
		runInfo.Status = "FAILED"
		runInfo.EndedAt = time.Now()
		e.mu.Unlock()

		if counts.TimedOut == 0 {
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total), "n/a", true)
		} else {
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed, %d/%d tests timed out.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total, counts.TimedOut, counts.Total), "n/a", true)
		}
	}
}

// getTestStatusCounts returns all test status counts in a single lock acquisition
func (e *Engine) getTestStatusCounts(runID string) (TestStatusCounts, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		return TestStatusCounts{}, fmt.Errorf("run not found: %s", runID)
	}

	counts := TestStatusCounts{
		Total: len(runInfo.Tests),
	}

	for _, testInfo := range runInfo.Tests {
		switch testInfo.Status {
		case "PASSED":
			counts.Passed++
		case "FAILED":
			counts.Failed++
		case "TIMEOUT":
			counts.TimedOut++
		case "PENDING":
			counts.Pending++
		}
	}

	return counts, nil
}

// AddLog implements the add log endpoint for activities to send custom log messages
func (e *Engine) AddLog(ctx context.Context, req *generated.AddLogRequest) (*generated.AddLogResponse, error) {
	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	e.addLogWithContext(req.RunId, req.Message, req.Color, req.Bold, req.TestName, req.StepName)
	return &generated.AddLogResponse{}, nil
}

// Health implements the health check endpoint
func (e *Engine) Health(ctx context.Context, req *generated.HealthRequest) (*generated.HealthResponse, error) {
	return &generated.HealthResponse{
		Status: "ok",
	}, nil
}

// ListRuns implements the list runs endpoint - Phase 1 with in-memory data
// sortRuns sorts a slice of RunSummary based on the specified field and direction
func sortRuns(runs []*generated.RunSummary, orderBy string, ascending bool) {
	sort.Slice(runs, func(i, j int) bool {
		var result bool

		switch orderBy {
		case "duration":
			result = runs[i].DurationMs < runs[j].DurationMs
		case "ended_at":
			// Parse time strings for comparison
			timeI, errI := time.Parse(time.RFC3339, runs[i].EndedAt)
			timeJ, errJ := time.Parse(time.RFC3339, runs[j].EndedAt)
			if errI != nil || errJ != nil {
				// Fall back to string comparison if parsing fails
				result = runs[i].EndedAt < runs[j].EndedAt
			} else {
				result = timeI.Before(timeJ)
			}
		case "started_at":
		default:
			// Default to started_at
			timeI, errI := time.Parse(time.RFC3339, runs[i].StartedAt)
			timeJ, errJ := time.Parse(time.RFC3339, runs[j].StartedAt)
			if errI != nil || errJ != nil {
				// Fall back to string comparison if parsing fails
				result = runs[i].StartedAt < runs[j].StartedAt
			} else {
				result = timeI.Before(timeJ)
			}
		}

		// Reverse the result if not ascending (i.e., descending)
		if !ascending {
			result = !result
		}

		return result
	})
}

func (e *Engine) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	slog.Debug("ListRuns called",
		"project_id", req.ProjectId,
		"source", req.Source,
		"branch", req.Branch,
		"status", req.Status,
		"limit", req.Limit)

	// For Phase 1, return in-memory runs with basic filtering
	// TODO: Integrate with Temporal search attributes

	e.mu.RLock()
	defer e.mu.RUnlock()

	runs := make([]*generated.RunSummary, 0)

	for _, runInfo := range e.runs {
		// Basic filtering by status
		if req.Status != "" && runInfo.Status != req.Status {
			continue
		}

		// Filter by project ID
		if req.ProjectId != "" && runInfo.Context.ProjectID != req.ProjectId {
			continue
		}

		// Filter by source
		if req.Source != "" && runInfo.Context.Source != req.Source {
			continue
		}

		// Filter by branch
		if req.Branch != "" && runInfo.Context.Branch != req.Branch {
			continue
		}

		// Filter by schedule name
		if req.ScheduleName != "" && runInfo.Context.ScheduleName != req.ScheduleName {
			continue
		}

		duration := int64(0)
		if !runInfo.EndedAt.IsZero() {
			duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
		}

		// Count test statuses
		var passed, failed, timeout int32
		for _, test := range runInfo.Tests {
			switch test.Status {
			case "PASSED":
				passed++
			case "FAILED":
				failed++
			case "TIMEOUT":
				timeout++
			}
		}

		runSummary := &generated.RunSummary{
			RunId:        runInfo.ID,
			SuiteName:    runInfo.Name,
			Status:       runInfo.Status,
			StartedAt:    runInfo.StartedAt.Format(time.RFC3339),
			EndedAt:      runInfo.EndedAt.Format(time.RFC3339),
			DurationMs:   duration,
			TotalTests:   int32(len(runInfo.Tests)),
			PassedTests:  passed,
			FailedTests:  failed,
			TimeoutTests: timeout,
			Context: &generated.RunContext{
				ProjectId:    runInfo.Context.ProjectID,
				Source:       runInfo.Context.Source,
				Branch:       runInfo.Context.Branch,
				CommitSha:    runInfo.Context.CommitSHA,
				Trigger:      runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:     runInfo.Context.Metadata,
			},
		}

		runs = append(runs, runSummary)
	}

	// Sort runs based on request parameters
	sortRuns(runs, req.OrderBy, !req.Descending)

	// Apply limit
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(runs) > int(limit) {
		runs = runs[:limit]
	}

	result := &generated.ListRunsResponse{
		Runs:       runs,
		TotalCount: int32(len(runs)),
	}

	slog.Debug("Returning ListRuns response", "runs_count", len(runs))
	return result, nil
}

// GetRun implements the get run endpoint
func (e *Engine) GetRun(ctx context.Context, req *generated.GetRunRequest) (*generated.GetRunResponse, error) {
	slog.Debug("GetRun called", "run_id", req.RunId)

	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	// First check in-memory runs for active runs
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Try exact match first
	if runInfo, exists := e.runs[req.RunId]; exists {
		slog.Debug("Found active run in memory", "run_id", req.RunId)
		return mapRunInfoToRunDetails(runInfo), nil
	}

	// If not found and the ID looks like a prefix (12 chars or less), search for prefix match
	if len(req.RunId) <= 12 {
		slog.Debug("Searching for run by prefix", "prefix", req.RunId)
		for fullID, runInfo := range e.runs {
			if strings.HasPrefix(fullID, req.RunId) {
				slog.Debug("Found run by prefix match", "prefix", req.RunId, "full_id", fullID)
				return mapRunInfoToRunDetails(runInfo), nil
			}
		}
	}

	// TODO: Query Temporal for historical run data
	// For now, return not found
	slog.Debug("Run not found in memory", "run_id", req.RunId)
	return nil, fmt.Errorf("run not found: %s", req.RunId)
}

// TODO: Temporal query helpers will be added in Phase 2

// Map in-memory RunInfo to RunDetails for GetRun response
func mapRunInfoToRunDetails(runInfo *RunInfo) *generated.GetRunResponse {
	tests := make([]*generated.TestDetails, 0, len(runInfo.Tests))

	for _, testInfo := range runInfo.Tests {
		duration := int64(0)
		if !testInfo.EndedAt.IsZero() {
			duration = testInfo.EndedAt.Sub(testInfo.StartedAt).Milliseconds()
		}

		tests = append(tests, &generated.TestDetails{
			TestId:     testInfo.WorkflowID,
			Name:       testInfo.Name,
			Status:     testInfo.Status,
			StartedAt:  testInfo.StartedAt.Format(time.RFC3339),
			EndedAt:    testInfo.EndedAt.Format(time.RFC3339),
			DurationMs: duration,
		})
	}

	duration := int64(0)
	if !runInfo.EndedAt.IsZero() {
		duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
	}

	return &generated.GetRunResponse{
		Run: &generated.RunDetails{
			RunId:      runInfo.ID,
			SuiteName:  runInfo.Name,
			Status:     runInfo.Status,
			StartedAt:  runInfo.StartedAt.Format(time.RFC3339),
			EndedAt:    runInfo.EndedAt.Format(time.RFC3339),
			DurationMs: duration,
			Context: &generated.RunContext{
				ProjectId:    runInfo.Context.ProjectID,
				Source:       runInfo.Context.Source,
				Branch:       runInfo.Context.Branch,
				CommitSha:    runInfo.Context.CommitSHA,
				Trigger:      runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:     runInfo.Context.Metadata,
			},
			Tests: tests,
		},
	}
}

// CancelRun cancels all workflows associated with a run
func (e *Engine) CancelRun(ctx context.Context, req *generated.CancelRunRequest) (*generated.CancelRunResponse, error) {
	runID := req.RunId

	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("run not found: %s", runID),
		}, nil
	}

	// Mark run as cancelled
	runInfo.Status = "CANCELLED"

	// Cancel all workflows in this run
	cancelledCount := 0
	errorCount := 0
	for testID, testInfo := range runInfo.Tests {
		slog.Debug("Cancelling workflow", "workflow_id", testInfo.WorkflowID, "test_id", testID)
		err := e.temporal.CancelWorkflow(context.Background(), testInfo.WorkflowID, "")
		if err != nil {
			slog.Debug("Failed to cancel workflow", "workflow_id", testInfo.WorkflowID, "error", err)
			errorCount++
		} else {
			slog.Debug("Successfully cancelled workflow", "workflow_id", testInfo.WorkflowID)
			cancelledCount++
		}
	}

	e.mu.Unlock()

	message := fmt.Sprintf("Cancelled %d workflows, %d errors", cancelledCount, errorCount)

	return &generated.CancelRunResponse{
		Success: true,
		Message: message,
	}, nil
}

// Team Management Handlers

func (e *Engine) CreateTeam(ctx context.Context, req *generated.CreateTeamRequest) (*generated.CreateTeamResponse, error) {
	// Get auth context (this uses the authentication interceptor)
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user can create teams (only org admins)
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionTeamsWrite); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Create team
	team := &rbac.Team{
		ID:        uuid.New().String(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := e.rbacRepo.CreateTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	return &generated.CreateTeamResponse{
		TeamId: team.ID,
		Name:   team.Name,
	}, nil
}

func (e *Engine) ListTeams(ctx context.Context, req *generated.ListTeamsRequest) (*generated.ListTeamsResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Get all teams from database
	teams, err := e.rbacRepo.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	// Convert to proto format
	var protoTeams []*generated.Team
	for _, team := range teams {
		protoTeam := &generated.Team{
			Id:        team.ID,
			Name:      team.Name,
			CreatedAt: team.CreatedAt.Format(time.RFC3339),
		}

		// Get member count
		members, err := e.rbacRepo.GetTeamMembers(ctx, team.ID)
		if err == nil {
			protoTeam.MemberCount = int32(len(members))
		}

		// Get repository count
		repos, err := e.rbacRepo.GetTeamRepositories(ctx, team.ID)
		if err == nil {
			protoTeam.RepositoryCount = int32(len(repos))
		}

		// Check user's role in this team
		userTeams, err := e.rbacRepo.GetUserTeams(ctx, authCtx.UserID)
		if err == nil {
			for _, membership := range userTeams {
				if membership.TeamID == team.ID {
					protoTeam.UserRole = string(membership.Role)
					break
				}
			}
		}

		protoTeams = append(protoTeams, protoTeam)
	}

	return &generated.ListTeamsResponse{
		Teams: protoTeams,
	}, nil
}

func (e *Engine) AddTeamMember(ctx context.Context, req *generated.AddTeamMemberRequest) (*generated.AddTeamMemberResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Get team
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", req.TeamName)
	}

	// Check if user can manage team members
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersWrite); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate role
	var role rbac.Role
	switch req.Role {
	case "admin":
		role = rbac.RoleAdmin
	case "member":
		role = rbac.RoleMember
	default:
		return nil, fmt.Errorf("invalid role: %s (must be 'admin' or 'member')", req.Role)
	}

	// Parse permissions
	var permissions []rbac.Permission
	for _, permStr := range req.Permissions {
		switch permStr {
		case "tests:run":
			permissions = append(permissions, rbac.PermissionTestsRun)
		case "repositories:read":
			permissions = append(permissions, rbac.PermissionRepositoriesRead)
		case "repositories:write":
			permissions = append(permissions, rbac.PermissionRepositoriesWrite)
		case "repositories:manage":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		case "team:members:read":
			permissions = append(permissions, rbac.PermissionTeamMembersRead)
		case "team:members:write":
			permissions = append(permissions, rbac.PermissionTeamMembersWrite)
		case "team:members:manage":
			permissions = append(permissions, rbac.PermissionTeamMembersManage)
		case "test:schedules:manage":
			permissions = append(permissions, rbac.PermissionTestSchedulesManage)
		default:
			return nil, fmt.Errorf("invalid permission: %s", permStr)
		}
	}

	// Get or create user by email
	user, err := e.rbacRepo.GetOrCreateUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	// Add team member
	if err := e.rbacRepo.AddTeamMember(ctx, team.ID, user.ID, role, permissions); err != nil {
		return nil, fmt.Errorf("failed to add team member: %w", err)
	}

	return &generated.AddTeamMemberResponse{
		Success: true,
		Message: fmt.Sprintf("Added %s to team '%s' as %s", req.Email, req.TeamName, req.Role),
	}, nil
}

func (e *Engine) AddTeamRepository(ctx context.Context, req *generated.AddTeamRepositoryRequest) (*generated.AddTeamRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Get team
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", req.TeamName)
	}

	// Check if user can manage repositories for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// For now, return a simplified implementation that doesn't validate GitHub repo
	// TODO: Implement full GitHub validation like in the CLI version
	
	// Parse repository URL to get standard format
	parts := strings.Split(strings.TrimSuffix(req.RepositoryUrl, ".git"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repository URL format")
	}
	
	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]
	standardURL := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)
	fullName := fmt.Sprintf("%s/%s", owner, repoName)

	// Create a simple repository entry (without full GitHub validation for now)
	repository := &rbac.RepositoryEntity{
		ID:                    uuid.New().String(),
		URL:                   standardURL,
		GitHubInstallationID:  nil, // Not using GitHub app integration yet
		EnforceCodeowners:     req.EnforceCodeowners,
		CodeownersCache:       nil, // No cache yet
		CodeownersCachedAt:    nil, // No cache yet
		CreatedAt:             time.Now(),
	}

	// Create repository in database
	if err := e.rbacRepo.CreateRepository(ctx, repository); err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Add team repository association
	if err := e.rbacRepo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to add team repository: %w", err)
	}

	return &generated.AddTeamRepositoryResponse{
		Success:            true,
		Message:            fmt.Sprintf("Added repository '%s' to team '%s'", fullName, req.TeamName),
		RepositoryFullName: fullName,
	}, nil
}

func (e *Engine) RemoveTeamRepository(ctx context.Context, req *generated.RemoveTeamRepositoryRequest) (*generated.RemoveTeamRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Get team
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", req.TeamName)
	}

	// Check if user can manage repositories for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Parse repository URL to get standard format
	parts := strings.Split(strings.TrimSuffix(req.RepositoryUrl, ".git"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repository URL format")
	}
	
	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]
	standardURL := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)

	// Get repository from database
	repository, err := e.rbacRepo.GetRepository(ctx, standardURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return nil, fmt.Errorf("repository not found in system: %s", standardURL)
	}

	// Remove team repository association
	if err := e.rbacRepo.RemoveTeamFromRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to remove repository from team: %w", err)
	}

	return &generated.RemoveTeamRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Removed repository '%s' from team '%s'", standardURL, req.TeamName),
	}, nil
}

// GetTeam retrieves detailed information about a team including members and repositories
func (e *Engine) GetTeam(ctx context.Context, req *generated.GetTeamRequest) (*generated.GetTeamResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Get team by name
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", req.TeamName)
	}

	// Check if user can view this team (members can view, or org admins)
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersRead); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get team members
	members, err := e.rbacRepo.GetTeamMembers(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	// Get team repositories
	repositories, err := e.rbacRepo.GetTeamRepositories(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team repositories: %w", err)
	}

	// Convert to proto format
	var protoMembers []*generated.TeamMember
	for _, member := range members {
		// Get user details for this member
		user, err := e.rbacRepo.GetUser(ctx, member.UserID)
		if err != nil {
			slog.Warn("failed to get user details for team member", "userID", member.UserID, "error", err)
			continue
		}

		// Convert permissions to strings
		var permissionStrs []string
		for _, perm := range member.Permissions {
			permissionStrs = append(permissionStrs, string(perm))
		}

		protoMembers = append(protoMembers, &generated.TeamMember{
			UserId:      member.UserID,
			Email:       user.Email,
			Role:        string(member.Role),
			Permissions: permissionStrs,
			JoinedAt:    member.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	var protoRepositories []*generated.TeamRepository
	for _, repo := range repositories {
		// Extract repository name from URL (e.g., "github.com/owner/repo" -> "owner/repo")
		repoName := repo.URL
		if idx := strings.LastIndex(repo.URL, "/"); idx != -1 {
			if idx2 := strings.LastIndex(repo.URL[:idx], "/"); idx2 != -1 {
				repoName = repo.URL[idx2+1:]
			}
		}

		protoRepositories = append(protoRepositories, &generated.TeamRepository{
			RepositoryUrl:  repo.URL,
			RepositoryName: repoName,
			AddedAt:        repo.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	// Get user's role in this team for the response
	userRole := ""
	for _, member := range members {
		if member.UserID == authCtx.UserID {
			userRole = string(member.Role)
			break
		}
	}

	return &generated.GetTeamResponse{
		Team: &generated.Team{
			Id:               team.ID,
			Name:             team.Name,
			CreatedAt:        team.CreatedAt.Format("2006-01-02T15:04:05Z"),
			MemberCount:      int32(len(members)),
			RepositoryCount:  int32(len(repositories)),
			UserRole:         userRole,
		},
		Members:      protoMembers,
		Repositories: protoRepositories,
	}, nil
}

// RemoveTeamMember removes a member from a team
func (e *Engine) RemoveTeamMember(ctx context.Context, req *generated.RemoveTeamMemberRequest) (*generated.RemoveTeamMemberResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Get team by name
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", req.TeamName)
	}

	// Check if user can manage members for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get user by email
	user, err := e.rbacRepo.GetOrCreateUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", req.Email)
	}

	// Remove the member from the team
	if err := e.rbacRepo.RemoveTeamMember(ctx, team.ID, user.ID); err != nil {
		return nil, fmt.Errorf("failed to remove team member: %w", err)
	}

	return &generated.RemoveTeamMemberResponse{
		Success: true,
		Message: fmt.Sprintf("Removed %s from team '%s'", req.Email, req.TeamName),
	}, nil
}

// GetAuthConfig provides authentication configuration discovery for clients
func (e *Engine) GetAuthConfig(ctx context.Context, req *generated.GetAuthConfigRequest) (*generated.GetAuthConfigResponse, error) {
	// This endpoint is always accessible (no auth required) since clients need it to know HOW to authenticate
	
	// Check if authentication is configured on the server
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	dbHost := os.Getenv("ROCKETSHIP_DB_HOST")
	
	authEnabled := issuer != "" && clientID != "" && dbHost != ""
	
	response := &generated.GetAuthConfigResponse{
		AuthEnabled: authEnabled,
	}
	
	// Only include OIDC config if authentication is enabled
	if authEnabled {
		response.Oidc = &generated.OIDCConfig{
			Issuer:   issuer,
			ClientId: clientID,
			Scopes:   []string{"openid", "profile", "email"},
		}
	}
	
	slog.Debug("Auth config requested", "auth_enabled", authEnabled, "issuer", issuer, "client_id", clientID)
	
	return response, nil
}

// GetCurrentUser returns the current authenticated user's information with server-determined role
func (e *Engine) GetCurrentUser(ctx context.Context, req *generated.GetCurrentUserRequest) (*generated.GetCurrentUserResponse, error) {
	// This endpoint requires authentication - the auth context will be populated by the interceptor
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}
	
	slog.Debug("GetCurrentUser called", "user_id", authCtx.UserID, "email", authCtx.Email, "org_role", authCtx.OrgRole)
	
	// Return user info with SERVER-DETERMINED role
	return &generated.GetCurrentUserResponse{
		UserId:  authCtx.UserID,
		Email:   authCtx.Email,
		Name:    authCtx.Name,
		OrgRole: string(authCtx.OrgRole), // This is the SERVER-DETERMINED role
		Groups:  []string{}, // TODO: Add groups if needed
	}, nil
}

// Repository Management Endpoints

// AddRepository adds a new repository to the system
func (e *Engine) AddRepository(ctx context.Context, req *generated.AddRepositoryRequest) (*generated.AddRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user has repository management permissions (global admin)
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate repository URL
	if err := validateRepositoryURL(req.RepositoryUrl); err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Check if repository already exists
	existing, err := e.rbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, fmt.Errorf("failed to check existing repository: %w", err)
	}
	if existing != nil {
		return &generated.AddRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository already exists: %s", req.RepositoryUrl),
		}, nil
	}

	// Create new repository entity
	repoEntity := &rbac.RepositoryEntity{
		ID:                uuid.New().String(),
		URL:               req.RepositoryUrl,
		EnforceCodeowners: req.EnforceCodeowners,
		CreatedAt:         time.Now(),
	}

	// Create repository
	if err := e.rbacRepo.CreateRepository(ctx, repoEntity); err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return &generated.AddRepositoryResponse{
		RepositoryId:  repoEntity.ID,
		RepositoryUrl: repoEntity.URL,
		Success:       true,
		Message:       fmt.Sprintf("Repository '%s' added successfully", req.RepositoryUrl),
	}, nil
}

// ListRepositories lists all repositories in the system
func (e *Engine) ListRepositories(ctx context.Context, req *generated.ListRepositoriesRequest) (*generated.ListRepositoriesResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user has permission to view repositories
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesRead); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// List repositories
	repositories, err := e.rbacRepo.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	// Convert to proto format
	var protoRepos []*generated.Repository
	for _, repo := range repositories {
		// Get teams for this repository
		teams, err := e.rbacRepo.GetRepositoryTeamsDetailed(ctx, repo.ID)
		if err != nil {
			slog.Warn("failed to get teams for repository", "repoID", repo.ID, "error", err)
		}

		// Extract team names
		var teamNames []string
		for _, team := range teams {
			teamNames = append(teamNames, team.Name)
		}

		protoRepos = append(protoRepos, &generated.Repository{
			Id:               repo.ID,
			Url:              repo.URL,
			EnforceCodeowners: repo.EnforceCodeowners,
			CreatedAt:        repo.CreatedAt.Format("2006-01-02T15:04:05Z"),
			TeamNames:        teamNames,
			TeamCount:        int32(len(teams)),
		})
	}

	return &generated.ListRepositoriesResponse{
		Repositories: protoRepos,
	}, nil
}

// GetRepository gets detailed information about a repository
func (e *Engine) GetRepository(ctx context.Context, req *generated.GetRepositoryRequest) (*generated.GetRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user has permission to view repositories
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesRead); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := e.rbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return nil, fmt.Errorf("repository not found: %s", req.RepositoryUrl)
	}

	// Get teams for this repository
	teams, err := e.rbacRepo.GetRepositoryTeamsDetailed(ctx, repository.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository teams: %w", err)
	}

	// Extract team names
	var teamNames []string
	var protoTeams []*generated.Team
	for _, team := range teams {
		teamNames = append(teamNames, team.Name)
		
		// Convert to proto team
		protoTeams = append(protoTeams, &generated.Team{
			Id:        team.ID,
			Name:      team.Name,
			CreatedAt: team.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return &generated.GetRepositoryResponse{
		Repository: &generated.Repository{
			Id:                repository.ID,
			Url:               repository.URL,
			EnforceCodeowners: repository.EnforceCodeowners,
			CreatedAt:         repository.CreatedAt.Format("2006-01-02T15:04:05Z"),
			TeamNames:         teamNames,
			TeamCount:         int32(len(teams)),
		},
		Teams: protoTeams,
	}, nil
}

// RemoveRepository removes a repository from the system
func (e *Engine) RemoveRepository(ctx context.Context, req *generated.RemoveRepositoryRequest) (*generated.RemoveRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user has repository management permissions
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := e.rbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return &generated.RemoveRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository not found: %s", req.RepositoryUrl),
		}, nil
	}

	// Delete repository (cascade will remove team assignments)
	if err := e.rbacRepo.DeleteRepository(ctx, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to delete repository: %w", err)
	}

	return &generated.RemoveRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Repository '%s' removed successfully", req.RepositoryUrl),
	}, nil
}

// AssignTeamToRepository assigns a team to manage a repository
func (e *Engine) AssignTeamToRepository(ctx context.Context, req *generated.AssignTeamToRepositoryRequest) (*generated.AssignTeamToRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user has repository management permissions
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := e.rbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return &generated.AssignTeamToRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository not found: %s", req.RepositoryUrl),
		}, nil
	}

	// Get team
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return &generated.AssignTeamToRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("team not found: %s", req.TeamName),
		}, nil
	}

	// Assign team to repository
	if err := e.rbacRepo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to assign team to repository: %w", err)
	}

	return &generated.AssignTeamToRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Team '%s' assigned to repository '%s'", req.TeamName, req.RepositoryUrl),
	}, nil
}

// UnassignTeamFromRepository removes a team from a repository
func (e *Engine) UnassignTeamFromRepository(ctx context.Context, req *generated.UnassignTeamFromRepositoryRequest) (*generated.UnassignTeamFromRepositoryResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Check if user has repository management permissions
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := e.rbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return &generated.UnassignTeamFromRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository not found: %s", req.RepositoryUrl),
		}, nil
	}

	// Get team
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return &generated.UnassignTeamFromRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("team not found: %s", req.TeamName),
		}, nil
	}

	// Remove team from repository
	if err := e.rbacRepo.RemoveTeamFromRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to remove team from repository: %w", err)
	}

	return &generated.UnassignTeamFromRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Team '%s' removed from repository '%s'", req.TeamName, req.RepositoryUrl),
	}, nil
}

// validateRepositoryURL validates a repository URL
func validateRepositoryURL(repoURL string) error {
	// Parse URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	// Check if it looks like a GitHub URL
	if !strings.Contains(u.Host, "github") {
		return fmt.Errorf("currently only GitHub repositories are supported")
	}

	// Check path format
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 {
		return fmt.Errorf("URL must include owner and repository name")
	}

	return nil
}

// CreateToken creates a new API token for a team
func (e *Engine) CreateToken(ctx context.Context, req *generated.CreateTokenRequest) (*generated.CreateTokenResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create RBAC enforcer
	enforcer := rbac.NewEnforcer(e.rbacRepo)

	// Get team
	team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", req.TeamName)
	}

	// Check if user can manage API tokens for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Parse permissions
	var permissions []rbac.Permission
	for _, permStr := range req.Permissions {
		switch permStr {
		case "tests:run":
			permissions = append(permissions, rbac.PermissionTestsRun)
		case "repositories:read":
			permissions = append(permissions, rbac.PermissionRepositoriesRead)
		case "repositories:write":
			permissions = append(permissions, rbac.PermissionRepositoriesWrite)
		case "repositories:manage":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		case "test_runs": // Legacy support
			permissions = append(permissions, rbac.PermissionTestsRun)
		default:
			return &generated.CreateTokenResponse{
				Success: false,
				Message: fmt.Sprintf("invalid permission: %s. Valid permissions: tests:run, repositories:read, repositories:write, repositories:manage", permStr),
			}, nil
		}
	}

	// Parse expiration date
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		expires, err := time.Parse("2006-01-02", req.ExpiresAt)
		if err != nil {
			return &generated.CreateTokenResponse{
				Success: false,
				Message: fmt.Sprintf("invalid expires date format (use YYYY-MM-DD): %v", err),
			}, nil
		}
		expiresAt = &expires
	}

	// Create token manager
	tokenManager := tokens.NewManager(e.rbacRepo)

	// Create token request
	createReq := &tokens.CreateTokenRequest{
		TeamID:      team.ID,
		Name:        req.Name,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		CreatedBy:   authCtx.UserID,
	}

	// Create token
	resp, err := tokenManager.CreateToken(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Format expiration date for response
	expiresStr := ""
	if resp.ExpiresAt != nil {
		expiresStr = resp.ExpiresAt.Format("2006-01-02")
	}

	return &generated.CreateTokenResponse{
		TokenId:     resp.TokenID,
		Token:       resp.Token,
		TeamName:    req.TeamName,
		Permissions: req.Permissions,
		ExpiresAt:   expiresStr,
		Success:     true,
		Message:     fmt.Sprintf("API token '%s' created successfully for team '%s'", req.Name, req.TeamName),
	}, nil
}

// ListTokens lists API tokens for teams the user has access to
func (e *Engine) ListTokens(ctx context.Context, req *generated.ListTokensRequest) (*generated.ListTokensResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create token manager
	tokenManager := tokens.NewManager(e.rbacRepo)

	var allTokens []*generated.ApiToken

	if req.TeamName != "" {
		// List tokens for specific team
		team, err := e.rbacRepo.GetTeamByName(ctx, req.TeamName)
		if err != nil {
			return nil, fmt.Errorf("failed to get team: %w", err)
		}
		if team == nil {
			return nil, fmt.Errorf("team not found: %s", req.TeamName)
		}

		// Check if user has access to this team
		hasAccess := false
		if authCtx.IsOrgAdmin() {
			hasAccess = true
		} else {
			for _, membership := range authCtx.TeamMemberships {
				if membership.TeamID == team.ID {
					hasAccess = true
					break
				}
			}
		}

		if !hasAccess {
			return nil, fmt.Errorf("permission denied: no access to team %s", req.TeamName)
		}

		// Get tokens for this team
		teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list tokens for team %s: %w", req.TeamName, err)
		}

		// Convert to protobuf format
		for _, token := range teamTokens {
			// Convert permissions to string slice
			permStrs := make([]string, len(token.Permissions))
			for i, perm := range token.Permissions {
				permStrs[i] = string(perm)
			}

			// Format timestamps
			lastUsedStr := ""
			if token.LastUsedAt != nil {
				lastUsedStr = token.LastUsedAt.Format("2006-01-02 15:04:05")
			}

			expiresStr := ""
			if token.ExpiresAt != nil {
				expiresStr = token.ExpiresAt.Format("2006-01-02")
			}

			allTokens = append(allTokens, &generated.ApiToken{
				Id:          token.ID,
				Name:        token.Name,
				TeamId:      token.TeamID,
				TeamName:    team.Name,
				Permissions: permStrs,
				CreatedAt:   token.CreatedAt.Format("2006-01-02 15:04:05"),
				LastUsedAt:  lastUsedStr,
				ExpiresAt:   expiresStr,
				CreatedBy:   token.CreatedBy,
			})
		}
	} else {
		// List tokens for all teams the user has access to
		if authCtx.IsOrgAdmin() {
			// Org admins can see all tokens
			teams, err := e.rbacRepo.ListTeams(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list teams: %w", err)
			}

			for _, team := range teams {
				teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
				if err != nil {
					continue // Skip teams with errors
				}

				// Convert to protobuf format
				for _, token := range teamTokens {
					permStrs := make([]string, len(token.Permissions))
					for i, perm := range token.Permissions {
						permStrs[i] = string(perm)
					}

					lastUsedStr := ""
					if token.LastUsedAt != nil {
						lastUsedStr = token.LastUsedAt.Format("2006-01-02 15:04:05")
					}

					expiresStr := ""
					if token.ExpiresAt != nil {
						expiresStr = token.ExpiresAt.Format("2006-01-02")
					}

					allTokens = append(allTokens, &generated.ApiToken{
						Id:          token.ID,
						Name:        token.Name,
						TeamId:      token.TeamID,
						TeamName:    team.Name,
						Permissions: permStrs,
						CreatedAt:   token.CreatedAt.Format("2006-01-02 15:04:05"),
						LastUsedAt:  lastUsedStr,
						ExpiresAt:   expiresStr,
						CreatedBy:   token.CreatedBy,
					})
				}
			}
		} else {
			// Regular users can only see tokens for their teams
			for _, membership := range authCtx.TeamMemberships {
				team, err := e.rbacRepo.GetTeam(ctx, membership.TeamID)
				if err != nil {
					continue
				}
				if team == nil {
					continue
				}

				teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
				if err != nil {
					continue
				}

				// Convert to protobuf format
				for _, token := range teamTokens {
					permStrs := make([]string, len(token.Permissions))
					for i, perm := range token.Permissions {
						permStrs[i] = string(perm)
					}

					lastUsedStr := ""
					if token.LastUsedAt != nil {
						lastUsedStr = token.LastUsedAt.Format("2006-01-02 15:04:05")
					}

					expiresStr := ""
					if token.ExpiresAt != nil {
						expiresStr = token.ExpiresAt.Format("2006-01-02")
					}

					allTokens = append(allTokens, &generated.ApiToken{
						Id:          token.ID,
						Name:        token.Name,
						TeamId:      token.TeamID,
						TeamName:    team.Name,
						Permissions: permStrs,
						CreatedAt:   token.CreatedAt.Format("2006-01-02 15:04:05"),
						LastUsedAt:  lastUsedStr,
						ExpiresAt:   expiresStr,
						CreatedBy:   token.CreatedBy,
					})
				}
			}
		}
	}

	return &generated.ListTokensResponse{
		Tokens: allTokens,
	}, nil
}

// RevokeToken revokes an API token
func (e *Engine) RevokeToken(ctx context.Context, req *generated.RevokeTokenRequest) (*generated.RevokeTokenResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create token manager
	tokenManager := tokens.NewManager(e.rbacRepo)

	// Check if user can revoke tokens
	canRevoke := false
	
	// Check if user is org admin (can revoke any token)
	if authCtx.IsOrgAdmin() {
		canRevoke = true
	} else {
		// Check if user is admin of any team (simplified check)
		for _, teamMember := range authCtx.TeamMemberships {
			if teamMember.Role == rbac.RoleAdmin {
				canRevoke = true
				break
			}
		}
	}

	if !canRevoke {
		return &generated.RevokeTokenResponse{
			Success: false,
			Message: "Permission denied: only organization admins or team admins can revoke API tokens",
		}, nil
	}

	// Revoke the token
	if err := tokenManager.RevokeToken(ctx, req.TokenId); err != nil {
		return &generated.RevokeTokenResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to revoke token: %v", err),
		}, nil
	}

	return &generated.RevokeTokenResponse{
		Success: true,
		Message: fmt.Sprintf("API token '%s' revoked successfully", req.TokenId),
	}, nil
}
