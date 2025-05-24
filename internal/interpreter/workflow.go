package interpreter

import (
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	// plugins
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
)

func TestWorkflow(ctx workflow.Context, test dsl.Test) error {
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
