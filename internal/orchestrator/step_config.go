package orchestrator

import (
	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// BuildStepConfig creates a step_config map from a DSL step definition.
// This is used to populate placeholder run_steps at run start so the UI
// can display step metadata (name, plugin, config, assertions, etc.) before execution.
func BuildStepConfig(step dsl.Step) map[string]interface{} {
	config := make(map[string]interface{})

	config["name"] = step.Name
	config["plugin"] = step.Plugin

	if step.Config != nil {
		config["config"] = step.Config
	}

	if len(step.Assertions) > 0 {
		config["assertions"] = step.Assertions
	}

	if len(step.Save) > 0 {
		config["save"] = step.Save
	}

	if step.Retry != nil {
		retry := make(map[string]interface{})
		if step.Retry.InitialInterval != "" {
			retry["initial_interval"] = step.Retry.InitialInterval
		}
		if step.Retry.MaximumInterval != "" {
			retry["maximum_interval"] = step.Retry.MaximumInterval
		}
		if step.Retry.MaximumAttempts > 0 {
			retry["maximum_attempts"] = step.Retry.MaximumAttempts
		}
		if step.Retry.BackoffCoefficient > 0 {
			retry["backoff_coefficient"] = step.Retry.BackoffCoefficient
		}
		if len(step.Retry.NonRetryableErrors) > 0 {
			retry["non_retryable_errors"] = step.Retry.NonRetryableErrors
		}
		if len(retry) > 0 {
			config["retry"] = retry
		}
	}

	return config
}
