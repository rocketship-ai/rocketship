package supabase

import (
	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// replaceVariables replaces {{variable}} placeholders with values from state using DSL template system
// env parameter provides environment secrets from project environment for {{ .env.* }} resolution
func replaceVariables(input string, state map[string]string, env map[string]string) string {
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}

	// Create template context with runtime variables and env secrets
	// Config variables ({{ .vars.* }}) are processed earlier by CLI
	// Environment variables ({{ .env.* }}) are handled by DSL template system
	// with precedence: OS env > env secrets from DB
	context := dsl.TemplateContext{
		Runtime: runtime,
		Env:     env,
	}

	// Use centralized template processing for consistent variable handling
	// This supports runtime vars, environment vars, and escaped handlebars
	result, err := dsl.ProcessTemplate(input, context)
	if err != nil {
		// If template processing fails, return original input to maintain backward compatibility
		// This can happen if the input contains invalid template syntax
		return input
	}

	return result
}

// processVariablesInMap recursively processes variables in a map structure using DSL template system
// env parameter provides environment secrets from project environment for {{ .env.* }} resolution
func processVariablesInMap(data map[string]interface{}, state map[string]string, env map[string]string) map[string]interface{} {
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}

	// Create template context with runtime variables and env secrets
	context := dsl.TemplateContext{
		Runtime: runtime,
		Env:     env,
	}

	// Use DSL recursive processing which handles all data types consistently
	// This leverages the same logic as ProcessConfigVariablesRecursive but for runtime vars
	result := processRuntimeVariablesRecursive(data, context)
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}

	// Fallback to original data if processing fails
	return data
}

// processRuntimeVariablesRecursive processes runtime variables in any nested data structure
func processRuntimeVariablesRecursive(data interface{}, context dsl.TemplateContext) interface{} {
	switch v := data.(type) {
	case string:
		result, err := dsl.ProcessTemplate(v, context)
		if err != nil {
			return v // Return original on error
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Process the key itself (in case it contains variables)
			processedKey, err := dsl.ProcessTemplate(key, context)
			if err != nil {
				processedKey = key // Use original key on error
			}
			// Process the value recursively
			result[processedKey] = processRuntimeVariablesRecursive(value, context)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = processRuntimeVariablesRecursive(item, context)
		}
		return result
	default:
		// For non-string types (numbers, booleans, etc.), return as-is
		return data
	}
}

// processFilters processes variables in filter configurations using DSL template system
// env parameter provides environment secrets from project environment for {{ .env.* }} resolution
func processFilters(filters []FilterConfig, state map[string]string, env map[string]string) []FilterConfig {
	if filters == nil {
		return nil
	}

	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}

	// Create template context with runtime variables and env secrets
	context := dsl.TemplateContext{
		Runtime: runtime,
		Env:     env,
	}

	result := make([]FilterConfig, len(filters))
	for i, filter := range filters {
		result[i] = FilterConfig{
			Column:   replaceVariables(filter.Column, state, env),
			Operator: filter.Operator,
			Value:    processRuntimeVariablesRecursive(filter.Value, context),
		}
	}
	return result
}
