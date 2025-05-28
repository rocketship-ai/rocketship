package dsl

import (
	"bytes"
	"fmt"
	"regexp"
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

	// Convert safe escaped format back to literal handlebars
	result := restoreSafeEscapedHandlebars(buf.String())
	return result, nil
}

// ProcessConfigVariablesOnly processes only config variables ({{ .vars.* }}) patterns
// Leaves runtime variables ({{ variable }}) untouched for later processing
// Escaped handlebars using \{{ }} will be handled in final processing
func ProcessConfigVariablesOnly(input string, vars map[string]interface{}) (string, error) {
	if input == "" {
		return input, nil
	}

	// Handle escaped handlebars first - convert to safe placeholders
	processed := handleAllEscapedHandlebars(input)

	// Only process if the string contains .vars patterns
	if !strings.Contains(processed, ".vars.") {
		return processed, nil
	}

	// Create template data with only vars
	templateData := map[string]interface{}{
		"vars": vars,
	}

	// Create template
	tmpl, err := template.New("rocketship").Parse(processed)
	if err != nil {
		// If parsing fails due to runtime variables, try to process just the config vars
		result := processConfigVarsWithRegex(processed, vars)
		return result, nil
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		// If execution fails due to missing runtime variables, try regex approach
		result := processConfigVarsWithRegex(processed, vars)
		return result, nil
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


// handleAllEscapedHandlebars processes escaped handlebars with support for unlimited escape levels:
// Algorithm: Count consecutive backslashes before {{ }}
// - Even number of \: template variable (half backslashes remain)
// - Odd number of \: literal handlebars (half backslashes remain, rounded down)
// Examples:
//   \{{ }} (1) -> {{ }} (0, literal handlebars)
//   \\{{ }} (2) -> \{{ }} (1, literal text)  
//   \\\{{ }} (3) -> \{{ }} (1, literal handlebars)
//   \\\\{{ }} (4) -> \\{{ }} (2, literal text)
func handleAllEscapedHandlebars(input string) string {
	result := input
	
	// Match any number of consecutive backslashes followed by handlebars
	re := regexp.MustCompile(`(\\+)(\{\{[^}]*\}\})`)
	
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) >= 3 {
			backslashes := submatch[1]  // The backslashes
			handlebars := submatch[2]   // The {{ content }}
			
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
	re2 := regexp.MustCompile(`__ROCKETSHIP_ESCAPED_HANDLEBARS_\d+__`)
	result = re2.ReplaceAllString(result, "{_{ESCAPED}_}")
	
	return result
}

// restoreSafeEscapedHandlebars converts safe escaped placeholders back to literal handlebars
func restoreSafeEscapedHandlebars(input string) string {
	// Convert {_{ content }_} -> {{ content }}
	re := regexp.MustCompile(`\{_\{([^}]*?)\}_\}`)
	return re.ReplaceAllString(input, "{{$1}}")
}

// convertRuntimeVariables converts runtime variables from {{ var }} to {{ .var }} for Go template compatibility
func convertRuntimeVariables(input string, runtime map[string]interface{}) string {
	result := input
	
	// Use regex to handle whitespace around variable names
	for key := range runtime {
		// Create pattern that matches {{ key }} with optional whitespace
		pattern := fmt.Sprintf(`\{\{\s*%s\s*\}\}`, regexp.QuoteMeta(key))
		replacement := fmt.Sprintf("{{ .%s }}", key)
		
		// Only replace if it's not already using dot notation
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, replacement)
	}
	
	return result
}