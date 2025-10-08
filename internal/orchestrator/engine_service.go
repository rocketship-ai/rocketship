package orchestrator

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/embedded"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"go.temporal.io/sdk/client"
)

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
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

	if len(run.Tests) == 0 {
		return nil, fmt.Errorf("test run must contain at least one test")
	}

	runContext := extractRunContext(req.Context)

	slog.Debug("Starting run",
		"name", run.Name,
		"test_count", len(run.Tests),
		"project_id", runContext.ProjectID,
		"source", runContext.Source,
		"branch", runContext.Branch)

	runInfo := &RunInfo{
		ID:           runID,
		Name:         run.Name,
		Status:       "PENDING",
		StartedAt:    time.Now(),
		Tests:        make(map[string]*TestInfo),
		Context:      runContext,
		SuiteCleanup: run.Cleanup,
		Vars:         cloneInterfaceMap(run.Vars),
		SuiteOpenAPI: run.OpenAPI,
		Logs: []LogLine{
			{
				Msg:   fmt.Sprintf("Starting test run \"%s\"... 🚀 [%s/%s]", run.Name, runContext.ProjectID, runContext.Source),
				Color: "purple",
				Bold:  true,
			},
		},
	}

	e.mu.Lock()
	e.runs[runID] = runInfo
	e.mu.Unlock()

	var suiteGlobals map[string]string
	if len(run.Init) > 0 {
		e.addLog(runID, "Running suite init...", "n/a", false)
		initGlobals, initErr := e.runSuiteInitWorkflow(ctx, runID, run.Name, run.Init, runInfo.Vars, run.OpenAPI)
		if initErr != nil {
			e.handleSuiteInitFailure(runID, runInfo, initErr)
			return &generated.CreateRunResponse{RunId: runID}, nil
		}

		suiteGlobals = initGlobals

		e.mu.Lock()
		runInfo.SuiteGlobals = cloneStringMap(suiteGlobals)
		runInfo.SuiteInitCompleted = true
		e.mu.Unlock()

		e.addLog(runID, "Suite init completed", "green", true)
	} else {
		e.mu.Lock()
		runInfo.SuiteInitCompleted = true
		e.mu.Unlock()
	}

	// Ensure suiteSaved is non-nil when passing to workflows
	if suiteGlobals == nil {
		suiteGlobals = make(map[string]string)
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

		workflowOptions := client.StartWorkflowOptions{
			ID:        testID,
			TaskQueue: "test-workflows",
		}

		slog.Debug("Starting workflow with search attributes",
			"workflow_id", testID,
			"project_id", runContext.ProjectID,
			"suite_name", run.Name,
			"source", runContext.Source)

		suiteGlobalsCopy := cloneStringMap(suiteGlobals)
		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test, runInfo.Vars, runID, run.OpenAPI, suiteGlobalsCopy)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Failed to start test \"%s\": %v", test.Name, err), "red", true)
			e.triggerSuiteCleanup(runID, true)
			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		e.mu.Lock()
		runInfo.Tests[testID] = testInfo
		runInfo.Logs = append(runInfo.Logs, LogLine{
			Msg:   fmt.Sprintf("Running test: \"%s\"...", test.Name),
			Color: "n/a",
			Bold:  false,
		})
		e.mu.Unlock()

		go e.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())
	}

	return &generated.CreateRunResponse{RunId: runID}, nil
}

func (e *Engine) runSuiteInitWorkflow(ctx context.Context, runID, runName string, initSteps []dsl.Step, vars map[string]interface{}, suiteOpenAPI *dsl.OpenAPISuiteConfig) (map[string]string, error) {
	if len(initSteps) == 0 {
		return make(map[string]string), nil
	}

	suiteTest := dsl.Test{
		Name:  fmt.Sprintf("%s::suite-init", runName),
		Init:  initSteps,
		Steps: []dsl.Step{},
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("%s_suite_init", runID),
		TaskQueue: "test-workflows",
	}

	execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", suiteTest, vars, runID, suiteOpenAPI, map[string]string(nil))
	if err != nil {
		return nil, err
	}

	var state map[string]string
	if err := execution.Get(ctx, &state); err != nil {
		return nil, err
	}

	return extractSavedValues(state), nil
}

func (e *Engine) handleSuiteInitFailure(runID string, runInfo *RunInfo, initErr error) {
	log.Printf("[ERROR] Suite init failed for run %s: %v", runID, initErr)

	e.mu.Lock()
	runInfo.Status = "FAILED"
	runInfo.SuiteInitFailed = true
	runInfo.EndedAt = time.Now()
	e.mu.Unlock()

	e.addLog(runID, fmt.Sprintf("Suite init failed: %v", initErr), "red", true)
	e.addLog(runID, "Skipping all tests because suite init failed.", "red", true)
	e.addLog(runID, fmt.Sprintf("Test run: \"%s\" ended without executing any tests.", runInfo.Name), "n/a", true)

	e.triggerSuiteCleanup(runID, true)
}

func (e *Engine) triggerSuiteCleanup(runID string, hasFailure bool) {
	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		return
	}

	if runInfo.SuiteCleanup == nil || runInfo.SuiteCleanupRan {
		e.mu.Unlock()
		return
	}

	runInfo.SuiteCleanupRan = true
	cleanupSpec := runInfo.SuiteCleanup
	varsCopy := cloneInterfaceMap(runInfo.Vars)
	suiteGlobalsCopy := cloneStringMap(runInfo.SuiteGlobals)
	suiteOpenAPI := runInfo.SuiteOpenAPI
	e.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()

		options := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("%s_suite_cleanup", runID),
			TaskQueue: "test-workflows",
		}

		params := interpreter.SuiteCleanupParams{
			RunID:          runID,
			TestName:       "suite-cleanup",
			Cleanup:        cleanupSpec,
			Vars:           varsCopy,
			SuiteOpenAPI:   suiteOpenAPI,
			SuiteGlobals:   suiteGlobalsCopy,
			TreatAsFailure: hasFailure,
		}

		execution, err := e.temporal.ExecuteWorkflow(ctx, options, "SuiteCleanupWorkflow", params)
		if err != nil {
			log.Printf("[ERROR] Failed to start suite cleanup workflow for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Failed to start suite cleanup: %v", err), "red", true)
			return
		}

		if err := execution.Get(ctx, nil); err != nil {
			log.Printf("[ERROR] Suite cleanup workflow failed for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Suite cleanup workflow failed: %v", err), "red", true)
			return
		}

		e.addLog(runID, "Suite cleanup completed", "green", true)
	}()
}

func (e *Engine) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	runID := req.RunId

	e.mu.RLock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}

	logs := make([]LogLine, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	e.mu.RUnlock()

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

			if status == "PASSED" || status == "FAILED" {
				return nil
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

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

func (e *Engine) Health(ctx context.Context, _ *generated.HealthRequest) (*generated.HealthResponse, error) {
	return &generated.HealthResponse{Status: "ok"}, nil
}

func (e *Engine) GetServerInfo(ctx context.Context, _ *generated.GetServerInfoRequest) (*generated.GetServerInfoResponse, error) {
	version := embedded.DefaultVersion
	if version == "" {
		version = "dev"
	}

	resp := &generated.GetServerInfoResponse{
		Version:      version,
		AuthEnabled:  false,
		AuthType:     "none",
		AuthEndpoint: "",
		Capabilities: []string{"discovery.v2"},
	}

	e.authConfig.configureServerInfo(resp)
	return resp, nil
}

func (e *Engine) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	slog.Debug("ListRuns called",
		"project_id", req.ProjectId,
		"source", req.Source,
		"branch", req.Branch,
		"status", req.Status,
		"limit", req.Limit)

	e.mu.RLock()
	defer e.mu.RUnlock()

	runs := make([]*generated.RunSummary, 0)

	for _, runInfo := range e.runs {
		if req.Status != "" && runInfo.Status != req.Status {
			continue
		}
		if req.ProjectId != "" && runInfo.Context.ProjectID != req.ProjectId {
			continue
		}
		if req.Source != "" && runInfo.Context.Source != req.Source {
			continue
		}
		if req.Branch != "" && runInfo.Context.Branch != req.Branch {
			continue
		}
		if req.ScheduleName != "" && runInfo.Context.ScheduleName != req.ScheduleName {
			continue
		}

		duration := int64(0)
		if !runInfo.EndedAt.IsZero() {
			duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
		}

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

	sortRuns(runs, req.OrderBy, !req.Descending)

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

func (e *Engine) GetRun(ctx context.Context, req *generated.GetRunRequest) (*generated.GetRunResponse, error) {
	slog.Debug("GetRun called", "run_id", req.RunId)

	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if runInfo, exists := e.runs[req.RunId]; exists {
		slog.Debug("Found active run in memory", "run_id", req.RunId)
		return mapRunInfoToRunDetails(runInfo), nil
	}

	if len(req.RunId) <= 12 {
		slog.Debug("Searching for run by prefix", "prefix", req.RunId)
		for fullID, runInfo := range e.runs {
			if strings.HasPrefix(fullID, req.RunId) {
				slog.Debug("Found run by prefix match", "prefix", req.RunId, "full_id", fullID)
				return mapRunInfoToRunDetails(runInfo), nil
			}
		}
	}

	slog.Debug("Run not found in memory", "run_id", req.RunId)
	return nil, fmt.Errorf("run not found: %s", req.RunId)
}
