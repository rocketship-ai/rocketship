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
	"go.temporal.io/sdk/temporal"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&HTTPPlugin{})
}

// ActivityResponse represents the response from the HTTP activity
type ActivityResponse struct {
	Response          *HTTPResponse         `json:"response"`
	Saved             map[string]string     `json:"saved"`
	UIPayload         *UIPayload            `json:"ui_payload,omitempty"`
	AssertionResults  []HTTPAssertionResult `json:"assertion_results,omitempty"`
	AssertionFailed   bool                  `json:"assertion_failed,omitempty"`
	AssertionError    string                `json:"assertion_error,omitempty"`
}

// replaceVariables replaces {{ variable }} patterns in the input string with values from the state
// Now uses DSL template functions to properly handle escaped handlebars
// env parameter provides environment secrets from project environment for {{ .env.* }} resolution
func replaceVariables(input string, state map[string]string, env map[string]string) (string, error) {
	// Convert state to interface{} map for DSL functions
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}

	// Create template context with runtime variables and env secrets
	context := dsl.TemplateContext{
		Runtime: runtime,
		Env:     env,
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

// sensitiveHeaders is the list of header names (case-insensitive) that should be redacted
var sensitiveHeaders = map[string]bool{
	"authorization": true,
	"cookie":        true,
	"x-api-key":     true,
	"x-auth-token":  true,
}

// maxUIBodyBytes is the maximum body size to include in UI payload (8KB)
const maxUIBodyBytes = 8 * 1024

// redactHeaders returns a copy of headers with sensitive values redacted
func redactHeaders(headers map[string]string) map[string]string {
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// truncateBody returns a truncated body if it exceeds maxUIBodyBytes
func truncateBody(body string) (truncated string, wasTruncated bool, originalBytes int) {
	originalBytes = len(body)
	if originalBytes <= maxUIBodyBytes {
		return body, false, originalBytes
	}
	return body[:maxUIBodyBytes], true, originalBytes
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

	// Extract env secrets from params (for {{ .env.* }} template resolution)
	env := make(map[string]string)
	if envData, ok := p["env"].(map[string]interface{}); ok {
		for k, v := range envData {
			if strVal, ok := v.(string); ok {
				env[k] = strVal
			}
		}
	} else if envData, ok := p["env"].(map[string]string); ok {
		env = envData
	}

	// Parse the plugin configuration
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format: got type %T", p["config"])
	}

	var suiteOpenAPI map[string]interface{}
	if rawSuite, exists := p["suite_openapi"]; exists && rawSuite != nil {
		var ok bool
		suiteOpenAPI, ok = rawSuite.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("suite_openapi config must be an object when provided")
		}
	}

	// Replace variables in URL
	urlStr, ok := configData["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required")
	}
	urlStr, err := replaceVariables(urlStr, state, env)
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
				replaced, rerr := replaceVariables(val, state, env)
				if rerr != nil {
					return nil, fmt.Errorf("failed to replace variables in form field %s: %w", k, rerr)
				}
				values.Add(k, replaced)
			case []interface{}:
				for _, elem := range val {
					str := fmt.Sprint(elem)
					// Try replacement on strings only
					if s, ok := elem.(string); ok {
						if rep, rerr := replaceVariables(s, state, env); rerr == nil {
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
		bodyStr, err = replaceVariables(bodyStr, state, env)
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
				strValue, err = replaceVariables(strValue, state, env)
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

	var reqBodyBytes []byte
	if req.Body != nil {
		bodyBytes, readErr := io.ReadAll(req.Body)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read request body: %w", readErr)
		}
		reqBodyBytes = bodyBytes
		logger.Info("Body:", "content", string(reqBodyBytes))
	} else {
		logger.Info("Body: <empty>")
	}
	setRequestBody(req, reqBodyBytes)
	logger.Info("=== END HTTP REQUEST DEBUG ===")

	var openapiValidator *openAPIValidator
	if validator, err := newOpenAPIValidator(ctx, configData, suiteOpenAPI, state, env); err != nil {
		return nil, err
	} else {
		openapiValidator = validator
	}

	if openapiValidator != nil {
		if err := openapiValidator.prepareRequestValidation(ctx, req, reqBodyBytes); err != nil {
			return nil, err
		}
		if openapiValidator.shouldValidateRequest() {
			if err := openapiValidator.validateRequest(ctx); err != nil {
				return nil, err
			}
		}
	}

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

	if openapiValidator != nil && openapiValidator.shouldValidateResponse() {
		if err := openapiValidator.validateResponse(ctx, resp, respBody); err != nil {
			return nil, err
		}
	}

	// Process assertions - collect results without failing the activity
	assertionResults, assertionFailed, assertionError := hp.processAssertionsWithResults(p, resp, respBody)

	// Process saves - we still do this even if assertions failed so we capture all data
	saved := make(map[string]string)
	if err := hp.processSaves(p, resp, respBody, saved); err != nil {
		return nil, err
	}

	// Build UI payload with request/response details for the web UI
	// Apply redaction and truncation for security and storage efficiency

	// Prepare request headers for UI (flatten multi-value headers)
	reqHeaders := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			reqHeaders[key] = values[0]
		}
	}

	// Truncate request body if needed
	reqBodyStr, reqTruncated, reqOrigBytes := truncateBody(string(reqBodyBytes))
	// Truncate response body if needed
	respBodyStr, respTruncated, respOrigBytes := truncateBody(string(respBody))

	uiPayload := &UIPayload{
		Request: &UIRequestData{
			Method:        method,
			URL:           urlStr,
			Headers:       redactHeaders(reqHeaders),
			Body:          reqBodyStr,
			BodyTruncated: reqTruncated,
			BodyBytes:     reqOrigBytes,
		},
		Response: &UIResponseData{
			StatusCode:    resp.StatusCode,
			Headers:       redactHeaders(response.Headers),
			Body:          respBodyStr,
			BodyTruncated: respTruncated,
			BodyBytes:     respOrigBytes,
		},
	}

	// Preserve Temporal retry semantics for assertion failures:
	// return a retryable activity error so Temporal can perform retries according to the step retry policy.
	// Include rich UI payload + assertion results + saved values as error details so the workflow can persist them.
	if assertionFailed {
		details := map[string]interface{}{
			"ui_payload":        uiPayload,
			"assertion_results": assertionResults,
			"saved":             saved,
		}
		return nil, temporal.NewApplicationError(assertionError, "http_assertion_failed", details)
	}

	return &ActivityResponse{
		Response:         response,
		Saved:            saved,
		UIPayload:        uiPayload,
		AssertionResults: assertionResults,
		AssertionFailed:  false,
		AssertionError:   "",
	}, nil
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

// processAssertionsWithResults evaluates all assertions and returns structured results
// Returns (results, hasFailed, errorSummary) - never returns an error so the activity can complete
func (hp *HTTPPlugin) processAssertionsWithResults(p map[string]interface{}, resp *http.Response, respBody []byte) ([]HTTPAssertionResult, bool, string) {
	assertions, ok := p["assertions"].([]interface{})
	if !ok || len(assertions) == 0 {
		return nil, false, ""
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
					state[k] = fmt.Sprintf("%v", val)
				} else {
					state[k] = string(bytes)
				}
			}
		}
	}

	// Extract env secrets from params (for {{ .env.* }} template resolution)
	env := make(map[string]string)
	if envData, ok := p["env"].(map[string]interface{}); ok {
		for k, v := range envData {
			if strVal, ok := v.(string); ok {
				env[k] = strVal
			}
		}
	} else if envData, ok := p["env"].(map[string]string); ok {
		env = envData
	}

	var results []HTTPAssertionResult
	var hasFailed bool
	var failedMessages []string

	for _, assertion := range assertions {
		assertionMap, ok := assertion.(map[string]interface{})
		if !ok {
			results = append(results, HTTPAssertionResult{
				Type:    "unknown",
				Passed:  false,
				Message: fmt.Sprintf("invalid assertion format: got type %T", assertion),
			})
			hasFailed = true
			failedMessages = append(failedMessages, "invalid assertion format")
			continue
		}

		assertionType, ok := assertionMap["type"].(string)
		if !ok {
			results = append(results, HTTPAssertionResult{
				Type:    "unknown",
				Passed:  false,
				Message: "assertion type is required",
			})
			hasFailed = true
			failedMessages = append(failedMessages, "assertion type is required")
			continue
		}

		// Replace variables in expected value if it's a string
		expected := assertionMap["expected"]
		if expectedStr, ok := expected.(string); ok {
			if replaced, err := replaceVariables(expectedStr, state, env); err == nil {
				expected = replaced
			}
		}

		result := HTTPAssertionResult{
			Type:     assertionType,
			Expected: expected,
		}

		switch assertionType {
		case AssertionTypeStatusCode:
			expectedCode, ok := expected.(float64)
			if !ok {
				result.Passed = false
				result.Message = fmt.Sprintf("status code assertion expected value must be a number: got type %T", expected)
				result.Actual = resp.StatusCode
			} else {
				result.Actual = resp.StatusCode
				if int(expectedCode) == resp.StatusCode {
					result.Passed = true
				} else {
					result.Passed = false
					result.Message = fmt.Sprintf("expected %d, got %d", int(expectedCode), resp.StatusCode)
				}
			}

		case AssertionTypeHeader:
			headerName, ok := assertionMap["name"].(string)
			if !ok {
				result.Passed = false
				result.Message = "header name is required for header assertion"
			} else {
				result.Name = headerName
				expectedVal, ok := expected.(string)
				if !ok {
					result.Passed = false
					result.Message = "header value must be a string"
				} else {
					actual := resp.Header.Get(headerName)
					result.Actual = actual
					if actual == expectedVal {
						result.Passed = true
					} else {
						result.Passed = false
						result.Message = fmt.Sprintf("expected %q, got %q", expectedVal, actual)
					}
				}
			}

		case AssertionTypeJSONPath:
			path, ok := assertionMap["path"].(string)
			if !ok {
				result.Passed = false
				result.Message = "path is required for JSONPath assertion"
			} else {
				// Replace variables in path
				if replaced, err := replaceVariables(path, state, env); err == nil {
					path = replaced
				}
				result.Path = path

				var jsonData interface{}
				if err := json.Unmarshal(respBody, &jsonData); err != nil {
					result.Passed = false
					result.Message = fmt.Sprintf("failed to parse response body as JSON: %v", err)
				} else {
					query, err := gojq.Parse(path)
					if err != nil {
						result.Passed = false
						result.Message = fmt.Sprintf("failed to parse jq expression: %v", err)
					} else {
						iter := query.Run(jsonData)
						var queryResult interface{}
						var found bool

						for {
							v, ok := iter.Next()
							if !ok {
								break
							}
							if err, ok := v.(error); ok {
								result.Passed = false
								result.Message = fmt.Sprintf("error evaluating jq expression: %v", err)
								break
							}
							if !found {
								queryResult = v
								found = true
							}
						}

						result.Actual = queryResult

						// If we're just checking existence
						if exists, ok := assertionMap["exists"].(bool); ok && exists {
							if found {
								result.Passed = true
							} else {
								result.Passed = false
								result.Message = fmt.Sprintf("path %q does not exist", path)
							}
						} else if !found {
							result.Passed = false
							result.Message = fmt.Sprintf("no results from jq expression %q", path)
						} else if expected != nil {
							// Compare values
							equal := false
							switch v := queryResult.(type) {
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
								equal = queryResult == expected
							}

							if equal {
								result.Passed = true
							} else {
								result.Passed = false
								result.Message = fmt.Sprintf("expected %v, got %v", expected, queryResult)
							}
						} else {
							// No expected value and we found a result
							result.Passed = true
						}
					}
				}
			}

		default:
			result.Passed = false
			result.Message = fmt.Sprintf("unknown assertion type: %s", assertionType)
		}

		results = append(results, result)
		if !result.Passed {
			hasFailed = true
			if result.Message != "" {
				failedMessages = append(failedMessages, fmt.Sprintf("%s: %s", result.Type, result.Message))
			}
		}
	}

	var errorSummary string
	if hasFailed && len(failedMessages) > 0 {
		if len(failedMessages) == 1 {
			errorSummary = fmt.Sprintf("Assertion failed: %s", failedMessages[0])
		} else {
			errorSummary = fmt.Sprintf("Assertions failed: %s", strings.Join(failedMessages, "; "))
		}
	}

	return results, hasFailed, errorSummary
}

// processAssertions is kept for backward compatibility but now uses the new implementation
func (hp *HTTPPlugin) processAssertions(p map[string]interface{}, resp *http.Response, respBody []byte) error {
	results, hasFailed, errorSummary := hp.processAssertionsWithResults(p, resp, respBody)
	if hasFailed {
		// Find first failure for detailed error message
		for _, r := range results {
			if !r.Passed && r.Message != "" {
				return fmt.Errorf("%s", r.Message)
			}
		}
		return fmt.Errorf("%s", errorSummary)
	}
	return nil
}
