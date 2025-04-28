package interpreter

import (
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func TestWorkflow(ctx workflow.Context, spec dsl.Test) error {
	vars := map[string]string{}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 2,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for i, step := range spec.Steps {
		workflow.GetLogger(ctx).Info(fmt.Sprintf("Executing step %d: %s", i+1, step.Op))

		switch step.Op {
		case "sleep":
			d, err := time.ParseDuration(step.Duration)
			if err != nil {
				return fmt.Errorf("step %d: invalid duration %q: %w", i, step.Duration, err)
			}
			if err := workflow.Sleep(ctx, d); err != nil {
				return fmt.Errorf("step %d: sleep error: %w", i, err)
			}

		case "http.send":
			p := interpolateParams(step.Params, vars)
			var out map[string]interface{}
			err := workflow.ExecuteActivity(ctx, "http.send", p).Get(ctx, &out)
			if err != nil {
				return fmt.Errorf("step %d: http.send error: %w", i, err)
			}

			if exp := step.Expect; exp != nil {
				if statusExp, ok := exp["status"]; ok {
					status := int(out["status"].(float64))
					if status != int(statusExp.(float64)) {
						return fmt.Errorf("step %d: HTTP status mismatch: expected %v, got %v", i, statusExp, status)
					}
				}
				if bodyContainsExp, ok := exp["bodyContains"]; ok {
					body := out["body"].(string)
					if !contains(body, bodyContainsExp.(string)) {
						return fmt.Errorf("step %d: HTTP body does not contain expected string", i)
					}
				}
			}

			if step.Save != nil {
				value, err := extractJSONPath(out["body"].(string), step.Save.JSONPath)
				if err != nil {
					return fmt.Errorf("step %d: failed to extract JSON path: %w", i, err)
				}
				vars[step.Save.As] = value
			}

		case "aws.s3.get", "aws.s3.exists":
			p := interpolateParams(step.Params, vars)
			var out map[string]interface{}
			err := workflow.ExecuteActivity(ctx, step.Op, p).Get(ctx, &out)
			if err != nil {
				return fmt.Errorf("step %d: %s error: %w", i, step.Op, err)
			}

			if exp := step.Expect; exp != nil {
				if existsExp, ok := exp["exists"]; ok {
					exists := out["exists"].(bool)
					if exists != existsExp.(bool) {
						return fmt.Errorf("step %d: S3 exists mismatch: expected %v, got %v", i, existsExp, exists)
					}
				}
			}

		case "aws.ddb.query":
			p := interpolateParams(step.Params, vars)
			var out map[string]interface{}
			err := workflow.ExecuteActivity(ctx, step.Op, p).Get(ctx, &out)
			if err != nil {
				return fmt.Errorf("step %d: %s error: %w", i, step.Op, err)
			}

			if exp := step.Expect; exp != nil {
				for path, expectedValue := range exp {
					value, err := extractJSONPath(fmt.Sprintf("%v", out), path)
					if err != nil {
						return fmt.Errorf("step %d: failed to extract JSON path: %w", i, err)
					}
					if value != fmt.Sprintf("%v", expectedValue) {
						return fmt.Errorf("step %d: DynamoDB value mismatch: expected %v, got %v", i, expectedValue, value)
					}
				}
			}

		case "aws.sqs.send":
			p := interpolateParams(step.Params, vars)
			var out map[string]interface{}
			err := workflow.ExecuteActivity(ctx, step.Op, p).Get(ctx, &out)
			if err != nil {
				return fmt.Errorf("step %d: %s error: %w", i, step.Op, err)
			}

		default:
			return fmt.Errorf("step %d: unknown opcode %q", i, step.Op)
		}

		workflow.GetLogger(ctx).Info(fmt.Sprintf("Step %d/%d PASSED", i+1, len(spec.Steps)))
	}

	return nil
}

func interpolateParams(params map[string]interface{}, vars map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range params {
		if strVal, ok := v.(string); ok {
			result[k] = interpolateString(strVal, vars)
		} else {
			result[k] = v
		}
	}
	return result
}

func interpolateString(s string, vars map[string]string) string {
	for k, v := range vars {
		s = replaceVar(s, k, v)
	}
	return s
}

func replaceVar(s, varName, varValue string) string {
	return fmt.Sprintf("%s", s) // Placeholder implementation
}

func extractJSONPath(jsonStr, path string) (string, error) {
	return "", nil
}

func contains(s, substr string) bool {
	return true // Placeholder implementation
}
