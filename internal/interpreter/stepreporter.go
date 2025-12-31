package interpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rocketship-ai/rocketship/internal/cli"
	"go.temporal.io/sdk/activity"
)

// StepReportParams contains parameters for reporting step execution results
type StepReportParams struct {
	RunID            string                 `json:"run_id"`
	WorkflowID       string                 `json:"workflow_id"`
	StepIndex        int                    `json:"step_index"`
	StepName         string                 `json:"step_name"`
	Plugin           string                 `json:"plugin"`
	Status           string                 `json:"status"` // PENDING, RUNNING, PASSED, FAILED
	ErrorMessage     string                 `json:"error_message,omitempty"`
	StartedAt        string                 `json:"started_at,omitempty"`
	EndedAt          string                 `json:"ended_at,omitempty"`
	DurationMs       int64                  `json:"duration_ms,omitempty"`
	AssertionsPassed int                    `json:"assertions_passed"`
	AssertionsFailed int                    `json:"assertions_failed"`
	RequestData      map[string]interface{} `json:"request_data,omitempty"`
	ResponseData     map[string]interface{} `json:"response_data,omitempty"`
}

// StepReporterActivity forwards step execution reports to the engine for persistence
func StepReporterActivity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Extract required parameters
	runID, ok := params["run_id"].(string)
	if !ok || runID == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	stepName, ok := params["step_name"].(string)
	if !ok || stepName == "" {
		return nil, fmt.Errorf("step_name is required")
	}

	plugin, ok := params["plugin"].(string)
	if !ok || plugin == "" {
		return nil, fmt.Errorf("plugin is required")
	}

	status, ok := params["status"].(string)
	if !ok || status == "" {
		return nil, fmt.Errorf("status is required")
	}

	// Extract optional parameters
	stepIndex := 0
	if idx, ok := params["step_index"].(float64); ok {
		stepIndex = int(idx)
	}

	errorMessage, _ := params["error_message"].(string)
	startedAt, _ := params["started_at"].(string)
	endedAt, _ := params["ended_at"].(string)

	var durationMs int64
	if dur, ok := params["duration_ms"].(float64); ok {
		durationMs = int64(dur)
	}

	var assertionsPassed, assertionsFailed int
	if ap, ok := params["assertions_passed"].(float64); ok {
		assertionsPassed = int(ap)
	}
	if af, ok := params["assertions_failed"].(float64); ok {
		assertionsFailed = int(af)
	}

	// Extract request/response data (for HTTP plugin)
	var requestJSON, responseJSON []byte
	if reqData, ok := params["request_data"].(map[string]interface{}); ok && len(reqData) > 0 {
		if data, err := json.Marshal(reqData); err == nil {
			requestJSON = data
		}
	}
	if respData, ok := params["response_data"].(map[string]interface{}); ok && len(respData) > 0 {
		if data, err := json.Marshal(respData); err == nil {
			responseJSON = data
		}
	}

	// Extract assertions_data, variables_data, step_config_data for rich step details
	var assertionsJSON, variablesJSON, stepConfigJSON []byte
	if assertionsData, ok := params["assertions_data"].([]interface{}); ok && len(assertionsData) > 0 {
		if data, err := json.Marshal(assertionsData); err == nil {
			assertionsJSON = data
		}
	}
	if variablesData, ok := params["variables_data"].([]interface{}); ok && len(variablesData) > 0 {
		if data, err := json.Marshal(variablesData); err == nil {
			variablesJSON = data
		}
	}
	if stepConfigData, ok := params["step_config"].(map[string]interface{}); ok && len(stepConfigData) > 0 {
		if data, err := json.Marshal(stepConfigData); err == nil {
			stepConfigJSON = data
		}
	}

	// Get engine address from environment or use default
	engineAddr := os.Getenv(EnvEngineGRPCAddr)
	if engineAddr == "" {
		engineAddr = "localhost:7700"
	}

	// Create engine client
	client, err := cli.NewEngineClient(engineAddr)
	if err != nil {
		logger.Error("Failed to create engine client", "error", err)
		return nil, fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Send step report to engine
	stepID, err := client.UpsertRunStep(ctx, cli.UpsertRunStepRequest{
		RunID:            runID,
		WorkflowID:       workflowID,
		StepIndex:        stepIndex,
		StepName:         stepName,
		Plugin:           plugin,
		Status:           status,
		ErrorMessage:     errorMessage,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		DurationMs:       durationMs,
		AssertionsPassed: assertionsPassed,
		AssertionsFailed: assertionsFailed,
		RequestJSON:      requestJSON,
		ResponseJSON:     responseJSON,
		AssertionsJSON:   assertionsJSON,
		VariablesJSON:    variablesJSON,
		StepConfigJSON:   stepConfigJSON,
	})
	if err != nil {
		logger.Error("Failed to send step report to engine", "error", err)
		return nil, fmt.Errorf("failed to send step report to engine: %w", err)
	}

	logger.Debug("Successfully forwarded step report to engine",
		"run_id", runID,
		"step_name", stepName,
		"status", status,
		"step_id", stepID)

	return map[string]interface{}{
		"step_id": stepID,
	}, nil
}
