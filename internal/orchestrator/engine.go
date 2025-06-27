package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/client"
)

func NewEngine(c client.Client) *Engine {
	return &Engine{
		temporal: c,
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

	log.Printf("[DEBUG] Starting to monitor workflow %s for run %s", workflowID, runID)
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
				ProjectId: runInfo.Context.ProjectID,
				Source:    runInfo.Context.Source,
				Branch:    runInfo.Context.Branch,
				CommitSha: runInfo.Context.CommitSHA,
				Trigger:   runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:  runInfo.Context.Metadata,
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
			TestId:    testInfo.WorkflowID,
			Name:      testInfo.Name,
			Status:    testInfo.Status,
			StartedAt: testInfo.StartedAt.Format(time.RFC3339),
			EndedAt:   testInfo.EndedAt.Format(time.RFC3339),
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
				ProjectId: runInfo.Context.ProjectID,
				Source:    runInfo.Context.Source,
				Branch:    runInfo.Context.Branch,
				CommitSha: runInfo.Context.CommitSHA,
				Trigger:   runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:  runInfo.Context.Metadata,
			},
			Tests: tests,
		},
	}
}
