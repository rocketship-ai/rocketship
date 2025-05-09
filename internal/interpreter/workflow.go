package interpreter

import (
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	// plugins
	"github.com/rocketship-ai/rocketship/internal/plugins/delay"
)

func TestWorkflow(ctx workflow.Context, test dsl.Test) error {
	// vars := map[string]string{}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 30, // TODO: Make this configurable
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 2,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for _, step := range test.Steps {
		workflow.GetLogger(ctx).Info(fmt.Sprintf("Executing test %q, step %q", test.Name, step.Name))

		switch step.Plugin {
		case "delay":
			dp, err := delay.ParseYAML(step)
			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}

			err = workflow.ExecuteActivity(ctx, dp.Activity, step.Config).Get(ctx, nil)
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
		// case "http":
		// 	p := interpolateParams(step.Params, vars)
		// 	var out map[string]interface{}
		// 	err := workflow.ExecuteActivity(ctx, "http.send", p).Get(ctx, &out)
		// 	if err != nil {
		// 		return fmt.Errorf("step %d: http.send error: %w", i, err)
		// 	}

		// 	if exp := step.Expect; exp != nil {
		// 		if statusExp, ok := exp["status"]; ok {
		// 			status := int(out["status"].(float64))
		// 			if status != int(statusExp.(float64)) {
		// 				return fmt.Errorf("step %d: HTTP status mismatch: expected %v, got %v", i, statusExp, status)
		// 			}
		// 		}
		// 		if bodyContainsExp, ok := exp["bodyContains"]; ok {
		// 			body := out["body"].(string)
		// 			if !contains(body, bodyContainsExp.(string)) {
		// 				return fmt.Errorf("step %d: HTTP body does not contain expected string", i)
		// 			}
		// 		}
		// 	}

		// 	if step.Save != nil {
		// 		value, err := extractJSONPath(out["body"].(string), step.Save.JSONPath)
		// 		if err != nil {
		// 			return fmt.Errorf("step %d: failed to extract JSON path: %w", i, err)
		// 		}
		// 		vars[step.Save.As] = value
		// 	}
		default:
			return fmt.Errorf("step %s: unknown plugin %s", step.Name, step.Plugin)
		}

		workflow.GetLogger(ctx).Info(fmt.Sprintf("Step %q PASSED", step.Name))
	}

	return nil
}

// func interpolateParams(params map[string]interface{}, vars map[string]string) map[string]interface{} {
// 	result := make(map[string]interface{})
// 	for k, v := range params {
// 		if strVal, ok := v.(string); ok {
// 			result[k] = interpolateString(strVal, vars)
// 		} else {
// 			result[k] = v
// 		}
// 	}
// 	return result
// }

// func interpolateString(s string, vars map[string]string) string {
// 	for k, v := range vars {
// 		s = replaceVar(s, k, v)
// 	}
// 	return s
// }

// func replaceVar(s, varName, varValue string) string {
// 	return s // Placeholder implementation
// }

// func extractJSONPath(jsonStr, path string) (string, error) {
// 	return "", nil
// }

// func contains(s, substr string) bool {
// 	return true // Placeholder implementation
// }
