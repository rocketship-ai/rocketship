package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
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

	principal, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("CreateRun called", "payload_size", len(req.YamlPayload), "org_id", orgID.String())

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
	startTime := time.Now().UTC()
	configSource := detectConfigSource(runContext)
	environment := detectEnvironment(runContext)
	initiator := determineInitiator(principal)

	if orgID != uuid.Nil && e.runStore != nil {
		record := persistence.RunRecord{
			ID:             runID,
			OrganizationID: orgID,
			Status:         "RUNNING",
			SuiteName:      run.Name,
			Initiator:      initiator,
			Trigger:        strings.TrimSpace(runContext.Trigger),
			ScheduleName:   strings.TrimSpace(runContext.ScheduleName),
			ConfigSource:   configSource,
			Source:         strings.TrimSpace(runContext.Source),
			Branch:         strings.TrimSpace(runContext.Branch),
			Environment:    environment,
			CommitSHA:      makeNullString(runContext.CommitSHA),
			BundleSHA:      sql.NullString{},
			TotalTests:     len(run.Tests),
			PassedTests:    0,
			FailedTests:    0,
			TimeoutTests:   0,
			StartedAt:      sql.NullTime{Time: startTime, Valid: true},
		}

		if _, err := e.runStore.InsertRun(ctx, record); err != nil {
			slog.Error("CreateRun: failed to persist run metadata", "run_id", runID, "error", err)
			return nil, fmt.Errorf("failed to persist run metadata: %w", err)
		}
	}

	slog.Debug("Starting run",
		"name", run.Name,
		"test_count", len(run.Tests),
		"project_id", runContext.ProjectID,
		"source", runContext.Source,
		"branch", runContext.Branch)

	runInfo := &RunInfo{
		ID:             runID,
		Name:           run.Name,
		Status:         "RUNNING",
		StartedAt:      startTime,
		Tests:          make(map[string]*TestInfo),
		Context:        runContext,
		SuiteCleanup:   run.Cleanup,
		Vars:           cloneInterfaceMap(run.Vars),
		SuiteOpenAPI:   run.OpenAPI,
		OrganizationID: orgID,
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

			if orgID != uuid.Nil && e.runStore != nil {
				status := "FAILED"
				ended := time.Now().UTC()
				totals := &persistence.RunTotals{Total: len(run.Tests)}
				if _, updErr := e.runStore.UpdateRun(ctx, persistence.RunUpdate{
					RunID:          runID,
					OrganizationID: orgID,
					Status:         &status,
					EndedAt:        &ended,
					Totals:         totals,
				}); updErr != nil {
					slog.Error("CreateRun: failed to mark run as failed after workflow start error", "run_id", runID, "error", updErr)
				}
			}

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

	ended := time.Now().UTC()

	e.mu.Lock()
	runInfo.Status = "FAILED"
	runInfo.SuiteInitFailed = true
	runInfo.EndedAt = ended
	e.mu.Unlock()

	e.addLog(runID, fmt.Sprintf("Suite init failed: %v", initErr), "red", true)
	e.addLog(runID, "Skipping all tests because suite init failed.", "red", true)
	e.addLog(runID, fmt.Sprintf("Test run: \"%s\" ended without executing any tests.", runInfo.Name), "n/a", true)

	if runInfo.OrganizationID != uuid.Nil && e.runStore != nil {
		if _, err := e.runStore.UpdateRun(context.Background(), persistence.RunUpdate{
			RunID:          runID,
			OrganizationID: runInfo.OrganizationID,
			Status:         stringPtr("FAILED"),
			EndedAt:        timePtr(ended),
			Totals:         &persistence.RunTotals{Total: len(runInfo.Tests)},
		}); err != nil {
			slog.Error("handleSuiteInitFailure: failed to persist failure state", "run_id", runID, "error", err)
		}
	}

	e.triggerSuiteCleanup(runID, true)
}

func (e *Engine) triggerSuiteCleanup(runID string, hasFailure bool) {
	slog.Info("triggerSuiteCleanup: Starting", "run_id", runID, "has_failure", hasFailure)

	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		slog.Warn("triggerSuiteCleanup: Run not found", "run_id", runID)
		return
	}

	if runInfo.SuiteCleanup == nil {
		e.mu.Unlock()
		slog.Debug("triggerSuiteCleanup: No suite cleanup configured", "run_id", runID)
		return
	}

	if runInfo.SuiteCleanupRan {
		e.mu.Unlock()
		slog.Debug("triggerSuiteCleanup: Suite cleanup already ran", "run_id", runID)
		return
	}

	runInfo.SuiteCleanupRan = true
	cleanupSpec := runInfo.SuiteCleanup
	varsCopy := cloneInterfaceMap(runInfo.Vars)
	suiteGlobalsCopy := cloneStringMap(runInfo.SuiteGlobals)
	suiteOpenAPI := runInfo.SuiteOpenAPI
	e.mu.Unlock()

	slog.Info("triggerSuiteCleanup: Starting suite cleanup workflow", "run_id", runID)

	// Track suite cleanup workflow so server waits for completion before shutdown
	e.cleanupWg.Add(1)
	slog.Debug("triggerSuiteCleanup: Added to cleanupWg", "run_id", runID)
	go func() {
		defer func() {
			e.cleanupWg.Done()
			slog.Debug("triggerSuiteCleanup: Removed from cleanupWg (Done called)", "run_id", runID)
		}()

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

		slog.Debug("triggerSuiteCleanup: Executing suite cleanup workflow", "run_id", runID, "workflow_id", options.ID)
		execution, err := e.temporal.ExecuteWorkflow(ctx, options, "SuiteCleanupWorkflow", params)
		if err != nil {
			slog.Error("triggerSuiteCleanup: Failed to start suite cleanup workflow", "run_id", runID, "error", err)
			log.Printf("[ERROR] Failed to start suite cleanup workflow for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Failed to start suite cleanup: %v", err), "red", true)
			return
		}

		slog.Debug("triggerSuiteCleanup: Suite cleanup workflow started, waiting for completion", "run_id", runID)
		if err := execution.Get(ctx, nil); err != nil {
			slog.Error("triggerSuiteCleanup: Suite cleanup workflow failed", "run_id", runID, "error", err)
			log.Printf("[ERROR] Suite cleanup workflow failed for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Suite cleanup workflow failed: %v", err), "red", true)
			return
		}

		slog.Info("triggerSuiteCleanup: Suite cleanup completed successfully", "run_id", runID)
		e.addLog(runID, "Suite cleanup completed", "green", true)
	}()
}

func (e *Engine) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	runID := req.RunId

	_, orgID, err := e.resolvePrincipalAndOrg(stream.Context())
	if err != nil {
		return err
	}

	e.mu.RLock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}
	if orgID != uuid.Nil && runInfo.OrganizationID != uuid.Nil && runInfo.OrganizationID != orgID {
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
			if orgID != uuid.Nil && runInfo.OrganizationID != uuid.Nil && runInfo.OrganizationID != orgID {
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

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	runInfo, exists := e.runs[req.RunId]
	if !exists {
		e.mu.RUnlock()
		return nil, fmt.Errorf("run not found: %s", req.RunId)
	}
	if orgID != uuid.Nil && runInfo.OrganizationID != uuid.Nil && runInfo.OrganizationID != orgID {
		e.mu.RUnlock()
		return nil, fmt.Errorf("run not found: %s", req.RunId)
	}
	e.mu.RUnlock()

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

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	if orgID == uuid.Nil || e.runStore == nil {
		return e.listRunsInMemory(req)
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	records, err := e.runStore.ListRuns(ctx, orgID, limit)
	if err != nil {
		slog.Error("ListRuns: failed to load runs", "error", err)
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	filtered := make([]*generated.RunSummary, 0, len(records))
	for _, rec := range records {
		if req.Status != "" && !strings.EqualFold(rec.Status, req.Status) {
			continue
		}
		if req.Source != "" && !strings.EqualFold(rec.Source, req.Source) {
			continue
		}
		if req.Branch != "" && !strings.EqualFold(rec.Branch, req.Branch) {
			continue
		}
		if req.ScheduleName != "" && !strings.EqualFold(rec.ScheduleName, req.ScheduleName) {
			continue
		}

		filtered = append(filtered, mapRunRecordToSummary(rec))
	}

	sortRuns(filtered, req.OrderBy, !req.Descending)

	result := &generated.ListRunsResponse{
		Runs:       filtered,
		TotalCount: int32(len(filtered)),
	}

	slog.Debug("Returning ListRuns response", "runs_count", len(filtered))
	return result, nil
}

func (e *Engine) GetRun(ctx context.Context, req *generated.GetRunRequest) (*generated.GetRunResponse, error) {
	slog.Debug("GetRun called", "run_id", req.RunId)

	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	if runInfo, exists := e.runs[req.RunId]; exists {
		if orgID == uuid.Nil || runInfo.OrganizationID == orgID {
			e.mu.RUnlock()
			slog.Debug("Found active run in memory", "run_id", req.RunId)
			return mapRunInfoToRunDetails(runInfo), nil
		}
	}
	e.mu.RUnlock()

	if orgID == uuid.Nil || e.runStore == nil {
		return e.getRunInMemory(req.RunId)
	}

	rec, err := e.runStore.GetRun(ctx, orgID, req.RunId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("run not found: %s", req.RunId)
		}
		slog.Error("GetRun: failed to load run", "run_id", req.RunId, "error", err)
		return nil, fmt.Errorf("failed to load run: %w", err)
	}

	return mapRunRecordToRunDetails(rec), nil
}

// CancelRun cancels all workflows for a given run and marks it as cancelled
func (e *Engine) CancelRun(ctx context.Context, req *generated.CancelRunRequest) (*generated.CancelRunResponse, error) {
	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	slog.Info("CancelRun: Starting cancellation", "run_id", req.RunId)

	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing authentication context")
	}

	orgIDStr := strings.TrimSpace(principal.OrgID)
	if orgIDStr == "" {
		return nil, fmt.Errorf("token missing organization scope")
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid organization identifier: %w", err)
	}

	e.mu.Lock()
	runInfo, exists := e.runs[req.RunId]
	if !exists {
		e.mu.Unlock()
		slog.Warn("CancelRun: Run not found", "run_id", req.RunId)
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("run not found: %s", req.RunId),
		}, nil
	}
	if runInfo.OrganizationID != orgID {
		e.mu.Unlock()
		slog.Warn("CancelRun: Run not accessible for caller", "run_id", req.RunId)
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("run not found: %s", req.RunId),
		}, nil
	}

	runInfo.Status = "CANCELLED"
	slog.Debug("CancelRun: Marked run as CANCELLED", "run_id", req.RunId)

	testWorkflows := make([]string, 0, len(runInfo.Tests))
	for workflowID, testInfo := range runInfo.Tests {
		testWorkflows = append(testWorkflows, workflowID)
		slog.Debug("CancelRun: Found test workflow to cancel", "workflow_id", workflowID, "test_name", testInfo.Name, "status", testInfo.Status)
	}
	e.mu.Unlock()

	slog.Info("CancelRun: Cancelling workflows", "run_id", req.RunId, "workflow_count", len(testWorkflows))

	var cancelErrors []string
	for _, workflowID := range testWorkflows {
		slog.Debug("CancelRun: Attempting to cancel workflow", "workflow_id", workflowID)
		if err := e.temporal.CancelWorkflow(ctx, workflowID, ""); err != nil {
			slog.Warn("CancelRun: Failed to cancel workflow", "workflow_id", workflowID, "error", err)
			cancelErrors = append(cancelErrors, fmt.Sprintf("workflow %s: %v", workflowID, err))
			continue
		}
		slog.Info("CancelRun: Successfully cancelled workflow", "workflow_id", workflowID)

		slog.Debug("CancelRun: Waiting for workflow to complete cleanup", "workflow_id", workflowID)
		workflowRun := e.temporal.GetWorkflow(ctx, workflowID, "")
		var result interface{}
		if err := workflowRun.Get(ctx, &result); err != nil {
			slog.Debug("CancelRun: Workflow completed with cancellation", "workflow_id", workflowID, "error", err)
		} else {
			slog.Debug("CancelRun: Workflow completed successfully", "workflow_id", workflowID)
		}
	}

	e.addLog(req.RunId, "Run cancelled by user (Ctrl+C)", "yellow", true)
	slog.Debug("CancelRun: Triggering suite cleanup", "run_id", req.RunId)
	e.triggerSuiteCleanup(req.RunId, true)

	ended := time.Now().UTC()
	e.mu.Lock()
	if runInfo, ok := e.runs[req.RunId]; ok {
		runInfo.EndedAt = ended
	}
	e.mu.Unlock()

	counts, err := e.getTestStatusCounts(req.RunId)
	if err != nil {
		slog.Warn("CancelRun: unable to compute test counts", "run_id", req.RunId, "error", err)
		counts = TestStatusCounts{}
	}

	if orgID != uuid.Nil && e.runStore != nil {
		if _, updErr := e.runStore.UpdateRun(ctx, persistence.RunUpdate{
			RunID:          req.RunId,
			OrganizationID: orgID,
			Status:         stringPtr("CANCELLED"),
			EndedAt:        timePtr(ended),
			Totals:         makeRunTotals(counts),
		}); updErr != nil {
			slog.Error("CancelRun: failed to persist cancellation", "run_id", req.RunId, "error", updErr)
		}
	}

	if len(cancelErrors) > 0 {
		slog.Warn("CancelRun: Completed with errors", "run_id", req.RunId, "errors", cancelErrors)
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("cancelled with errors: %s", strings.Join(cancelErrors, "; ")),
		}, nil
	}

	slog.Info("CancelRun: Completed successfully", "run_id", req.RunId)
	return &generated.CancelRunResponse{
		Success: true,
		Message: "run cancelled successfully",
	}, nil
}

// WaitForCleanup waits for all suite cleanup workflows to complete
// This should be called before server shutdown to ensure cleanup completes
func (e *Engine) WaitForCleanup(ctx context.Context, req *generated.WaitForCleanupRequest) (*generated.WaitForCleanupResponse, error) {
	slog.Info("WaitForCleanup: Waiting for cleanup workflows to complete")

	// Wait for all cleanup workflows with a timeout
	done := make(chan struct{})
	go func() {
		slog.Debug("WaitForCleanup: Calling cleanupWg.Wait()")
		e.cleanupWg.Wait()
		slog.Debug("WaitForCleanup: cleanupWg.Wait() returned - all cleanups done")
		close(done)
	}()

	select {
	case <-done:
		slog.Info("WaitForCleanup: All cleanup workflows completed successfully")
		return &generated.WaitForCleanupResponse{Completed: true}, nil
	case <-ctx.Done():
		slog.Warn("WaitForCleanup: Context cancelled/timed out before cleanup completed", "error", ctx.Err())
		return &generated.WaitForCleanupResponse{Completed: false}, ctx.Err()
	}
}

func determineInitiator(principal *Principal) string {
	if principal == nil {
		return "unknown"
	}
	if email := strings.TrimSpace(principal.Email); email != "" {
		return email
	}
	if subject := strings.TrimSpace(principal.Subject); subject != "" {
		return subject
	}
	return "unknown"
}

func detectConfigSource(ctx *RunContext) string {
	if ctx == nil {
		return "repo_commit"
	}
	if strings.TrimSpace(ctx.CommitSHA) == "" {
		return "uncommitted"
	}
	return "repo_commit"
}

func detectEnvironment(ctx *RunContext) string {
	if ctx == nil {
		return ""
	}
	if ctx.Metadata == nil {
		return ""
	}
	keys := []string{"environment", "env", "rs_environment"}
	for _, key := range keys {
		if val, ok := ctx.Metadata[key]; ok {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func makeNullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func stringPtr(value string) *string {
	if value == "" {
		return new(string)
	}
	v := value
	return &v
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return &time.Time{}
	}
	val := t
	return &val
}

func mapRunRecordToSummary(rec persistence.RunRecord) *generated.RunSummary {
	start := rec.StartedAt.Time
	if !rec.StartedAt.Valid {
		start = rec.CreatedAt
	}

	var (
		endedAt  string
		duration int64
		endTime  time.Time
	)
	if rec.EndedAt.Valid {
		endTime = rec.EndedAt.Time
		endedAt = endTime.Format(time.RFC3339)
		if !start.IsZero() {
			duration = endTime.Sub(start).Milliseconds()
		}
	}

	commitSha := ""
	if rec.CommitSHA.Valid {
		commitSha = rec.CommitSHA.String
	}

	context := &generated.RunContext{
		Source:       rec.Source,
		Branch:       rec.Branch,
		CommitSha:    commitSha,
		Trigger:      rec.Trigger,
		ScheduleName: rec.ScheduleName,
		Metadata:     map[string]string{},
	}
	if rec.ProjectID.Valid {
		context.ProjectId = rec.ProjectID.UUID.String()
	}

	startedAt := ""
	if !start.IsZero() {
		startedAt = start.Format(time.RFC3339)
	}

	return &generated.RunSummary{
		RunId:        rec.ID,
		SuiteName:    rec.SuiteName,
		Status:       rec.Status,
		StartedAt:    startedAt,
		EndedAt:      endedAt,
		DurationMs:   duration,
		TotalTests:   int32(rec.TotalTests),
		PassedTests:  int32(rec.PassedTests),
		FailedTests:  int32(rec.FailedTests),
		TimeoutTests: int32(rec.TimeoutTests),
		Context:      context,
	}
}

func mapRunRecordToRunDetails(rec persistence.RunRecord) *generated.GetRunResponse {
	start := rec.StartedAt.Time
	if !rec.StartedAt.Valid {
		start = rec.CreatedAt
	}

	startedAt := ""
	if !start.IsZero() {
		startedAt = start.Format(time.RFC3339)
	}

	var (
		endedAt  string
		duration int64
	)
	if rec.EndedAt.Valid {
		end := rec.EndedAt.Time
		endedAt = end.Format(time.RFC3339)
		if !start.IsZero() {
			duration = end.Sub(start).Milliseconds()
		}
	}

	commitSha := ""
	if rec.CommitSHA.Valid {
		commitSha = rec.CommitSHA.String
	}

	details := &generated.RunDetails{
		RunId:      rec.ID,
		SuiteName:  rec.SuiteName,
		Status:     rec.Status,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		DurationMs: duration,
		Context: &generated.RunContext{
			Source:       rec.Source,
			Branch:       rec.Branch,
			CommitSha:    commitSha,
			Trigger:      rec.Trigger,
			ScheduleName: rec.ScheduleName,
			Metadata:     map[string]string{},
		},
		Tests: []*generated.TestDetails{},
	}
	if rec.ProjectID.Valid {
		details.Context.ProjectId = rec.ProjectID.UUID.String()
	}

	return &generated.GetRunResponse{Run: details}
}

func makeRunTotals(counts TestStatusCounts) *persistence.RunTotals {
	return &persistence.RunTotals{
		Total:   counts.Total,
		Passed:  counts.Passed,
		Failed:  counts.Failed,
		Timeout: counts.TimedOut,
	}
}

func (e *Engine) resolvePrincipalAndOrg(ctx context.Context) (*Principal, uuid.UUID, error) {
	principal, ok := PrincipalFromContext(ctx)
	if e.authConfig.mode == authModeNone {
		return principal, uuid.Nil, nil
	}
	if !ok {
		return nil, uuid.Nil, fmt.Errorf("missing authentication context")
	}
	orgIDStr := strings.TrimSpace(principal.OrgID)
	if orgIDStr == "" {
		return nil, uuid.Nil, fmt.Errorf("token missing organization scope")
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("invalid organization identifier: %w", err)
	}
	return principal, orgID, nil
}

func (e *Engine) listRunsInMemory(req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
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
		if req.Branch != "" && !strings.EqualFold(runInfo.Context.Branch, req.Branch) {
			continue
		}
		if req.ScheduleName != "" && runInfo.Context.ScheduleName != req.ScheduleName {
			continue
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

		duration := int64(0)
		if !runInfo.EndedAt.IsZero() {
			duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
		}

		runs = append(runs, &generated.RunSummary{
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
		})
	}

	sortRuns(runs, req.OrderBy, !req.Descending)

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(runs) > int(limit) {
		runs = runs[:limit]
	}

	return &generated.ListRunsResponse{
		Runs:       runs,
		TotalCount: int32(len(runs)),
	}, nil
}

func (e *Engine) getRunInMemory(runID string) (*generated.GetRunResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if runInfo, exists := e.runs[runID]; exists {
		slog.Debug("Found active run in memory", "run_id", runID)
		return mapRunInfoToRunDetails(runInfo), nil
	}

	if len(runID) <= 12 {
		slog.Debug("Searching for run by prefix", "prefix", runID)
		for fullID, runInfo := range e.runs {
			if strings.HasPrefix(fullID, runID) {
				slog.Debug("Found run by prefix match", "prefix", runID, "full_id", fullID)
				return mapRunInfoToRunDetails(runInfo), nil
			}
		}
	}

	return nil, fmt.Errorf("run not found: %s", runID)
}
