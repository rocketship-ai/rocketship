package interpreter

import (
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	// plugins
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
	"github.com/rocketship-ai/rocketship/internal/plugins/script"
)

func TestWorkflow(ctx workflow.Context, test dsl.Test, vars map[string]interface{}) error {
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

		switch step.Plugin {
		case "delay":
			if err := handleDelayStep(ctx, step); err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}

		case "http":
			if err := handleHTTPStep(ctx, step, state); err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}

		case "script":
			if err := handleScriptStep(ctx, step, state, vars); err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}

		default:
			return fmt.Errorf("step %s: unknown plugin %s", step.Name, step.Plugin)
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

func handleHTTPStep(ctx workflow.Context, step dsl.Step, state map[string]string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info(fmt.Sprintf("Executing HTTP step: %s", step.Name))
	logger.Info(fmt.Sprintf("Current state: %v", state))

	// Pass raw step data to activity
	pluginParams := map[string]interface{}{
		"name":       step.Name,
		"plugin":     step.Plugin,
		"config":     step.Config,
		"assertions": step.Assertions,
		"save":       step.Save,
		"state":      state,
	}

	// Create HTTP plugin instance for activity execution
	httpPlugin := &http.HTTPPlugin{}

	var activityResp http.ActivityResponse
	err := workflow.ExecuteActivity(ctx, httpPlugin.Activity, pluginParams).Get(ctx, &activityResp)
	if err != nil {
		logger.Error("HTTP activity failed", "error", err)
		return fmt.Errorf("http activity error: %w", err)
	}

	// Update workflow state with saved values
	logger.Info(fmt.Sprintf("Saved values from step: %v", activityResp.Saved))
	keys := workflow.DeterministicKeys(activityResp.Saved)
	for _, key := range keys {
		state[key] = activityResp.Saved[key]
		logger.Info(fmt.Sprintf("Updated state[%s] = %s", key, state[key]))
	}
	logger.Info(fmt.Sprintf("Updated state: %v", state))

	return nil
}

func handleScriptStep(ctx workflow.Context, step dsl.Step, state map[string]string, vars map[string]interface{}) error {
	logger := workflow.GetLogger(ctx)
	logger.Info(fmt.Sprintf("Executing script step: %s", step.Name))
	logger.Info(fmt.Sprintf("Current state: %v", state))

	// Pass raw step data to activity
	pluginParams := map[string]interface{}{
		"name":   step.Name,
		"plugin": step.Plugin,
		"config": step.Config,
		"state":  state,
		"vars":   vars,
	}

	// Create script plugin instance for activity execution
	scriptPlugin := &script.ScriptPlugin{}

	var activityResp interface{}
	err := workflow.ExecuteActivity(ctx, scriptPlugin.Activity, pluginParams).Get(ctx, &activityResp)
	if err != nil {
		logger.Error("Script activity failed", "error", err)
		return fmt.Errorf("script activity error: %w", err)
	}

	// Handle response - it comes back as map[string]interface{} due to JSON serialization
	var savedValues map[string]string
	if respMap, ok := activityResp.(map[string]interface{}); ok {
		if savedInterface, exists := respMap["saved"]; exists {
			if savedMap, ok := savedInterface.(map[string]interface{}); ok {
				savedValues = make(map[string]string)
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
	} else {
		return fmt.Errorf("unexpected response type from script activity: %T", activityResp)
	}

	// Update workflow state with saved values
	logger.Info(fmt.Sprintf("Saved values from script: %v", savedValues))
	keys := workflow.DeterministicKeys(savedValues)
	for _, key := range keys {
		state[key] = savedValues[key]
		logger.Info(fmt.Sprintf("Updated state[%s] = %s", key, state[key]))
	}
	logger.Info(fmt.Sprintf("Updated state: %v", state))

	return nil
}
