package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/itchyny/gojq"
)

// processSave handles saving values from response
func processSave(ctx context.Context, response *SupabaseResponse, saveConfig map[string]interface{}, saved map[string]string) error {
	asName, ok := saveConfig["as"].(string)
	if !ok {
		return fmt.Errorf("'as' field is required for save config")
	}

	// Check if required is explicitly set to false (defaults to true)
	required := true
	if req, ok := saveConfig["required"].(bool); ok {
		required = req
	}

	var value interface{}

	// Check for JSON path extraction
	if jsonPath, ok := saveConfig["json_path"].(string); ok {
		// Parse the JSON path using gojq
		query, err := gojq.Parse(jsonPath)
		if err != nil {
			return fmt.Errorf("failed to parse JSON path %s: %w", jsonPath, err)
		}

		// Run the query on the response data
		iter := query.Run(response.Data)
		v, ok := iter.Next()
		if !ok {
			if required {
				responseDataJSON, _ := json.Marshal(response.Data)
				return fmt.Errorf("no results from required JSON path %q on data %s", jsonPath, string(responseDataJSON))
			}
			// Optional save that returned no results - skip it
			return nil
		}
		if err, ok := v.(error); ok {
			return fmt.Errorf("error evaluating JSON path %s: %w", jsonPath, err)
		}
		value = v
	} else if header, ok := saveConfig["header"].(string); ok {
		// Extract from headers
		if response.Metadata != nil && response.Metadata.Headers != nil {
			value = response.Metadata.Headers[header]
		}
		if value == nil || value == "" {
			if required {
				return fmt.Errorf("required header %s not found in response", header)
			}
			// Optional save that returned no value - skip it
			return nil
		}
	} else {
		return fmt.Errorf("either 'json_path' or 'header' must be specified for save config")
	}

	// Check for nil value (path exists but value is null)
	if value == nil {
		if required {
			return fmt.Errorf("required value for %q is null", asName)
		}
		// Optional save with null value - save as empty string
		saved[asName] = ""
		return nil
	}

	// Convert value to string based on type
	switch val := value.(type) {
	case string:
		saved[asName] = val
	case float64:
		saved[asName] = fmt.Sprintf("%.0f", val)
	case bool:
		saved[asName] = fmt.Sprintf("%t", val)
	default:
		// For complex types, use JSON marshaling
		bytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("failed to marshal value for %s: %w", asName, err)
		}
		saved[asName] = string(bytes)
	}

	return nil
}

// processAssertions validates response against assertions
func processAssertions(response *SupabaseResponse, assertions []interface{}, params map[string]interface{}) error {
	state := make(map[string]string)
	if stateInterface, ok := params["state"]; ok {
		if stateMap, ok := stateInterface.(map[string]interface{}); ok {
			for k, v := range stateMap {
				state[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	for _, assertionInterface := range assertions {
		assertion, ok := assertionInterface.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid assertion format")
		}

		assertionType, ok := assertion["type"].(string)
		if !ok {
			return fmt.Errorf("assertion type is required")
		}

		var err error
		switch assertionType {
		case "status_code":
			err = processStatusCodeAssertion(response, assertion, state)
		case "json_path":
			err = processJSONPathAssertion(response, assertion, state)
		case "row_count":
			err = processRowCountAssertion(response, assertion, state)
		case "error_code":
			err = processErrorCodeAssertion(response, assertion, state)
		case "error_message":
			err = processErrorMessageAssertion(response, assertion, state)
		default:
			return fmt.Errorf("unknown assertion type: %s", assertionType)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// processStatusCodeAssertion validates HTTP status code
func processStatusCodeAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expectedInterface, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for status_code assertion")
	}

	var expected int
	switch v := expectedInterface.(type) {
	case float64:
		expected = int(v)
	case int:
		expected = v
	case string:
		// Try to parse or replace variables
		str := replaceVariables(v, state)
		parsed, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("invalid expected status code: %s", str)
		}
		expected = parsed
	default:
		return fmt.Errorf("invalid expected status code type")
	}

	actual := response.Metadata.StatusCode
	if actual != expected {
		return fmt.Errorf("status code assertion failed: expected %d, got %d", expected, actual)
	}

	return nil
}

// processJSONPathAssertion validates JSON path value
func processJSONPathAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	path, ok := assertion["path"].(string)
	if !ok {
		return fmt.Errorf("path is required for json_path assertion")
	}

	expectedInterface, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for json_path assertion")
	}

	// Parse JSON path
	query, err := gojq.Parse(path)
	if err != nil {
		return fmt.Errorf("failed to parse JSON path %s: %w", path, err)
	}

	// Run query on response data
	iter := query.Run(response.Data)
	actual, ok := iter.Next()
	if !ok {
		return fmt.Errorf("JSON path %s returned no results", path)
	}
	if err, ok := actual.(error); ok {
		return fmt.Errorf("error evaluating JSON path %s: %w", path, err)
	}

	// Process expected value with variable replacement
	var expected = expectedInterface
	if expectedStr, ok := expectedInterface.(string); ok {
		expected = replaceVariables(expectedStr, state)

		// Try to convert to same type as actual
		switch actual.(type) {
		case float64, int:
			if num, err := strconv.ParseFloat(expected.(string), 64); err == nil {
				expected = num
			}
		case bool:
			if b, err := strconv.ParseBool(expected.(string)); err == nil {
				expected = b
			}
		}
	}

	// Compare values
	if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
		return fmt.Errorf("JSON path assertion failed for %s: expected %v, got %v", path, expected, actual)
	}

	return nil
}

// processRowCountAssertion validates row count
func processRowCountAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expectedInterface, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for row_count assertion")
	}

	var expected int
	switch v := expectedInterface.(type) {
	case float64:
		expected = int(v)
	case int:
		expected = v
	case string:
		str := replaceVariables(v, state)
		parsed, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("invalid expected row count: %s", str)
		}
		expected = parsed
	default:
		return fmt.Errorf("invalid expected row count type")
	}

	if response.Count == nil {
		return fmt.Errorf("row count assertion failed: no count in response")
	}

	actual := *response.Count
	if actual != expected {
		return fmt.Errorf("row count assertion failed: expected %d, got %d", expected, actual)
	}

	return nil
}

// processErrorCodeAssertion validates error code
func processErrorCodeAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expectedInterface, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for error_code assertion")
	}

	var expected string
	if expectedStr, ok := expectedInterface.(string); ok {
		expected = replaceVariables(expectedStr, state)
	} else {
		expected = fmt.Sprintf("%v", expectedInterface)
	}

	if response.Error == nil {
		return fmt.Errorf("error code assertion failed: no error in response")
	}

	if response.Error.Code != expected {
		return fmt.Errorf("error code assertion failed: expected %s, got %s", expected, response.Error.Code)
	}

	return nil
}

// processErrorMessageAssertion validates error message
func processErrorMessageAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expectedInterface, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for error_message assertion")
	}

	var expected string
	if expectedStr, ok := expectedInterface.(string); ok {
		expected = replaceVariables(expectedStr, state)
	} else {
		expected = fmt.Sprintf("%v", expectedInterface)
	}

	if response.Error == nil {
		return fmt.Errorf("error message assertion failed: no error in response")
	}

	if response.Error.Message != expected {
		return fmt.Errorf("error message assertion failed: expected %s, got %s", expected, response.Error.Message)
	}

	return nil
}
