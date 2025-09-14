package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
	"go.temporal.io/sdk/activity"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&HTTPPlugin{})
}

// ActivityResponse represents the response from the HTTP activity
type ActivityResponse struct {
	Response *HTTPResponse     `json:"response"`
	Saved    map[string]string `json:"saved"`
}

// replaceVariables replaces {{ variable }} patterns in the input string with values from the state
// Now uses DSL template functions to properly handle escaped handlebars
func replaceVariables(input string, state map[string]string) (string, error) {
	// Convert state to interface{} map for DSL functions
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}

	// Create template context with only runtime variables (config vars already processed by CLI)
	context := dsl.TemplateContext{
		Runtime: runtime,
	}

	// Use DSL template processing which handles escaped handlebars
	result, err := dsl.ProcessTemplate(input, context)
	if err != nil {
		availableVars := getStateKeys(state)
		return "", fmt.Errorf("undefined variables: %v. Available runtime variables: %v", extractMissingVars(err), availableVars)
	}

	return result, nil
}

// extractMissingVars extracts variable names from template execution errors
func extractMissingVars(err error) []string {
	// For now, just return the error string
	// TODO: Parse Go template errors more intelligently
	return []string{err.Error()}
}

// getStateKeys returns a sorted list of keys from the state map
func getStateKeys(state map[string]string) []string {
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (hp *HTTPPlugin) GetType() string {
	return "http"
}

func (hp *HTTPPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("[DEBUG] Activity input parameters:", "params", fmt.Sprintf("%+v", p))

	// Get state from input parameters with improved type handling
	state := make(map[string]string)
	if stateData, ok := p["state"].(map[string]interface{}); ok {
		for k, v := range stateData {
			switch val := v.(type) {
			case string:
				state[k] = val
			case float64:
				state[k] = fmt.Sprintf("%.0f", val) // Remove decimal for whole numbers
			case bool:
				state[k] = fmt.Sprintf("%t", val)
			case nil:
				state[k] = ""
			default:
				// For complex types, use JSON marshaling
				bytes, err := json.Marshal(val)
				if err != nil {
					return nil, fmt.Errorf("failed to convert state value for %s: %w", k, err)
				}
				state[k] = string(bytes)
			}
			logger.Info(fmt.Sprintf("[DEBUG] Loaded state[%s] = %s (type: %T)", k, state[k], v))
		}
	}
	logger.Info(fmt.Sprintf("[DEBUG] Loaded state: %v", state))

	// Parse the plugin configuration
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format: got type %T", p["config"])
	}

    // Replace variables in URL
    urlStr, ok := configData["url"].(string)
    if !ok {
        return nil, fmt.Errorf("url is required")
    }
    urlStr, err := replaceVariables(urlStr, state)
    if err != nil {
        return nil, fmt.Errorf("failed to replace variables in URL: %w", err)
    }


	// Build request body
	var body io.Reader
	isForm := false
	// Prefer form encoding when provided
	if formData, ok := configData["form"].(map[string]interface{}); ok && len(formData) > 0 {
		values := url.Values{}
		for k, v := range formData {
			switch val := v.(type) {
			case string:
				// Apply runtime variable replacement for string values
				replaced, rerr := replaceVariables(val, state)
				if rerr != nil {
					return nil, fmt.Errorf("failed to replace variables in form field %s: %w", k, rerr)
				}
				values.Add(k, replaced)
			case []interface{}:
				for _, elem := range val {
					str := fmt.Sprint(elem)
					// Try replacement on strings only
					if s, ok := elem.(string); ok {
						if rep, rerr := replaceVariables(s, state); rerr == nil {
							str = rep
						}
					}
					values.Add(k, str)
				}
			default:
				values.Add(k, fmt.Sprint(val))
			}
		}
		encoded := values.Encode()
		body = strings.NewReader(encoded)
		isForm = true
	} else if bodyStr, ok := configData["body"].(string); ok && bodyStr != "" {
		// Raw body path
		bodyStr, err = replaceVariables(bodyStr, state)
		if err != nil {
			return nil, fmt.Errorf("failed to replace variables in body: %w", err)
		}
		body = bytes.NewReader([]byte(bodyStr))
	}

	method, ok := configData["method"].(string)
	if !ok {
		return nil, fmt.Errorf("method is required")
	}

    // Create request
    req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers with variable replacement
	if headers, ok := configData["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				strValue, err = replaceVariables(strValue, state)
				if err != nil {
					return nil, fmt.Errorf("failed to replace variables in header %s: %w", key, err)
				}
				req.Header.Add(key, strValue)
			}
		}
	}

	// Default Content-Type for form submissions if not explicitly set
	if isForm && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Debug: Print the final HTTP request details
	logger.Info("=== HTTP REQUEST DEBUG ===")
	logger.Info("Final URL:", "url", req.URL.String())
	logger.Info("Method:", "method", req.Method)

	// Print all headers
	for headerName, headerValues := range req.Header {
		for _, headerValue := range headerValues {
			logger.Info("Header:", "name", headerName, "value", headerValue)
		}
	}

	// Print body if present
	if req.Body != nil {
		// Read body for logging (we need to recreate it since it's consumed)
		if bodyBytes, err := io.ReadAll(req.Body); err == nil {
			bodyStr := string(bodyBytes)
			logger.Info("Body:", "content", bodyStr)
			// Recreate the body reader for the actual request
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	} else {
		logger.Info("Body: <empty>")
	}
	logger.Info("=== END HTTP REQUEST DEBUG ===")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Create response object
	response := &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Body:       string(respBody),
	}

	// Copy headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	// Process assertions
	if err := hp.processAssertions(p, resp, respBody); err != nil {
		return nil, err
	}

	// Process saves
	saved := make(map[string]string)
	if err := hp.processSaves(p, resp, respBody, saved); err != nil {
		return nil, err
	}

	return &ActivityResponse{
		Response: response,
		Saved:    saved,
	}, nil
}

// buildRequest is a helper to construct an HTTP request from config and state.
// It's used by tests to validate request construction without requiring Temporal context.
func buildRequest(method, urlStr string, configData map[string]interface{}, state map[string]string) (*http.Request, error) {
    var err error
    var body io.Reader
    isForm := false

    if formData, ok := configData["form"].(map[string]interface{}); ok && len(formData) > 0 {
        values := url.Values{}
        for k, v := range formData {
            switch val := v.(type) {
            case string:
                replaced, rerr := replaceVariables(val, state)
                if rerr != nil {
                    return nil, fmt.Errorf("failed to replace variables in form field %s: %w", k, rerr)
                }
                values.Add(k, replaced)
            case []interface{}:
                for _, elem := range val {
                    str := fmt.Sprint(elem)
                    if s, ok := elem.(string); ok {
                        if rep, rerr := replaceVariables(s, state); rerr == nil {
                            str = rep
                        }
                    }
                    values.Add(k, str)
                }
            default:
                values.Add(k, fmt.Sprint(val))
            }
        }
        encoded := values.Encode()
        body = strings.NewReader(encoded)
        isForm = true
    } else if bodyStr, ok := configData["body"].(string); ok && bodyStr != "" {
        bodyStr, err = replaceVariables(bodyStr, state)
        if err != nil {
            return nil, fmt.Errorf("failed to replace variables in body: %w", err)
        }
        body = bytes.NewReader([]byte(bodyStr))
    }

    req, err := http.NewRequest(method, urlStr, body)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    if headers, ok := configData["headers"].(map[string]interface{}); ok {
        for key, value := range headers {
            if strValue, ok := value.(string); ok {
                strValue, err = replaceVariables(strValue, state)
                if err != nil {
                    return nil, fmt.Errorf("failed to replace variables in header %s: %w", key, err)
                }
                req.Header.Add(key, strValue)
            }
        }
    }

    if isForm && req.Header.Get("Content-Type") == "" {
        req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    }
    return req, nil
}

func (hp *HTTPPlugin) processSaves(p map[string]interface{}, resp *http.Response, respBody []byte, saved map[string]string) error {
	saves, ok := p["save"].([]interface{})
	if !ok {
		log.Printf("[DEBUG] No saves configured")
		return nil
	}

	log.Printf("[DEBUG] Processing %d saves", len(saves))
	for _, save := range saves {
		saveMap, ok := save.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid save format: got type %T", save)
		}

		as, ok := saveMap["as"].(string)
		if !ok {
			return fmt.Errorf("'as' field is required for save")
		}

		// Check if required is explicitly set to false
		required := true
		if req, ok := saveMap["required"].(bool); ok {
			required = req
		}

		// Handle JSON path save
		if jsonPath, ok := saveMap["json_path"].(string); ok && jsonPath != "" {
			log.Printf("[DEBUG] Processing JSON path save: '%s' as %s", jsonPath, as)
			var jsonData interface{}
			if err := json.Unmarshal(respBody, &jsonData); err != nil {
				log.Printf("[ERROR] Failed to parse response body as JSON: %v\nBody: %s", err, string(respBody))
				return fmt.Errorf("failed to parse response body as JSON for save: %w", err)
			}

			query, err := gojq.Parse(jsonPath)
			if err != nil {
				log.Printf("[ERROR] Failed to parse jq expression: %v", err)
				return fmt.Errorf("failed to parse jq expression %q: %w", jsonPath, err)
			}

			iter := query.Run(jsonData)
			v, ok := iter.Next()
			if !ok {
				if required {
					log.Printf("[ERROR] No results from required jq expression %q. Response body: %s", jsonPath, string(respBody))
					return fmt.Errorf("no results from required jq expression %q", jsonPath)
				}
				log.Printf("[WARN] No results from optional jq expression %q, skipping save", jsonPath)
				continue
			}
			if err, ok := v.(error); ok {
				log.Printf("[ERROR] Error evaluating jq expression: %v", err)
				return fmt.Errorf("error evaluating jq expression %q: %w", jsonPath, err)
			}

			// Improved type handling for different value types
			switch val := v.(type) {
			case string:
				saved[as] = val
			case float64:
				saved[as] = fmt.Sprintf("%.0f", val) // Remove decimal for whole numbers
			case bool:
				saved[as] = fmt.Sprintf("%t", val)
			case nil:
				if required {
					return fmt.Errorf("required value for %q is null", as)
				}
				saved[as] = ""
			default:
				// For complex types, use JSON marshaling
				bytes, err := json.Marshal(val)
				if err != nil {
					log.Printf("[ERROR] Failed to marshal value: %v", err)
					return fmt.Errorf("failed to marshal value for %s: %w", as, err)
				}
				saved[as] = string(bytes)
			}
			log.Printf("[DEBUG] Saved value for %s: %s (type: %T)", as, saved[as], v)
			continue
		}

		// Handle header save
		if headerName, ok := saveMap["header"].(string); ok {
			log.Printf("[DEBUG] Processing header save: %s as %s", headerName, as)
			headerValue := resp.Header.Get(headerName)
			if headerValue == "" && required {
				log.Printf("[ERROR] Required header %s not found in response", headerName)
				return fmt.Errorf("required header %s not found in response", headerName)
			}
			saved[as] = headerValue
			log.Printf("[DEBUG] Saved value for %s: %s", as, saved[as])
			continue
		}

		return fmt.Errorf("save configuration must specify either json_path or header")
	}

	log.Printf("[DEBUG] Final saved values: %v", saved)
	return nil
}

func (hp *HTTPPlugin) processAssertions(p map[string]interface{}, resp *http.Response, respBody []byte) error {
	assertions, ok := p["assertions"].([]interface{})
	if !ok {
		return nil
	}

	// Get state from input parameters
	state := make(map[string]string)
	if stateData, ok := p["state"].(map[string]interface{}); ok {
		for k, v := range stateData {
			switch val := v.(type) {
			case string:
				state[k] = val
			case float64:
				state[k] = fmt.Sprintf("%.0f", val)
			case bool:
				state[k] = fmt.Sprintf("%t", val)
			case nil:
				state[k] = ""
			default:
				bytes, err := json.Marshal(val)
				if err != nil {
					return fmt.Errorf("failed to convert state value for %s: %w", k, err)
				}
				state[k] = string(bytes)
			}
		}
	}

	for _, assertion := range assertions {
		assertionMap, ok := assertion.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid assertion format: got type %T", assertion)
		}

		assertionType, ok := assertionMap["type"].(string)
		if !ok {
			return fmt.Errorf("assertion type is required")
		}

		// Replace variables in expected value if it's a string
		if expectedStr, ok := assertionMap["expected"].(string); ok {
			expectedStr, err := replaceVariables(expectedStr, state)
			if err != nil {
				return fmt.Errorf("failed to replace variables in expected value: %w", err)
			}
			assertionMap["expected"] = expectedStr
		}

		switch assertionType {
		case AssertionTypeStatusCode:
			expected, ok := assertionMap["expected"].(float64)
			if !ok {
				return fmt.Errorf("status code assertion expected value must be a number: got type %T", assertionMap["expected"])
			}
			if int(expected) != resp.StatusCode {
				// Include response body in error for better debugging
				bodyPreview := string(respBody)
				if len(bodyPreview) > 500 {
					bodyPreview = bodyPreview[:500] + "..."
				}
				if bodyPreview != "" {
					return fmt.Errorf("status code assertion failed: expected %d, got %d. Response body: %s", int(expected), resp.StatusCode, bodyPreview)
				}
				return fmt.Errorf("status code assertion failed: expected %d, got %d", int(expected), resp.StatusCode)
			}

		case AssertionTypeHeader:
			headerName, ok := assertionMap["name"].(string)
			if !ok {
				return fmt.Errorf("header name is required for header assertion")
			}

			expected, ok := assertionMap["expected"].(string)
			if !ok {
				return fmt.Errorf("header value must be a string")
			}

			actual := resp.Header.Get(headerName)
			if actual != expected {
				return fmt.Errorf("header assertion failed for %q: expected %q, got %q", headerName, expected, actual)
			}

		case AssertionTypeJSONPath:
			// Handle JSON path assertions
			var jsonData interface{}
			if err := json.Unmarshal(respBody, &jsonData); err != nil {
				return fmt.Errorf("failed to parse response body as JSON: %w", err)
			}

			path, ok := assertionMap["path"].(string)
			if !ok {
				return fmt.Errorf("path is required for JSONPath assertion")
			}

			// Replace variables in path field before parsing as jq expression
			path, err := replaceVariables(path, state)
			if err != nil {
				return fmt.Errorf("failed to replace variables in path field: %w", err)
			}

			query, err := gojq.Parse(path)
			if err != nil {
				return fmt.Errorf("failed to parse jq expression %q: %w", path, err)
			}

			iter := query.Run(jsonData)
			var result interface{}
			var found bool

			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := v.(error); ok {
					return fmt.Errorf("error evaluating jq expression %q: %w", path, err)
				}
				if !found {
					result = v
					found = true
				}
			}

			// If we're just checking existence
			if exists, ok := assertionMap["exists"].(bool); ok && exists {
				if !found {
					// Include a preview of the response body for debugging
					bodyPreview := string(respBody)
					if len(bodyPreview) > 200 {
						bodyPreview = bodyPreview[:200] + "..."
					}
					return fmt.Errorf("jq assertion failed: path %q does not exist. Response body: %s", path, bodyPreview)
				}
				// Skip value comparison if we're only checking existence
				continue
			}

			if !found {
				// Include a preview of the response body for debugging
				bodyPreview := string(respBody)
				if len(bodyPreview) > 200 {
					bodyPreview = bodyPreview[:200] + "..."
				}
				return fmt.Errorf("no results from jq expression %q. Response body: %s", path, bodyPreview)
			}

			// Only compare values if we have an expected value
			if expected, hasExpected := assertionMap["expected"]; hasExpected {
				equal := false
				switch v := result.(type) {
				case int:
					if exp, ok := expected.(float64); ok {
						equal = float64(v) == exp
					} else if exp, ok := expected.(int); ok {
						equal = v == exp
					}
				case float64:
					if exp, ok := expected.(float64); ok {
						equal = v == exp
					} else if exp, ok := expected.(int); ok {
						equal = v == float64(exp)
					}
				case string:
					if exp, ok := expected.(string); ok {
						equal = v == exp
					}
				default:
					equal = result == expected
				}

				if !equal {
					return fmt.Errorf("jq assertion failed: path %q expected %v (type %T), got %v (type %T)",
						path, expected, expected, result, result)
				}
			}
		default:
			return fmt.Errorf("unknown assertion type: %s", assertionType)
		}
	}

	return nil
}
