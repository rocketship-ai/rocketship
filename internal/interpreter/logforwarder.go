package interpreter

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/cli"
	"go.temporal.io/sdk/activity"
)

// LogForwarderActivity forwards log messages from plugins to the engine
func LogForwarderActivity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	
	// Extract parameters
	runID, ok := params["run_id"].(string)
	if !ok || runID == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	
	workflowID, ok := params["workflow_id"].(string) 
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	
	message, ok := params["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("message is required")
	}
	
	color, _ := params["color"].(string)
	bold, _ := params["bold"].(bool)
	
	// Get engine address from environment or use default
	engineAddr := "localhost:7700" // TODO: Make this configurable
	
	// Create engine client
	client, err := cli.NewEngineClient(engineAddr)
	if err != nil {
		logger.Error("Failed to create engine client", "error", err)
		return nil, fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()
	
	// Send log message to engine
	if err := client.AddLog(ctx, runID, workflowID, message, color, bold); err != nil {
		logger.Error("Failed to send log to engine", "error", err)
		return nil, fmt.Errorf("failed to send log to engine: %w", err)
	}
	
	logger.Debug("Successfully forwarded log message to engine", "run_id", runID, "message", message)
	
	return map[string]interface{}{
		"forwarded": true,
	}, nil
}