package interpreter

import (
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func TestWorkflow(
	ctx workflow.Context,
	test dsl.Test,
	vars map[string]interface{},
	runID string,
	suiteOpenAPI *dsl.OpenAPISuiteConfig,
	suiteGlobals map[string]string,
) (map[string]string, error) {
	logger := workflow.GetLogger(ctx)

	baseAO := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 30,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, baseAO)

	state := make(map[string]string)
	logger.Info("Initialized workflow state", "state", state)

	// Clone vars to avoid mutating shared maps and inject suite-wide saved values
	runtimeVars := cloneVars(vars)
	injectSuiteGlobals(state, suiteGlobals)

	var primaryErr error

	if err := runStepSequence(ctx, runID, test.Name, phaseInit, test.Init, state, runtimeVars, suiteOpenAPI, nil, true); err != nil {
		primaryErr = err
	}

	if primaryErr == nil {
		if err := runStepSequence(ctx, runID, test.Name, phaseMain, test.Steps, state, runtimeVars, suiteOpenAPI, nil, true); err != nil {
			primaryErr = err
		}
	}

	testFailed := primaryErr != nil

	if err := runCleanupSequences(ctx, baseAO, runID, test.Name, test.Cleanup, state, runtimeVars, suiteOpenAPI, testFailed); err != nil {
		logger.Warn("Cleanup sequence reported errors", "error", err)
	}

	if primaryErr != nil {
		return state, primaryErr
	}

	return state, nil
}

type SuiteCleanupParams struct {
	RunID          string                  `json:"run_id"`
	TestName       string                  `json:"test_name"`
	Cleanup        *dsl.CleanupSpec        `json:"cleanup"`
	Vars           map[string]interface{}  `json:"vars"`
	SuiteOpenAPI   *dsl.OpenAPISuiteConfig `json:"suite_openapi"`
	SuiteGlobals   map[string]string       `json:"suite_globals"`
	TreatAsFailure bool                    `json:"treat_as_failure"`
}

func SuiteCleanupWorkflow(ctx workflow.Context, params SuiteCleanupParams) error {
	logger := workflow.GetLogger(ctx)

	baseAO := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 30,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, baseAO)

	state := make(map[string]string)
	runtimeVars := cloneVars(params.Vars)
	injectSuiteGlobals(state, params.SuiteGlobals)

	if params.Cleanup == nil {
		logger.Info("No suite cleanup configured, skipping")
		return nil
	}

	testName := params.TestName
	if testName == "" {
		testName = "suite-cleanup"
	}

	if err := runCleanupSequences(ctx, baseAO, params.RunID, testName, params.Cleanup, state, runtimeVars, params.SuiteOpenAPI, params.TreatAsFailure); err != nil {
		logger.Warn("Suite cleanup encountered errors", "error", err)
	}

	return nil
}
