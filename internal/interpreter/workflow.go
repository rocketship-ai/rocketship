package interpreter

import (
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	// plugins
	"github.com/rocketship-ai/rocketship/internal/plugins/delay"
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
)

func TestWorkflow(ctx workflow.Context, test dsl.Test) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 30, // TODO: Make this configurable
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 2,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for i, step := range test.Steps {
		workflow.GetLogger(ctx).Info(fmt.Sprintf("Executing test %q, step %q", test.Name, step.Name))

		switch step.Plugin {
		case "delay":
			dp, err := delay.ParseYAML(step)
			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}

			duration, err := time.ParseDuration(dp.Config.Duration)
			if err != nil {
				return fmt.Errorf("step %q: invalid duration format: %w", step.Name, err)
			}

			err = workflow.Sleep(ctx, duration)
			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}
		case "http":
			hp, err := http.ParseYAML(step)
			if err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}

			var resp *http.HTTPResponse
			err = workflow.ExecuteActivity(ctx, hp.Activity, hp).Get(ctx, &resp)
			if err != nil {
				return fmt.Errorf("step %d: http activity error: %w", i, err)
			}
		default:
			return fmt.Errorf("step %s: unknown plugin %s", step.Name, step.Plugin)
		}

		workflow.GetLogger(ctx).Info(fmt.Sprintf("Step %q PASSED", step.Name))
	}

	return nil
}
