package interpreter

import (
	"go.temporal.io/sdk/workflow"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

type workflowBuiltinExecutor func(ctx workflow.Context, step dsl.Step, testName, runID string, state map[string]string, envSecrets map[string]string) (interface{}, error)

var workflowBuiltinExecutors = map[string]workflowBuiltinExecutor{
	// workflow-native steps should be implemented here instead of Activities
	// to avoid tying up worker capacity (e.g. delay).
	"delay": func(ctx workflow.Context, step dsl.Step, testName, runID string, state map[string]string, envSecrets map[string]string) (interface{}, error) {
		return nil, handleDelayStep(ctx, step, testName, runID, state, envSecrets)
	},
}

func executeWorkflowBuiltinStep(ctx workflow.Context, step dsl.Step, testName, runID string, state map[string]string, envSecrets map[string]string) (interface{}, bool, error) {
	executor, ok := workflowBuiltinExecutors[step.Plugin]
	if !ok {
		return nil, false, nil
	}
	resp, err := executor(ctx, step, testName, runID, state, envSecrets)
	return resp, true, err
}
