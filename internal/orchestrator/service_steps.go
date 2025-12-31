package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// UpsertRunStep handles the UpsertRunStep RPC for step reporting
func (e *Engine) UpsertRunStep(ctx context.Context, req *generated.UpsertRunStepRequest) (*generated.UpsertRunStepResponse, error) {
	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	if req.WorkflowId == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	if req.StepName == "" {
		return nil, fmt.Errorf("step_name is required")
	}
	if req.Plugin == "" {
		return nil, fmt.Errorf("plugin is required")
	}

	// Use internal callback resolver to allow service accounts without org scope
	_, orgID, err := e.resolvePrincipalAndOrgForInternalCallbacks(ctx)
	if err != nil {
		return nil, err
	}

	// Verify run exists and belongs to org
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

	// Check if we have a run store (only when running with controlplane)
	if e.runStore == nil {
		slog.Debug("UpsertRunStep: no run store available, skipping persistence")
		return &generated.UpsertRunStepResponse{StepId: ""}, nil
	}

	// Resolve run_test_id from workflow_id
	runTest, err := e.runStore.GetRunTestByWorkflowID(ctx, req.WorkflowId)
	if err != nil {
		slog.Error("UpsertRunStep: failed to resolve run test", "workflow_id", req.WorkflowId, "error", err)
		return nil, fmt.Errorf("failed to resolve run test from workflow_id: %w", err)
	}

	// Parse timestamps
	var startedAt, endedAt sql.NullTime
	if req.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, req.StartedAt); err == nil {
			startedAt = sql.NullTime{Time: t, Valid: true}
		}
	}
	if req.EndedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, req.EndedAt); err == nil {
			endedAt = sql.NullTime{Time: t, Valid: true}
		}
	}

	// Parse JSON data
	var requestData, responseData map[string]interface{}
	if len(req.RequestJson) > 0 {
		if err := json.Unmarshal(req.RequestJson, &requestData); err != nil {
			slog.Warn("UpsertRunStep: failed to parse request_json", "error", err)
		}
	}
	if len(req.ResponseJson) > 0 {
		if err := json.Unmarshal(req.ResponseJson, &responseData); err != nil {
			slog.Warn("UpsertRunStep: failed to parse response_json", "error", err)
		}
	}

	// Parse extended data for rich step details
	var assertionsData []persistence.AssertionResult
	if len(req.AssertionsJson) > 0 {
		if err := json.Unmarshal(req.AssertionsJson, &assertionsData); err != nil {
			slog.Warn("UpsertRunStep: failed to parse assertions_json", "error", err)
		}
	}
	var variablesData []persistence.SavedVariable
	if len(req.VariablesJson) > 0 {
		if err := json.Unmarshal(req.VariablesJson, &variablesData); err != nil {
			slog.Warn("UpsertRunStep: failed to parse variables_json", "error", err)
		}
	}
	var stepConfig map[string]interface{}
	if len(req.StepConfigJson) > 0 {
		if err := json.Unmarshal(req.StepConfigJson, &stepConfig); err != nil {
			slog.Warn("UpsertRunStep: failed to parse step_config_json", "error", err)
		}
	}

	// Build step record
	step := persistence.RunStep{
		RunTestID:        runTest.ID,
		StepIndex:        int(req.StepIndex),
		Name:             req.StepName,
		Plugin:           req.Plugin,
		Status:           req.Status,
		AssertionsPassed: int(req.AssertionsPassed),
		AssertionsFailed: int(req.AssertionsFailed),
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		RequestData:      requestData,
		ResponseData:     responseData,
		AssertionsData:   assertionsData,
		VariablesData:    variablesData,
		StepConfig:       stepConfig,
	}

	if req.ErrorMessage != "" {
		step.ErrorMessage = sql.NullString{String: req.ErrorMessage, Valid: true}
	}
	if req.DurationMs > 0 {
		step.DurationMs = sql.NullInt64{Int64: req.DurationMs, Valid: true}
	}

	// Upsert the step
	upsertedStep, err := e.runStore.UpsertRunStep(ctx, step)
	if err != nil {
		slog.Error("UpsertRunStep: failed to upsert step", "error", err)
		return nil, fmt.Errorf("failed to upsert run step: %w", err)
	}

	// Update step counts on the run test
	if err := e.runStore.UpdateRunTestStepCounts(ctx, runTest.ID); err != nil {
		slog.Warn("UpsertRunStep: failed to update step counts", "run_test_id", runTest.ID, "error", err)
	}

	return &generated.UpsertRunStepResponse{
		StepId: upsertedStep.ID.String(),
	}, nil
}
