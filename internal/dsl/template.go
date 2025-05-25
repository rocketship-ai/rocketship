package dsl

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// TemplateContext holds both config variables and runtime variables for template processing
type TemplateContext struct {
	Vars    map[string]interface{} // Config variables (accessed via {{ vars.key }})
	Runtime map[string]interface{} // Runtime variables (accessed via {{ key }})
}

// ProcessTemplate processes a string containing template variables
// It supports both config variables ({{ .vars.key }}) and runtime variables ({{ key }})
func ProcessTemplate(input string, context TemplateContext) (string, error) {
	if input == "" {
		return input, nil
	}

	// Create template with custom delimiters to match our syntax
	tmpl, err := template.New("rocketship").Parse(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"vars": context.Vars,
	}

	// Add runtime variables to the root level
	for key, value := range context.Runtime {
		templateData[key] = value
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ProcessConfigVariablesOnly processes only config variables ({{ .vars.* }}) patterns
// Leaves runtime variables ({{ variable }}) untouched for later processing
func ProcessConfigVariablesOnly(input string, vars map[string]interface{}) (string, error) {
	if input == "" {
		return input, nil
	}

	// Only process if the string contains .vars patterns
	if !strings.Contains(input, ".vars.") {
		return input, nil
	}

	// Create template data with only vars
	templateData := map[string]interface{}{
		"vars": vars,
	}

	// Create template
	tmpl, err := template.New("rocketship").Parse(input)
	if err != nil {
		// If parsing fails due to runtime variables, try to process just the config vars
		return processConfigVarsWithRegex(input, vars), nil
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		// If execution fails due to missing runtime variables, try regex approach
		return processConfigVarsWithRegex(input, vars), nil
	}

	return buf.String(), nil
}

// processConfigVarsWithRegex uses regex to replace only {{ .vars.* }} patterns
func processConfigVarsWithRegex(input string, vars map[string]interface{}) string {
	// This is a fallback when Go templates fail due to mixed variable types
	// We'll manually replace {{ .vars.* }} patterns
	result := input
	
	// Simple string replacement for basic cases
	for key, value := range vars {
		pattern := fmt.Sprintf("{{ .vars.%s }}", key)
		if strValue, ok := value.(string); ok {
			result = strings.ReplaceAll(result, pattern, strValue)
		} else {
			result = strings.ReplaceAll(result, pattern, fmt.Sprintf("%v", value))
		}
	}
	
	// Handle nested vars like {{ .vars.auth.token }}
	result = processNestedVars(result, vars, "vars")
	
	return result
}

// processNestedVars handles nested variable replacement
func processNestedVars(input string, vars map[string]interface{}, prefix string) string {
	result := input
	
	for key, value := range vars {
		currentPrefix := prefix + "." + key
		
		if nestedMap, ok := value.(map[string]interface{}); ok {
			// Recursively process nested maps
			result = processNestedVars(result, nestedMap, currentPrefix)
		} else {
			// Replace the nested variable
			pattern := fmt.Sprintf("{{ .%s }}", currentPrefix)
			if strValue, ok := value.(string); ok {
				result = strings.ReplaceAll(result, pattern, strValue)
			} else {
				result = strings.ReplaceAll(result, pattern, fmt.Sprintf("%v", value))
			}
		}
	}
	
	return result
}

// ProcessConfigVariablesRecursive processes only config variables in any nested data structure
func ProcessConfigVariablesRecursive(data interface{}, vars map[string]interface{}) (interface{}, error) {
	switch v := data.(type) {
	case string:
		return ProcessConfigVariablesOnly(v, vars)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Process the key itself (in case it contains variables)
			processedKey, err := ProcessConfigVariablesOnly(key, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in key '%s': %w", key, err)
			}
			// Process the value recursively
			processedValue, err := ProcessConfigVariablesRecursive(value, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in value for key '%s': %w", key, err)
			}
			result[processedKey] = processedValue
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			processedItem, err := ProcessConfigVariablesRecursive(item, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in array item %d: %w", i, err)
			}
			result[i] = processedItem
		}
		return result, nil
	case []map[string]interface{}:
		result := make([]map[string]interface{}, len(v))
		for i, item := range v {
			processedItem, err := ProcessConfigVariablesRecursive(item, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in array item %d: %w", i, err)
			}
			if processedMap, ok := processedItem.(map[string]interface{}); ok {
				result[i] = processedMap
			} else {
				return nil, fmt.Errorf("expected map[string]interface{} after processing, got %T", processedItem)
			}
		}
		return result, nil
	default:
		// For non-string types (numbers, booleans, etc.), return as-is
		return data, nil
	}
}

// ProcessTemplateRecursive processes templates in any nested data structure
func ProcessTemplateRecursive(data interface{}, context TemplateContext) (interface{}, error) {
	switch v := data.(type) {
	case string:
		return ProcessTemplate(v, context)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Process the key itself (in case it contains variables)
			processedKey, err := ProcessTemplate(key, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in key '%s': %w", key, err)
			}
			// Process the value recursively
			processedValue, err := ProcessTemplateRecursive(value, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in value for key '%s': %w", key, err)
			}
			result[processedKey] = processedValue
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			processedItem, err := ProcessTemplateRecursive(item, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in array item %d: %w", i, err)
			}
			result[i] = processedItem
		}
		return result, nil
	case []map[string]interface{}:
		result := make([]map[string]interface{}, len(v))
		for i, item := range v {
			processedItem, err := ProcessTemplateRecursive(item, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in array item %d: %w", i, err)
			}
			if processedMap, ok := processedItem.(map[string]interface{}); ok {
				result[i] = processedMap
			} else {
				return nil, fmt.Errorf("expected map[string]interface{} after processing, got %T", processedItem)
			}
		}
		return result, nil
	default:
		// For non-string types (numbers, booleans, etc.), return as-is
		return data, nil
	}
}

// IsTemplateString checks if a string contains template variables
func IsTemplateString(s string) bool {
	return strings.Contains(s, "{{") && strings.Contains(s, "}}")
}

// MergeVariables merges CLI variables with YAML vars, with CLI taking precedence
func MergeVariables(yamlVars map[string]interface{}, cliVars map[string]string) map[string]interface{} {
	// Start with YAML vars as base
	result := make(map[string]interface{})
	for k, v := range yamlVars {
		result[k] = v
	}

	// Override with CLI vars (CLI takes precedence)
	for k, v := range cliVars {
		// Support dot notation for nested variables
		setNestedValue(result, k, v)
	}

	return result
}

// setNestedValue sets a value in a nested map using dot notation
// e.g., "auth.token" sets result["auth"]["token"] = value
func setNestedValue(m map[string]interface{}, key string, value interface{}) {
	keys := strings.Split(key, ".")
	current := m

	// Navigate to the parent of the final key
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		if _, exists := current[k]; !exists {
			current[k] = make(map[string]interface{})
		}
		if nested, ok := current[k].(map[string]interface{}); ok {
			current = nested
		} else {
			// If the intermediate key is not a map, create a new map
			current[k] = make(map[string]interface{})
			current = current[k].(map[string]interface{})
		}
	}

	// Set the final value
	current[keys[len(keys)-1]] = value
}