package playwright

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...interface{}) {}
func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Warn(string, ...interface{})  {}
func (noopLogger) Error(string, ...interface{}) {}

func getLogger(ctx context.Context) log.Logger {
	if activity.IsActivity(ctx) {
		return activity.GetLogger(ctx)
	}
	return noopLogger{}
}

func extractState(params map[string]interface{}) map[string]interface{} {
	state := make(map[string]interface{})
	if raw, ok := params["state"].(map[string]string); ok {
		for k, v := range raw {
			state[k] = v
		}
	}
	if raw, ok := params["state"].(map[string]interface{}); ok {
		for k, v := range raw {
			state[k] = v
		}
	}
	return state
}

func templateStringField(config map[string]interface{}, key string, ctx dsl.TemplateContext) (string, error) {
	raw, ok := config[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}

	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", key)
	}

	out, err := dsl.ProcessTemplate(value, ctx)
	if err != nil {
		return "", fmt.Errorf("failed to process template for %s: %w", key, err)
	}

	return out, nil
}

func processSaves(params map[string]interface{}, result map[string]interface{}, saved map[string]string) error {
	saves, ok := params["save"].([]interface{})
	if !ok {
		return nil
	}

	for _, rawSave := range saves {
		saveMap, ok := rawSave.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid save entry %T", rawSave)
		}

		as, ok := saveMap["as"].(string)
		if !ok || as == "" {
			return errors.New("'as' field is required for save")
		}

		required := true
		if v, ok := saveMap["required"].(bool); ok {
			required = v
		}

		jsonPath, hasJSONPath := saveMap["json_path"].(string)
		if !hasJSONPath || jsonPath == "" {
			return fmt.Errorf("save %q must define json_path", as)
		}

		query, err := gojq.Parse(jsonPath)
		if err != nil {
			return fmt.Errorf("failed to parse jq expression %q: %w", jsonPath, err)
		}

		iter := query.Run(result)
		var value interface{}
		var found bool

		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				return fmt.Errorf("jq evaluation error for %q: %w", jsonPath, err)
			}
			if !found {
				value = v
				found = true
			}
		}

		if !found {
			if required {
				return fmt.Errorf("save %q failed: no results from %s", as, jsonPath)
			}
			continue
		}

		saved[as] = fmt.Sprint(value)
	}

	return nil
}

func processAssertions(params map[string]interface{}, result map[string]interface{}, state map[string]interface{}, envSecrets map[string]string) error {
	assertions, ok := params["assertions"].([]interface{})
	if !ok {
		return nil
	}

	for _, rawAssertion := range assertions {
		assertion, ok := rawAssertion.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid assertion %T", rawAssertion)
		}

		typ, ok := assertion["type"].(string)
		if !ok {
			return errors.New("assertion type is required")
		}

		switch typ {
		case "json_path":
			path, ok := assertion["path"].(string)
			if !ok {
				return errors.New("json_path assertion requires path")
			}

			query, err := gojq.Parse(path)
			if err != nil {
				return fmt.Errorf("failed to parse jq expression %q: %w", path, err)
			}

			iter := query.Run(result)
			var actual interface{}
			var found bool

			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := v.(error); ok {
					return fmt.Errorf("jq evaluation error for %q: %w", path, err)
				}
				if !found {
					actual = v
					found = true
				}
			}

			if existsOnly, ok := assertion["exists"].(bool); ok && existsOnly {
				if !found {
					return fmt.Errorf("expected path %q to exist", path)
				}
				continue
			}

			if !found {
				return fmt.Errorf("jq path %q did not return a value", path)
			}

			if expectedRaw, ok := assertion["expected"]; ok {
				expected := expectedRaw
				if expectedStr, ok := expectedRaw.(string); ok {
					rendered, err := dsl.ProcessTemplate(expectedStr, dsl.TemplateContext{Runtime: state, Env: envSecrets})
					if err != nil {
						return fmt.Errorf("failed to process expected template: %w", err)
					}
					expected = rendered
				}
				if fmt.Sprint(actual) != fmt.Sprint(expected) {
					return fmt.Errorf("assertion failed for %q: expected %v, got %v", path, expected, actual)
				}
			}
		default:
			return fmt.Errorf("unsupported assertion type %q", typ)
		}
	}

	return nil
}
