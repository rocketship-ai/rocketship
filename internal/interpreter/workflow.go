package interpreter

import (
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	// plugins
	"github.com/rocketship-ai/rocketship/internal/plugins/delay"
)

func TestWorkflow(ctx workflow.Context, test dsl.Test) error {
	// vars := map[string]string{}

	ao := workflow.ActivityOptions{
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
			// TODO: Dummy activity. Remove once we have tested delay working e2e.
			_, err = dp.Activity(ctx, step.Config)
			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}
			workflow.Sleep(ctx, dp.Config.Duration)
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

		workflow.GetLogger(ctx).Info(fmt.Sprintf("Step %d/%d PASSED", i+1, len(test.Steps)))
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
	return s // Placeholder implementation
}

func extractJSONPath(jsonStr, path string) (string, error) {
	return "", nil
}

func contains(s, substr string) bool {
	return true // Placeholder implementation
}
