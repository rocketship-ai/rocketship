package dsl

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
)

// Pre-compiled regex patterns for better performance
var (
	escapedHandlebarsRegex = regexp.MustCompile(`(\\+)(\{\{[^}]*\}\})`)
	placeholderRegex       = regexp.MustCompile(`__ROCKETSHIP_ESCAPED_HANDLEBARS_\d+__`)
	runtimeVariableRegex   = regexp.MustCompile(`\{_\{([^}]*?)\}_\}`)
	templateVariableRegex  = regexp.MustCompile(`\{\{\s*([^.\s}][^}]*)\s*\}\}`)
)

// TemplateContext holds runtime variables for template processing
// Config variables are processed earlier by CLI, only runtime variables are needed here
type TemplateContext struct {
	Runtime map[string]interface{} // Runtime variables (accessed via {{ key }})
	Env     map[string]string      // Environment secrets from project environment (accessed via {{ .env.KEY }})
	// Precedence for .env.*: OS env (highest) > context.Env (lowest/from DB)
}

// getEnvironmentVariables returns all environment variables as a map
func getEnvironmentVariables() map[string]interface{} {
	envVars := make(map[string]interface{})
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}
	return envVars
}

// ProcessTemplate processes a string containing template variables
// It supports runtime variables ({{ key }}) and environment variables ({{ .env.key }})
// Config variables ({{ .vars.key }}) are processed earlier by CLI
// Escaped handlebars using \{{ }} will be converted to literal {{ }} text
func ProcessTemplate(input string, context TemplateContext) (string, error) {
	if input == "" {
		return input, nil
	}

	// Handle escaped handlebars - do this every time to catch any that were preserved from config processing
	processed := handleAllEscapedHandlebars(input)

	// Convert runtime variables to use dot notation if they don't already have it
	processed = convertRuntimeVariables(processed, context.Runtime)

	// Create template with custom delimiters to match our syntax
	tmpl, err := template.New("rocketship").Parse(processed)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Build env map with proper precedence:
	// 1. Start with context.Env (from project environment DB, lowest precedence)
	// 2. Overlay OS env (from os.Environ(), highest precedence)
	envMap := make(map[string]interface{})
	for key, value := range context.Env {
		envMap[key] = value
	}
	// OS env takes precedence over context.Env
	for key, value := range getEnvironmentVariables() {
		envMap[key] = value
	}

	// Prepare template data with merged environment variables
	templateData := map[string]interface{}{
		"env": envMap,
	}

	// Add runtime variables to the root level (supporting nested paths)
	for key, value := range context.Runtime {
		insertRuntimeValue(templateData, key, value)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Convert safe escaped format back to literal handlebars
	result := restoreSafeEscapedHandlebars(buf.String())
	return result, nil
}

func insertRuntimeValue(target map[string]interface{}, key string, value interface{}) {
	if key == "" {
		return
	}

	if !strings.Contains(key, ".") {
		target[key] = value
		return
	}

	parts := strings.Split(key, ".")
	current := target

	for i, part := range parts {
		if part == "" {
			continue
		}

		if i == len(parts)-1 {
			current[part] = value
			return
		}

		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
}

// ProcessConfigVariablesOnly processes only config variables ({{ .vars.* }}) and environment variables ({{ .env.* }}) patterns
// Leaves runtime variables ({{ variable }}) untouched for later processing
// Escaped handlebars using \{{ }} will be handled in final processing
func ProcessConfigVariablesOnly(input string, vars map[string]interface{}) (string, error) {
	if input == "" {
		return input, nil
	}

	// Handle escaped handlebars first - convert to safe placeholders
	processed := handleAllEscapedHandlebars(input)

	// Only process if the string contains .vars or .env patterns
	if !strings.Contains(processed, ".vars.") && !strings.Contains(processed, ".env.") {
		return processed, nil
	}

	// Create template data with vars and env
	templateData := map[string]interface{}{
		"vars": vars,
		"env":  getEnvironmentVariables(),
	}

	// Create template
	tmpl, err := template.New("rocketship").Parse(processed)
	if err != nil {
		// If parsing fails due to runtime variables, try to process just the config vars and env vars
		result := processConfigAndEnvVarsWithRegex(processed, vars)
		return result, nil
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		// If execution fails due to missing runtime variables, try regex approach
		result := processConfigAndEnvVarsWithRegex(processed, vars)
		return result, nil
	}

	return buf.String(), nil
}

// ProcessVarsOnlyServerSide processes ONLY config variables ({{ .vars.* }}) patterns
// This is used by the engine server-side after merging with environment config vars
// It leaves {{ .env.* }} patterns untouched for plugin-time resolution with env secrets from the database
func ProcessVarsOnlyServerSide(input string, vars map[string]interface{}) (string, error) {
	if input == "" {
		return input, nil
	}

	// Handle escaped handlebars first - convert to safe placeholders
	processed := handleAllEscapedHandlebars(input)

	// Only process if the string contains .vars patterns
	if !strings.Contains(processed, ".vars.") {
		return processed, nil
	}

	// Use regex approach to only replace .vars.* patterns
	// This leaves .env.* patterns untouched for plugin-time resolution
	result := processVarsOnlyWithRegex(processed, vars)
	return result, nil
}

// processVarsOnlyWithRegex uses regex to replace only {{ .vars.* }} patterns
// Leaves {{ .env.* }} patterns untouched
func processVarsOnlyWithRegex(input string, vars map[string]interface{}) string {
	result := input

	// Process flat config vars
	for key, value := range vars {
		pattern := fmt.Sprintf("{{ .vars.%s }}", key)
		if strValue, ok := value.(string); ok {
			result = strings.ReplaceAll(result, pattern, strValue)
		} else {
			result = strings.ReplaceAll(result, pattern, fmt.Sprintf("%v", value))
		}
		// Also handle without spaces
		patternNoSpace := fmt.Sprintf("{{.vars.%s}}", key)
		if strValue, ok := value.(string); ok {
			result = strings.ReplaceAll(result, patternNoSpace, strValue)
		} else {
			result = strings.ReplaceAll(result, patternNoSpace, fmt.Sprintf("%v", value))
		}
	}

	// Handle nested vars like {{ .vars.auth.token }}
	result = processNestedVars(result, vars, "vars")

	return result
}

// processConfigAndEnvVarsWithRegex uses regex to replace only {{ .vars.* }} and {{ .env.* }} patterns
func processConfigAndEnvVarsWithRegex(input string, vars map[string]interface{}) string {
	// This is a fallback when Go templates fail due to mixed variable types
	// We'll manually replace {{ .vars.* }} and {{ .env.* }} patterns
	result := input

	// Process config vars
	for key, value := range vars {
		pattern := fmt.Sprintf("{{ .vars.%s }}", key)
		if strValue, ok := value.(string); ok {
			result = strings.ReplaceAll(result, pattern, strValue)
		} else {
			result = strings.ReplaceAll(result, pattern, fmt.Sprintf("%v", value))
		}
	}

	// Process environment vars
	envVars := getEnvironmentVariables()
	for key, value := range envVars {
		pattern := fmt.Sprintf("{{ .env.%s }}", key)
		if strValue, ok := value.(string); ok {
			result = strings.ReplaceAll(result, pattern, strValue)
		} else {
			result = strings.ReplaceAll(result, pattern, fmt.Sprintf("%v", value))
		}
	}

	// Handle nested vars like {{ .vars.auth.token }}
	result = processNestedVars(result, vars, "vars")

	// Handle nested env vars like {{ .env.auth.token }} (though env vars are typically flat)
	result = processNestedVars(result, envVars, "env")

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

// ProcessVarsOnlyRecursive processes only {{ .vars.* }} patterns in any nested data structure
// This is used server-side to substitute vars after merging with environment config vars
// It leaves {{ .env.* }} patterns untouched for plugin-time resolution
func ProcessVarsOnlyRecursive(data interface{}, vars map[string]interface{}) (interface{}, error) {
	switch v := data.(type) {
	case string:
		return ProcessVarsOnlyServerSide(v, vars)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			processedKey, err := ProcessVarsOnlyServerSide(key, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in key '%s': %w", key, err)
			}
			processedValue, err := ProcessVarsOnlyRecursive(value, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in value for key '%s': %w", key, err)
			}
			result[processedKey] = processedValue
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			processedItem, err := ProcessVarsOnlyRecursive(item, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in array item %d: %w", i, err)
			}
			result[i] = processedItem
		}
		return result, nil
	case []map[string]interface{}:
		result := make([]map[string]interface{}, len(v))
		for i, item := range v {
			processedItem, err := ProcessVarsOnlyRecursive(item, vars)
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
		return data, nil
	}
}

// ProcessConfigVariablesRecursive processes only config variables in any nested data structure
// This function preserves escaped handlebars across processing phases
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

// MergeInterfaceMaps performs a deep merge of two maps with overlay taking precedence.
// Used for merging environment config vars (base) with run vars (overlay).
// Rules:
//   - If both sides are maps → recurse (merge nested maps)
//   - Otherwise overlay value replaces base value
//   - Arrays/slices: overlay replaces base (no merging)
func MergeInterfaceMaps(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all base values first
	for k, v := range base {
		// Deep copy maps to avoid mutation issues
		if baseMap, ok := v.(map[string]interface{}); ok {
			result[k] = deepCopyMap(baseMap)
		} else {
			result[k] = v
		}
	}

	// Overlay values on top
	for k, overlayVal := range overlay {
		baseVal, exists := result[k]
		if !exists {
			// Key doesn't exist in base - add it (deep copy if map)
			if overlayMap, ok := overlayVal.(map[string]interface{}); ok {
				result[k] = deepCopyMap(overlayMap)
			} else {
				result[k] = overlayVal
			}
			continue
		}

		// Both values exist - check if both are maps
		baseMap, baseIsMap := baseVal.(map[string]interface{})
		overlayMap, overlayIsMap := overlayVal.(map[string]interface{})

		if baseIsMap && overlayIsMap {
			// Both are maps - recursive merge
			result[k] = MergeInterfaceMaps(baseMap, overlayMap)
		} else {
			// Either or both are not maps - overlay replaces base
			if overlayMap, ok := overlayVal.(map[string]interface{}); ok {
				result[k] = deepCopyMap(overlayMap)
			} else {
				result[k] = overlayVal
			}
		}
	}

	return result
}

// deepCopyMap creates a deep copy of a map[string]interface{}
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if nestedMap, ok := v.(map[string]interface{}); ok {
			result[k] = deepCopyMap(nestedMap)
		} else if slice, ok := v.([]interface{}); ok {
			result[k] = deepCopySlice(slice)
		} else {
			result[k] = v
		}
	}
	return result
}

// deepCopySlice creates a deep copy of a []interface{}
func deepCopySlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		if nestedMap, ok := v.(map[string]interface{}); ok {
			result[i] = deepCopyMap(nestedMap)
		} else if slice, ok := v.([]interface{}); ok {
			result[i] = deepCopySlice(slice)
		} else {
			result[i] = v
		}
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

// handleAllEscapedHandlebars processes escaped handlebars with support for unlimited escape levels:
// Algorithm: Count consecutive backslashes before {{ }}
// - Even number of \: template variable (half backslashes remain)
// - Odd number of \: literal handlebars (half backslashes remain, rounded down)
// Examples:
//
//	\{{ }} (1) -> {{ }} (0, literal handlebars)
//	\\{{ }} (2) -> \{{ }} (1, literal text)
//	\\\{{ }} (3) -> \{{ }} (1, literal handlebars)
//	\\\\{{ }} (4) -> \\{{ }} (2, literal text)
func handleAllEscapedHandlebars(input string) string {
	result := input

	// Match any number of consecutive backslashes followed by handlebars
	re := escapedHandlebarsRegex

	result = re.ReplaceAllStringFunc(result, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) >= 3 {
			backslashes := submatch[1] // The backslashes
			handlebars := submatch[2]  // The {{ content }}

			backslashCount := len(backslashes)
			remainingBackslashes := backslashCount / 2
			isOddEscapes := backslashCount%2 == 1

			// Build the result with remaining backslashes
			result := strings.Repeat("\\", remainingBackslashes)

			if isOddEscapes {
				// Odd number of backslashes: treat handlebars as literal
				// Use safe placeholder format that will be restored later
				content := handlebars[2 : len(handlebars)-2] // Extract content from {{ }}
				result += fmt.Sprintf("{_{%s}_}", content)
			} else {
				// Even number of backslashes: treat as template variable
				result += handlebars
			}

			return result
		}
		return match
	})

	// Handle existing placeholders from previous config processing
	re2 := placeholderRegex
	result = re2.ReplaceAllString(result, "{_{ESCAPED}_}")

	return result
}

// restoreSafeEscapedHandlebars converts safe escaped placeholders back to literal handlebars
func restoreSafeEscapedHandlebars(input string) string {
	// Convert {_{ content }_} -> {{ content }}
	re := runtimeVariableRegex
	return re.ReplaceAllString(input, "{{$1}}")
}

// convertRuntimeVariables converts runtime variables from {{ var }} to {{ .var }} for Go template compatibility
func convertRuntimeVariables(input string, runtime map[string]interface{}) string {
	// Use the pre-compiled regex to find all template variables that don't already use dot notation
	return templateVariableRegex.ReplaceAllStringFunc(input, func(match string) string {
		// Extract the variable name from the match
		submatch := templateVariableRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		varName := strings.TrimSpace(submatch[1])

		// Check if this variable exists in runtime context
		if _, exists := runtime[varName]; exists {
			return fmt.Sprintf("{{ .%s }}", varName)
		}
		if strings.Contains(varName, ".") {
			parts := strings.Split(varName, ".")
			if len(parts) > 0 {
				if _, exists := runtime[parts[0]]; exists {
					return fmt.Sprintf("{{ .%s }}", varName)
				}
			}
		}

		// If not in runtime context, leave as-is (could be a .vars or .env variable)
		return match
	})
}
