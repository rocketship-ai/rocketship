package interpreter

import (
	"fmt"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ExtractCleanError removes Temporal error wrapping duplication.
// Temporal wraps errors multiple times with "(type: wrapError, retryable: true):" markers,
// causing the same error to appear 2-3 times. We truncate at the first marker.
func ExtractCleanError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Truncate at first Temporal wrap marker to avoid showing the same error multiple times
	marker := " (type: wrapError, retryable: true):"
	if idx := strings.Index(errMsg, marker); idx != -1 {
		return strings.TrimSpace(errMsg[:idx])
	}

	return errMsg
}

type stepPhase string

const (
	phaseMain           stepPhase = "main"
	phaseInit           stepPhase = "init"
	phaseCleanupAlways  stepPhase = "cleanup_always"
	phaseCleanupFailure stepPhase = "cleanup_on_failure"
)

func (p stepPhase) displayName() string {
	switch p {
	case phaseInit:
		return "init step"
	case phaseCleanupAlways:
		return "cleanup step"
	case phaseCleanupFailure:
		return "cleanup (on_failure) step"
	default:
		return "step"
	}
}

func (p stepPhase) startMessage(stepName string) string {
	if p == phaseMain {
		return fmt.Sprintf("Starting step: %s", stepName)
	}
	return fmt.Sprintf("Starting %s: %s", p.displayName(), stepName)
}

func (p stepPhase) successMessage() string {
	switch p {
	case phaseInit:
		return "Init step completed successfully"
	case phaseCleanupAlways:
		return "Cleanup step completed successfully"
	case phaseCleanupFailure:
		return "Cleanup (on_failure) step completed successfully"
	default:
		return "Step completed successfully"
	}
}

func (p stepPhase) failureMessage(err error) string {
	cleanErr := ExtractCleanError(err)
	switch p {
	case phaseInit:
		return fmt.Sprintf("Init step failed: %s", cleanErr)
	case phaseCleanupAlways:
		return fmt.Sprintf("Cleanup step failed: %s", cleanErr)
	case phaseCleanupFailure:
		return fmt.Sprintf("Cleanup (on_failure) step failed: %s", cleanErr)
	default:
		return fmt.Sprintf("Step failed: %s", cleanErr)
	}
}

func (p stepPhase) wrapError(idx int, name string, err error) error {
	switch p {
	case phaseInit:
		return fmt.Errorf("init step %q: %w", name, err)
	case phaseCleanupAlways, phaseCleanupFailure:
		return fmt.Errorf("cleanup step %q: %w", name, err)
	default:
		return fmt.Errorf("step %d: %w", idx, err)
	}
}

type executionOptions struct {
	ActivityTimeout time.Duration
	RetryPolicy     *temporal.RetryPolicy
}

func cloneVars(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		return make(map[string]interface{})
	}

	cloned := make(map[string]interface{}, len(source))
	keys := workflow.DeterministicKeys(source)
	for _, k := range keys {
		cloned[k] = source[k]
	}
	return cloned
}

func injectSuiteGlobals(state map[string]string, suiteGlobals map[string]string) {
	if len(suiteGlobals) == 0 {
		return
	}

	keys := workflow.DeterministicKeys(suiteGlobals)
	for _, key := range keys {
		state[key] = suiteGlobals[key]
	}
}

func runStepSequence(
	ctx workflow.Context,
	runID string,
	testName string,
	phase stepPhase,
	steps []dsl.Step,
	state map[string]string,
	vars map[string]interface{},
	suiteOpenAPI *dsl.OpenAPISuiteConfig,
	opts *executionOptions,
	stopOnError bool,
) error {
	if len(steps) == 0 {
		return nil
	}

	var firstErr error
	for idx, step := range steps {
		if err := executeStep(ctx, runID, testName, phase, idx, step, state, vars, suiteOpenAPI, opts); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if stopOnError {
				return firstErr
			}
		}
	}

	return firstErr
}

func executeStep(
	ctx workflow.Context,
	runID string,
	testName string,
	phase stepPhase,
	index int,
	step dsl.Step,
	state map[string]string,
	vars map[string]interface{},
	suiteOpenAPI *dsl.OpenAPISuiteConfig,
	opts *executionOptions,
) error {
	logger := workflow.GetLogger(ctx)

	sendStepLog(ctx, runID, testName, step.Name, phase.startMessage(step.Name), "n/a", false)

	var err error
	if step.Plugin == "delay" {
		err = handleDelayStep(ctx, step, testName, runID)
	} else {
		err = executePlugin(ctx, step, state, vars, runID, testName, suiteOpenAPI, opts)
	}

	if err != nil {
		sendStepLog(ctx, runID, testName, step.Name, phase.failureMessage(err), "red", true)
		logger.Error("Step execution failed", "phase", string(phase), "step", step.Name, "error", err)
		return phase.wrapError(index, step.Name, err)
	}

	sendStepLog(ctx, runID, testName, step.Name, phase.successMessage(), "green", false)
	logger.Info("Step completed successfully", "phase", string(phase), "step", step.Name)
	return nil
}

func runCleanupSequences(
	ctx workflow.Context,
	baseAO workflow.ActivityOptions,
	runID string,
	testName string,
	cleanup *dsl.CleanupSpec,
	state map[string]string,
	vars map[string]interface{},
	suiteOpenAPI *dsl.OpenAPISuiteConfig,
	testFailed bool,
) error {
	if cleanup == nil {
		return nil
	}

	logger := workflow.GetLogger(ctx)
	disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
	disconnected := workflow.WithActivityOptions(disconnectedCtx, baseAO)

	opts := &executionOptions{
		ActivityTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}

	var firstErr error

	if testFailed && len(cleanup.OnFailure) > 0 {
		if err := runStepSequence(disconnected, runID, testName, phaseCleanupFailure, cleanup.OnFailure, state, vars, suiteOpenAPI, opts, false); err != nil {
			logger.Warn("Cleanup on_failure sequence encountered errors", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if len(cleanup.Always) > 0 {
		if err := runStepSequence(disconnected, runID, testName, phaseCleanupAlways, cleanup.Always, state, vars, suiteOpenAPI, opts, false); err != nil {
			logger.Warn("Cleanup always sequence encountered errors", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func handleDelayStep(ctx workflow.Context, step dsl.Step, testName, runID string) error {
	// Extract duration directly from step config
	durationStr, ok := step.Config["duration"].(string)
	if !ok {
		return fmt.Errorf("step %q: duration is required and must be a string", step.Name)
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("step %q: invalid duration format: %w", step.Name, err)
	}

	// Send delay start log
	sendStepLog(ctx, runID, testName, step.Name, fmt.Sprintf("Delaying for %s", durationStr), "n/a", false)

	return workflow.Sleep(ctx, duration)
}

// executePlugin executes any registered plugin through the plugin registry
func executePlugin(ctx workflow.Context, step dsl.Step, state map[string]string, vars map[string]interface{}, runID, testName string, suiteOpenAPI *dsl.OpenAPISuiteConfig, opts *executionOptions) error {
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
		"run": map[string]interface{}{
			"id": runID,
		},
	}

	// Add additional parameters based on plugin type
	if step.Assertions != nil {
		pluginParams["assertions"] = step.Assertions
	}
	if step.Save != nil {
		logger.Info(fmt.Sprintf("Adding save to pluginParams: %v", step.Save))
		pluginParams["save"] = step.Save
	}
	// Pass vars for script plugin usage (other plugins ignore them since CLI processes config vars)
	if vars != nil {
		pluginParams["vars"] = vars
	}

	if step.Plugin == "http" && suiteOpenAPI != nil {
		suiteMap := map[string]interface{}{}
		if suiteOpenAPI.Spec != "" {
			suiteMap["spec"] = suiteOpenAPI.Spec
		}
		if suiteOpenAPI.Version != "" {
			suiteMap["version"] = suiteOpenAPI.Version
		}
		if suiteOpenAPI.ValidateRequest != nil {
			suiteMap["validate_request"] = *suiteOpenAPI.ValidateRequest
		}
		if suiteOpenAPI.ValidateResponse != nil {
			suiteMap["validate_response"] = *suiteOpenAPI.ValidateResponse
		}
		if suiteOpenAPI.CacheTTL != "" {
			suiteMap["cache_ttl"] = suiteOpenAPI.CacheTTL
		}
		pluginParams["suite_openapi"] = suiteMap
	}

	// Create step-specific activity options with retry policy
	retryPolicy := buildRetryPolicy(step.Retry)
	if opts != nil && opts.RetryPolicy != nil {
		retryPolicy = opts.RetryPolicy
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 30,
		RetryPolicy:         retryPolicy,
	}
	if opts != nil && opts.ActivityTimeout > 0 {
		ao.StartToCloseTimeout = opts.ActivityTimeout
	}
	stepCtx := workflow.WithActivityOptions(ctx, ao)

	// Execute the plugin activity with step-specific options
	var activityResp interface{}
	err := workflow.ExecuteActivity(stepCtx, step.Plugin, pluginParams).Get(stepCtx, &activityResp)
	if err != nil {
		logger.Error("Plugin activity failed", "plugin", step.Plugin, "error", err)
		return fmt.Errorf("%s activity error: %w", step.Plugin, err)
	}

	// Update workflow state with saved values (if any)
	if activityResp != nil {
		savedValues := extractSavedValues(ctx, activityResp)
		if len(savedValues) > 0 {
			logger.Info(fmt.Sprintf("Saved values from %s plugin: %v", step.Plugin, savedValues))
			keys := workflow.DeterministicKeys(savedValues)
			for _, key := range keys {
				value := savedValues[key]
				state[key] = value
				logger.Info(fmt.Sprintf("Updated state[%s] = %s", key, state[key]))
			}
		}

		// Handle log messages from log plugin
		if step.Plugin == "log" {
			if err := forwardLogMessage(ctx, activityResp, runID, testName, step.Name); err != nil {
				logger.Warn("Failed to forward log message", "error", err)
				// Don't fail the workflow if log forwarding fails
			}
		}
	}

	logger.Info(fmt.Sprintf("Updated state: %v", state))
	return nil
}

// buildRetryPolicy converts DSL retry configuration to Temporal RetryPolicy
func buildRetryPolicy(retryConfig *dsl.RetryPolicy) *temporal.RetryPolicy {
	// If no retry config is provided, disable retries entirely
	if retryConfig == nil {
		return &temporal.RetryPolicy{
			MaximumAttempts: 1,
		}
	}

	// Build retry policy from configuration
	policy := &temporal.RetryPolicy{}

	// Set initial interval
	if retryConfig.InitialInterval != "" {
		if duration, err := time.ParseDuration(retryConfig.InitialInterval); err == nil {
			policy.InitialInterval = duration
		}
	}

	// Set maximum interval
	if retryConfig.MaximumInterval != "" {
		if duration, err := time.ParseDuration(retryConfig.MaximumInterval); err == nil {
			policy.MaximumInterval = duration
		}
	}

	// Set maximum attempts
	if retryConfig.MaximumAttempts > 0 {
		policy.MaximumAttempts = int32(retryConfig.MaximumAttempts)
	} else {
		// Default to 1 attempt if not specified
		policy.MaximumAttempts = 1
	}

	// Set backoff coefficient
	if retryConfig.BackoffCoefficient > 0 {
		policy.BackoffCoefficient = retryConfig.BackoffCoefficient
	}

	// Set non-retryable error types
	if len(retryConfig.NonRetryableErrors) > 0 {
		policy.NonRetryableErrorTypes = retryConfig.NonRetryableErrors
	}

	return policy
}

// extractSavedValues extracts saved values from plugin response using deterministic iteration
func extractSavedValues(ctx workflow.Context, response interface{}) map[string]string {
	savedValues := make(map[string]string)

	// Handle response - it comes back as map[string]interface{} due to JSON serialization
	if respMap, ok := response.(map[string]interface{}); ok {
		// Check for saved values in response (all plugins use "saved" via json tags)
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
	}

	return savedValues
}

// sendStepLog sends step-level logs to the engine
func sendStepLog(ctx workflow.Context, runID, testName, stepName, message, color string, bold bool) {
	// Get workflow info to extract workflow ID
	workflowInfo := workflow.GetInfo(ctx)

	// Prepare parameters for log forwarder activity
	forwarderParams := map[string]interface{}{
		"run_id":      runID,
		"workflow_id": workflowInfo.WorkflowExecution.ID,
		"message":     message,
		"color":       color,
		"bold":        bold,
		"test_name":   testName,
		"step_name":   stepName,
	}

	// Execute log forwarder activity (ignore errors to not fail workflow)
	var forwarderResp interface{}
	err := workflow.ExecuteActivity(ctx, "LogForwarderActivity", forwarderParams).Get(ctx, &forwarderResp)
	if err != nil {
		// Log to temporal logger but don't fail the workflow
		logger := workflow.GetLogger(ctx)
		logger.Warn("Failed to forward step log", "error", err)
	}
}

// forwardLogMessage forwards log messages from log plugin to the engine
func forwardLogMessage(ctx workflow.Context, activityResp interface{}, runID, testName, stepName string) error {
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
				"test_name":   testName,
				"step_name":   stepName,
			}

			// Execute log forwarder activity
			var forwarderResp interface{}
			return workflow.ExecuteActivity(ctx, "LogForwarderActivity", forwarderParams).Get(ctx, &forwarderResp)
		}
	}
	return nil
}
