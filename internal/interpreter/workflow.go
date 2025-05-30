package interpreter

import (
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func TestWorkflow(ctx workflow.Context, test dsl.Test, vars map[string]interface{}, runID string) error {
	logger := workflow.GetLogger(ctx)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 30, // TODO: Make this configurable
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
			// BackoffCoefficient: 2,
			// MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Initialize workflow state
	state := make(map[string]string)
	logger.Info("Initialized workflow state", "state", state)

	for i, step := range test.Steps {
		logger.Info(fmt.Sprintf("Starting step %d: %q", i, step.Name))
		logger.Info(fmt.Sprintf("State before step %d: %v", i, state))

		// Handle delay plugin with workflow sleep (special case)
		if step.Plugin == "delay" {
			if err := handleDelayStep(ctx, step); err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}
		} else {
			// Execute plugin through registry
			if err := executePlugin(ctx, step, state, vars, runID); err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}
		}

		logger.Info(fmt.Sprintf("Step %q PASSED", step.Name))
		logger.Info(fmt.Sprintf("State after step %d: %v", i, state))
	}

	return nil
}

func handleDelayStep(ctx workflow.Context, step dsl.Step) error {
	// Extract duration directly from step config
	durationStr, ok := step.Config["duration"].(string)
	if !ok {
		return fmt.Errorf("step %q: duration is required and must be a string", step.Name)
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("step %q: invalid duration format: %w", step.Name, err)
	}

	return workflow.Sleep(ctx, duration)
}

// executePlugin executes any registered plugin through the plugin registry
func executePlugin(ctx workflow.Context, step dsl.Step, state map[string]string, vars map[string]interface{}, runID string) error {
	logger := workflow.GetLogger(ctx)

	// Check if plugin is registered
	_, exists := plugins.GetPlugin(step.Plugin)
	if !exists {
		return fmt.Errorf("unknown plugin: %s", step.Plugin)
	}

	logger.Info(fmt.Sprintf("Executing %s plugin step: %s", step.Plugin, step.Name))
	logger.Info(fmt.Sprintf("Current state: %v", state))

	// Build plugin parameters
	pluginParams := map[string]interface{}{
		"name":   step.Name,
		"plugin": step.Plugin,
		"config": step.Config,
		"state":  state,
	}

	// Add additional parameters based on plugin type
	if step.Assertions != nil {
		pluginParams["assertions"] = step.Assertions
	}
	if step.Save != nil {
		pluginParams["save"] = step.Save
	}
	// Pass vars for script plugin usage (other plugins ignore them since CLI processes config vars)
	if vars != nil {
		pluginParams["vars"] = vars
	}

	// Execute the plugin activity
	var activityResp interface{}
	err := workflow.ExecuteActivity(ctx, step.Plugin, pluginParams).Get(ctx, &activityResp)
	if err != nil {
		logger.Error("Plugin activity failed", "plugin", step.Plugin, "error", err)
		return fmt.Errorf("%s activity error: %w", step.Plugin, err)
	}

	// Update workflow state with saved values (if any)
	if activityResp != nil {
		savedValues := extractSavedValues(activityResp)
		if len(savedValues) > 0 {
			logger.Info(fmt.Sprintf("Saved values from %s plugin: %v", step.Plugin, savedValues))
			keys := workflow.DeterministicKeys(savedValues)
			for _, key := range keys {
				state[key] = savedValues[key]
				logger.Info(fmt.Sprintf("Updated state[%s] = %s", key, state[key]))
			}
		}

		// Handle log messages from log plugin
		if step.Plugin == "log" {
			if err := forwardLogMessage(ctx, activityResp, runID); err != nil {
				logger.Warn("Failed to forward log message", "error", err)
				// Don't fail the workflow if log forwarding fails
			}
		}
	}

	logger.Info(fmt.Sprintf("Updated state: %v", state))
	return nil
}

// extractSavedValues extracts saved values from plugin response using deterministic iteration
func extractSavedValues(response interface{}) map[string]string {
	savedValues := make(map[string]string)

	// Handle response - it comes back as map[string]interface{} due to JSON serialization
	if respMap, ok := response.(map[string]interface{}); ok {
		// Check for saved values in response
		if savedInterface, exists := respMap["saved"]; exists {
			if savedMap, ok := savedInterface.(map[string]interface{}); ok {
				// Use deterministic keys for Temporal workflow compliance
				keys := workflow.DeterministicKeys(savedMap)
				for _, k := range keys {
					v := savedMap[k]
					if strVal, ok := v.(string); ok {
						savedValues[k] = strVal
					} else {
						savedValues[k] = fmt.Sprintf("%v", v)
					}
				}
			}
		}

		// For HTTP plugin compatibility - check for "Saved" field
		if savedInterface, exists := respMap["Saved"]; exists {
			if savedMap, ok := savedInterface.(map[string]interface{}); ok {
				// Use deterministic keys for Temporal workflow compliance
				keys := workflow.DeterministicKeys(savedMap)
				for _, k := range keys {
					v := savedMap[k]
					if strVal, ok := v.(string); ok {
						savedValues[k] = strVal
					} else {
						savedValues[k] = fmt.Sprintf("%v", v)
					}
				}
			}
		}
	}

	return savedValues
}

// forwardLogMessage forwards log messages from log plugin to the engine
func forwardLogMessage(ctx workflow.Context, activityResp interface{}, runID string) error {
	if respMap, ok := activityResp.(map[string]interface{}); ok {
		logMessage, hasMessage := respMap["log_message"].(string)
		logColor, _ := respMap["log_color"].(string)
		logBold, _ := respMap["log_bold"].(bool)

		if hasMessage && logMessage != "" {
			// Get workflow info to extract run ID and workflow ID
			workflowInfo := workflow.GetInfo(ctx)

			// Prepare parameters for log forwarder activity
			forwarderParams := map[string]interface{}{
				"run_id":      runID,
				"workflow_id": workflowInfo.WorkflowExecution.ID,
				"message":     logMessage,
				"color":       logColor,
				"bold":        logBold,
			}

			// Execute log forwarder activity
			var forwarderResp interface{}
			return workflow.ExecuteActivity(ctx, "LogForwarderActivity", forwarderParams).Get(ctx, &forwarderResp)
		}
	}
	return nil
}
