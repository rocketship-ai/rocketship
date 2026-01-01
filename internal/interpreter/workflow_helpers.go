package interpreter

import (
	"errors"
	"fmt"
	"strconv"
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

	// Capture deterministic start time
	startTime := workflow.Now(ctx)

	// Capture snapshot of available state BEFORE plugin execution
	// This represents what variables are available for template substitution in this step
	availableRuntimeState := make(map[string]string, len(state))
	stateKeys := workflow.DeterministicKeys(state)
	for _, k := range stateKeys {
		availableRuntimeState[k] = state[k]
	}

	// Capture snapshot of config vars BEFORE plugin execution
	// These are YAML-defined variables accessed via {{ .vars.* }}
	availableConfigVars := cloneVars(vars)

	sendStepLog(ctx, runID, testName, step.Name, phase.startMessage(step.Name), "n/a", false)

	// Report step as RUNNING
	sendStepReport(ctx, runID, index, step, "RUNNING", "", startTime, time.Time{}, nil)

	var err error
	var activityResp interface{}
	if step.Plugin == "delay" {
		err = handleDelayStep(ctx, step, testName, runID)
	} else {
		activityResp, err = executePluginWithResponse(ctx, step, state, vars, runID, testName, suiteOpenAPI, opts)
	}

	// Capture end time and compute duration
	endTime := workflow.Now(ctx)
	durationMs := endTime.Sub(startTime).Milliseconds()

	// Count assertions from step definition
	assertionCount := 0
	if step.Assertions != nil {
		assertionCount = len(step.Assertions)
	}

	var assertionsPassed, assertionsFailed int
	if activityResp != nil {
		if respMap := toMap(activityResp); respMap != nil {
			// Count passed/failed from assertion results
			if assertionResults, ok := respMap["assertion_results"].([]interface{}); ok {
				for _, ar := range assertionResults {
					if arMap, ok := ar.(map[string]interface{}); ok {
						if passed, ok := arMap["passed"].(bool); ok {
							if passed {
								assertionsPassed++
							} else {
								assertionsFailed++
							}
						}
					}
				}
			}
		}
	}

	// If no assertion results from plugin, fall back to step definition count
	if assertionsPassed == 0 && assertionsFailed == 0 {
		if err == nil {
			assertionsPassed = assertionCount
		} else {
			assertionsFailed = assertionCount
		}
	}

	// Handle activity error
	if err != nil {
		sendStepLog(ctx, runID, testName, step.Name, phase.failureMessage(err), "red", true)
		logger.Error("Step execution failed", "phase", string(phase), "step", step.Name, "error", err)

		// Report step as FAILED with error message
		cleanErr := ExtractCleanError(err)
		sendStepReportWithDetails(ctx, runID, index, step, "FAILED", cleanErr, startTime, endTime, durationMs, assertionsPassed, assertionsFailed, activityResp, availableRuntimeState, availableConfigVars)

		return phase.wrapError(index, step.Name, err)
	}

	sendStepLog(ctx, runID, testName, step.Name, phase.successMessage(), "green", false)
	logger.Info("Step completed successfully", "phase", string(phase), "step", step.Name)

	// Report step as PASSED
	sendStepReportWithDetails(ctx, runID, index, step, "PASSED", "", startTime, endTime, durationMs, assertionsPassed, assertionsFailed, activityResp, availableRuntimeState, availableConfigVars)

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

// executePluginWithResponse executes any registered plugin and returns the response
func executePluginWithResponse(ctx workflow.Context, step dsl.Step, state map[string]string, vars map[string]interface{}, runID, testName string, suiteOpenAPI *dsl.OpenAPISuiteConfig, opts *executionOptions) (interface{}, error) {
	logger := workflow.GetLogger(ctx)

	// Check if plugin is registered
	_, exists := plugins.GetPlugin(step.Plugin)
	if !exists {
		return nil, fmt.Errorf("unknown plugin: %s", step.Plugin)
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
		// If an activity fails with rich details (e.g. HTTP assertion failures), attempt to
		// extract the details so we can persist request/response/assertion info even on failure.
		var appErr *temporal.ApplicationError
		if errors.As(err, &appErr) && appErr.Type() == "http_assertion_failed" {
			var detail map[string]interface{}
			if derr := appErr.Details(&detail); derr == nil && len(detail) > 0 {
				activityResp = detail
			}
		}

		logger.Error("Plugin activity failed", "plugin", step.Plugin, "error", err)
		return activityResp, fmt.Errorf("%s activity error: %w", step.Plugin, err)
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
	return activityResp, nil
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

// toMap safely converts an interface{} to map[string]interface{}
// This is used for extracting UI payload from activity responses
// Temporal's default JSON payload converter returns maps as map[string]interface{}
func toMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
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

// sendStepReport sends a step report to the engine (simple version)
func sendStepReport(ctx workflow.Context, runID string, stepIndex int, step dsl.Step, status, errorMsg string, startTime, endTime time.Time, activityResp interface{}) {
	sendStepReportWithDetails(ctx, runID, stepIndex, step, status, errorMsg, startTime, endTime, 0, 0, 0, activityResp, nil, nil)
}

func deterministicJSONValueString(v interface{}) string {
	switch typed := v.(type) {
	case nil:
		return "null"
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		// YAML numbers are commonly decoded as float64.
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case string:
		return strconv.Quote(typed)
	case []interface{}:
		if len(typed) == 0 {
			return "[]"
		}
		out := make([]byte, 0, 2+len(typed)*8)
		out = append(out, '[')
		for idx, elem := range typed {
			if idx > 0 {
				out = append(out, ',')
			}
			out = append(out, deterministicJSONValueString(elem)...)
		}
		out = append(out, ']')
		return string(out)
	case map[string]interface{}:
		keys := workflow.DeterministicKeys(typed)
		out := make([]byte, 0, 2+len(keys)*16)
		out = append(out, '{')
		for idx, key := range keys {
			if idx > 0 {
				out = append(out, ',')
			}
			out = append(out, strconv.Quote(key)...)
			out = append(out, ':')
			out = append(out, deterministicJSONValueString(typed[key])...)
		}
		out = append(out, '}')
		return string(out)
	default:
		// Fallback: ensure a stable string for common scalar types; avoid relying on fmt's map printing.
		return strconv.Quote(fmt.Sprintf("%v", typed))
	}
}

// buildVariablesData builds the variables_data array combining config vars, runtime state, and saved values
// Config variables are YAML-defined variables accessed via {{ .vars.* }}
// Runtime variables are the workflow state BEFORE the step runs (what templates can reference via {{ var_name }})
// Saved variables are what THIS step produced (extracted from response)
func buildVariablesData(ctx workflow.Context, availableRuntimeState map[string]string, availableConfigVars map[string]interface{}, savedValues map[string]string, step dsl.Step) []map[string]interface{} {
	// Return nil only if all are empty
	if len(availableRuntimeState) == 0 && len(availableConfigVars) == 0 && len(savedValues) == 0 {
		return nil
	}

	var variables []map[string]interface{}

	// First, add config variables (YAML-defined vars accessed via {{ .vars.* }})
	if len(availableConfigVars) > 0 {
		configKeys := workflow.DeterministicKeys(availableConfigVars)
		for _, name := range configKeys {
			value := availableConfigVars[name]
			// Stringify value
			var valueStr string
			if strVal, ok := value.(string); ok {
				valueStr = strVal
			} else {
				valueStr = deterministicJSONValueString(value)
			}

			varData := map[string]interface{}{
				"name":        name,
				"value":       valueStr,
				"source_type": "config",
			}
			variables = append(variables, varData)
		}
	}

	// Second, add runtime variables (pre-step state accessed via {{ var_name }})
	if len(availableRuntimeState) > 0 {
		runtimeKeys := workflow.DeterministicKeys(availableRuntimeState)
		for _, name := range runtimeKeys {
			value := availableRuntimeState[name]

			varData := map[string]interface{}{
				"name":        name,
				"value":       value,
				"source_type": "runtime",
			}
			variables = append(variables, varData)
		}
	}

	// Third, add saved variables (what this step produced)
	if len(savedValues) > 0 {
		// Build a map of save configs by "as" name for enrichment
		saveConfigs := make(map[string]map[string]interface{})
		if step.Save != nil {
			for _, saveMap := range step.Save {
				if as, ok := saveMap["as"].(string); ok && as != "" {
					saveConfigs[as] = saveMap
				}
			}
		}

		savedKeys := workflow.DeterministicKeys(savedValues)
		for _, name := range savedKeys {
			value := savedValues[name]

			varData := map[string]interface{}{
				"name":  name,
				"value": value,
			}

			// Enrich with source info from step.Save if available
			if saveConfig, exists := saveConfigs[name]; exists {
				if jsonPath, ok := saveConfig["json_path"].(string); ok && jsonPath != "" {
					varData["source_type"] = "json_path"
					varData["source"] = jsonPath
				} else if header, ok := saveConfig["header"].(string); ok && header != "" {
					varData["source_type"] = "header"
					varData["source"] = header
				}
			} else {
				varData["source_type"] = "auto"
			}

			variables = append(variables, varData)
		}
	}

	return variables
}

// buildStepConfig builds a sanitized step configuration snapshot
// Returns a minimal config without iterating over maps to satisfy Temporal workflow determinism requirements
func buildStepConfig(step dsl.Step) map[string]interface{} {
	config := map[string]interface{}{
		"name":   step.Name,
		"plugin": step.Plugin,
	}

	// Include config - we pass it as-is since deep sanitization would require map iteration
	// The StepReporterActivity or UI layer can handle redaction if needed
	if step.Config != nil {
		config["config"] = step.Config
	}

	// Include assertions config
	if step.Assertions != nil {
		config["assertions"] = step.Assertions
	}

	// Include save config
	if step.Save != nil {
		config["save"] = step.Save
	}

	// Include retry config
	if step.Retry != nil {
		config["retry"] = map[string]interface{}{
			"maximum_attempts":    step.Retry.MaximumAttempts,
			"initial_interval":    step.Retry.InitialInterval,
			"maximum_interval":    step.Retry.MaximumInterval,
			"backoff_coefficient": step.Retry.BackoffCoefficient,
		}
	}

	return config
}

// sendStepReportWithDetails sends a step report to the engine with full details
// availableRuntimeState is the workflow state BEFORE this step ran (runtime variables for {{ var_name }})
// availableConfigVars is the YAML-defined vars BEFORE this step ran (config variables for {{ .vars.* }})
func sendStepReportWithDetails(ctx workflow.Context, runID string, stepIndex int, step dsl.Step, status, errorMsg string, startTime, endTime time.Time, durationMs int64, assertionsPassed, assertionsFailed int, activityResp interface{}, availableRuntimeState map[string]string, availableConfigVars map[string]interface{}) {
	workflowInfo := workflow.GetInfo(ctx)

	// Build step report parameters
	reportParams := map[string]interface{}{
		"run_id":            runID,
		"workflow_id":       workflowInfo.WorkflowExecution.ID,
		"step_index":        stepIndex,
		"step_name":         step.Name,
		"plugin":            step.Plugin,
		"status":            status,
		"error_message":     errorMsg,
		"assertions_passed": assertionsPassed,
		"assertions_failed": assertionsFailed,
	}

	// Add timestamps
	if !startTime.IsZero() {
		reportParams["started_at"] = startTime.Format(time.RFC3339Nano)
	}
	if !endTime.IsZero() {
		reportParams["ended_at"] = endTime.Format(time.RFC3339Nano)
	}
	if durationMs > 0 {
		reportParams["duration_ms"] = durationMs
	}

	// Build step config snapshot
	stepConfig := buildStepConfig(step)
	reportParams["step_config"] = stepConfig

	// Extract saved values from activity response
	var savedValues map[string]string

	// Extract UI payload and assertion results from HTTP plugin response
	// Temporal's default JSON payload converter returns activity responses as map[string]interface{}
	if activityResp != nil {
		respMap := toMap(activityResp)
		if respMap != nil {
			if uiPayload := toMap(respMap["ui_payload"]); uiPayload != nil {
				// Extract request data
				if reqData := toMap(uiPayload["request"]); reqData != nil {
					reportParams["request_data"] = reqData
				}
				// Extract response data
				if respData := toMap(uiPayload["response"]); respData != nil {
					reportParams["response_data"] = respData
				}
			}

			// Extract assertion results for HTTP plugin
			if assertionResults, ok := respMap["assertion_results"].([]interface{}); ok && len(assertionResults) > 0 {
				reportParams["assertions_data"] = assertionResults
			}

			// Extract saved values from response
			if savedMap, ok := respMap["saved"].(map[string]interface{}); ok && len(savedMap) > 0 {
				savedValues = make(map[string]string)
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

	// Build variables data combining config vars, runtime state, and saved values
	// Include if any has entries (so Variables tab can show config/runtime sections even if step saves nothing)
	variablesData := buildVariablesData(ctx, availableRuntimeState, availableConfigVars, savedValues, step)
	if len(variablesData) > 0 {
		reportParams["variables_data"] = variablesData
	}

	// Execute step reporter activity (ignore errors to not fail workflow)
	var reporterResp interface{}
	err := workflow.ExecuteActivity(ctx, "StepReporterActivity", reportParams).Get(ctx, &reporterResp)
	if err != nil {
		// Log to temporal logger but don't fail the workflow
		logger := workflow.GetLogger(ctx)
		logger.Warn("Failed to send step report", "step", step.Name, "error", err)
	}
}
