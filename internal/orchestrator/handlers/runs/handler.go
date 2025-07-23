package runs

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/types"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/utils"
	"go.temporal.io/sdk/client"
)

// Handler manages test run operations
type Handler struct {
	temporal client.Client
	runs     map[string]*types.RunInfo
	mu       sync.RWMutex
}

// NewHandler creates a new run handler
func NewHandler(temporal client.Client) *Handler {
	return &Handler{
		temporal: temporal,
		runs:     make(map[string]*types.RunInfo),
	}
}

// CreateRun creates a new test run
func (h *Handler) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	// Validate input
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if len(req.YamlPayload) == 0 {
		return nil, fmt.Errorf("YAML payload cannot be empty")
	}

	slog.Debug("CreateRun called", "payload_size", len(req.YamlPayload))

	runID, err := utils.GenerateID()
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

	runInfo := &types.RunInfo{
		ID:        runID,
		Name:      run.Name,
		Status:    "PENDING",
		StartedAt: time.Now(),
		Tests:     make(map[string]*types.TestInfo),
		Context:   runContext,
		Logs: []types.LogLine{
			{
				Msg:   fmt.Sprintf("Starting test run \"%s\"... 🚀 [%s/%s]", run.Name, runContext.ProjectID, runContext.Source),
				Color: "purple",
				Bold:  true,
			},
		},
	}

	for _, test := range run.Tests {
		testID, err := utils.GenerateID()
		if err != nil {
			log.Printf("[ERROR] Failed to generate test ID: %v", err)
			return nil, fmt.Errorf("failed to generate test ID: %w", err)
		}
		testInfo := &types.TestInfo{
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
		}

		slog.Debug("Starting workflow with search attributes",
			"workflow_id", testID,
			"project_id", runContext.ProjectID,
			"suite_name", run.Name,
			"source", runContext.Source)

		execution, err := h.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test, run.Vars, runID)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		h.mu.Lock()
		runInfo.Tests[testID] = testInfo
		// add a log line for the start of the test
		runInfo.Logs = append(runInfo.Logs, types.LogLine{
			Msg:   fmt.Sprintf("Running test: \"%s\"...", test.Name),
			Color: "n/a",
			Bold:  false,
		})
		h.mu.Unlock()

		go h.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())
	}

	h.mu.Lock()
	h.runs[runID] = runInfo
	h.mu.Unlock()

	return &generated.CreateRunResponse{
		RunId: runID,
	}, nil
}

// StreamLogs streams logs for a test run
func (h *Handler) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	runID := req.RunId

	// Initial validation and log copy
	h.mu.RLock()
	runInfo, exists := h.runs[runID]
	if !exists {
		h.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}

	logs := make([]types.LogLine, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	h.mu.RUnlock()

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
			h.mu.RLock()
			runInfo, exists := h.runs[runID]
			if !exists {
				h.mu.RUnlock()
				return fmt.Errorf("run not found: %s", runID)
			}

			var newLogs []types.LogLine
			status := runInfo.Status

			if len(runInfo.Logs) > lastLogIndex {
				newLogs = make([]types.LogLine, len(runInfo.Logs)-lastLogIndex)
				copy(newLogs, runInfo.Logs[lastLogIndex:])
				lastLogIndex = len(runInfo.Logs)
			}
			h.mu.RUnlock()

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

// AddLog adds a log message to a run
func (h *Handler) AddLog(ctx context.Context, req *generated.AddLogRequest) (*generated.AddLogResponse, error) {
	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	h.addLogWithContext(req.RunId, req.Message, req.Color, req.Bold, req.TestName, req.StepName)
	return &generated.AddLogResponse{}, nil
}

// addLog adds a log message without test/step context
func (h *Handler) addLog(runID, message, color string, bold bool) {
	h.addLogWithContext(runID, message, color, bold, "", "")
}

// addLogWithContext adds a log message with optional test/step context
func (h *Handler) addLogWithContext(runID, message, color string, bold bool, testName, stepName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	runInfo, exists := h.runs[runID]
	if !exists {
		log.Printf("[WARN] Run %s not found when trying to add log", runID)
		return
	}

	runInfo.Logs = append(runInfo.Logs, types.LogLine{
		Msg:      message,
		Color:    color,
		Bold:     bold,
		TestName: testName,
		StepName: stepName,
	})
}

// GetRuns returns the runs map for other handlers to access
func (h *Handler) GetRuns() map[string]*types.RunInfo {
	return h.runs
}

// GetRunsMutex returns the mutex for safe access to runs
func (h *Handler) GetRunsMutex() *sync.RWMutex {
	return &h.mu
}