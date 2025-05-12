package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchyny/gojq"
	"go.temporal.io/sdk/activity"
)

func (hp *HTTPPlugin) GetType() string {
	return "http"
}

func (hp *HTTPPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Debug input parameters
	logger.Info("[DEBUG] Activity input parameters:", "params", fmt.Sprintf("%+v", p))

	// Parse the plugin configuration from the parameters
	pluginData := p

	// Debug plugin data
	logger.Info("[DEBUG] Plugin data:", "data", fmt.Sprintf("%+v", pluginData))

	configData, ok := pluginData["config"].(map[string]interface{})
	if !ok {
		rawConfig := pluginData["config"]
		logger.Error("[DEBUG] Failed to parse config data",
			"config_type", fmt.Sprintf("%T", rawConfig),
			"raw_config", fmt.Sprintf("%+v", rawConfig))
		return nil, fmt.Errorf("invalid config format: got type %T", rawConfig)
	}

	// Debug config data
	logger.Info("[DEBUG] Config data:", "config", fmt.Sprintf("%+v", configData))

	method, ok := configData["method"].(string)
	if !ok {
		rawMethod := configData["method"]
		logger.Error("[DEBUG] Failed to parse method",
			"method_type", fmt.Sprintf("%T", rawMethod),
			"raw_method", fmt.Sprintf("%+v", rawMethod))
		return nil, fmt.Errorf("method is required: got type %T", rawMethod)
	}

	url, ok := configData["url"].(string)
	if !ok {
		rawURL := configData["url"]
		logger.Error("[DEBUG] Failed to parse URL",
			"url_type", fmt.Sprintf("%T", rawURL),
			"raw_url", fmt.Sprintf("%+v", rawURL))
		return nil, fmt.Errorf("url is required: got type %T", rawURL)
	}

	logger.Info("[DEBUG] Starting HTTP request: " + method + " " + url)

	// Create HTTP request
	var body io.Reader
	if bodyStr, ok := configData["body"].(string); ok && bodyStr != "" {
		body = bytes.NewReader([]byte(bodyStr))
		logger.Info("[DEBUG] Request body: " + bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if headers, ok := configData["headers"].(map[string]interface{}); ok {
		logger.Info("[DEBUG] Processing headers:", "headers", fmt.Sprintf("%+v", headers))
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Add(key, strValue)
				logger.Info(fmt.Sprintf("[DEBUG] Request header: %s: %s", key, strValue))
			} else {
				logger.Error("[DEBUG] Invalid header value type",
					"key", key,
					"value_type", fmt.Sprintf("%T", value),
					"raw_value", fmt.Sprintf("%+v", value))
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

	// FOR DEBUGGING
	logger.Info(fmt.Sprintf("[DEBUG] Response status code: %d", resp.StatusCode))
	respBodyStr := string(respBody)
	if len(respBodyStr) > 280 {
		respBodyStr = respBodyStr[:280] + "..."
	}
	logger.Info("[DEBUG] Response body: " + respBodyStr)
	// END FOR DEBUGGING

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
			logger.Info(fmt.Sprintf("[DEBUG] Response header: %s: %s", key, values[0]))
		}
	}

	// Process assertions
	assertions, ok := pluginData["assertions"].([]interface{})
	if !ok {
		rawAssertions := pluginData["assertions"]
		logger.Info("[DEBUG] No assertions or invalid format",
			"assertions_type", fmt.Sprintf("%T", rawAssertions),
			"raw_assertions", fmt.Sprintf("%+v", rawAssertions))
		return response, nil
	}

	logger.Info(fmt.Sprintf("[DEBUG] Processing %d assertions", len(assertions)))
	for i, assertion := range assertions {
		logger.Info("[DEBUG] Processing assertion",
			"index", i,
			"assertion_type", fmt.Sprintf("%T", assertion),
			"raw_assertion", fmt.Sprintf("%+v", assertion))

		assertionMap, ok := assertion.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid assertion format: got type %T", assertion)
		}

		assertionType, ok := assertionMap["type"].(string)
		if !ok {
			rawType := assertionMap["type"]
			logger.Error("[DEBUG] Failed to parse assertion type",
				"type_type", fmt.Sprintf("%T", rawType),
				"raw_type", fmt.Sprintf("%+v", rawType))
			return nil, fmt.Errorf("assertion type is required: got type %T", rawType)
		}

		logger.Info(fmt.Sprintf("[DEBUG] Processing assertion %d: type=%s", i+1, assertionType))

		switch assertionType {
		case AssertionTypeStatusCode:
			expected, ok := assertionMap["expected"].(float64)
			if !ok {
				rawExpected := assertionMap["expected"]
				logger.Error("[DEBUG] Status code assertion failed: expected value is not a number",
					"expected_type", fmt.Sprintf("%T", rawExpected),
					"raw_expected", fmt.Sprintf("%+v", rawExpected))
				return nil, fmt.Errorf("status code assertion expected value must be a number: got type %T", rawExpected)
			}
			logger.Info(fmt.Sprintf("[DEBUG] Comparing status code: expected=%d, got=%d", int(expected), resp.StatusCode))
			if int(expected) != resp.StatusCode {
				logger.Error("[DEBUG] Status code assertion failed", "expected", int(expected), "got", resp.StatusCode)
				return nil, fmt.Errorf("status code assertion failed: expected %d, got %d", int(expected), resp.StatusCode)
			}
			logger.Info("[DEBUG] Status code assertion passed")

		case AssertionTypeJSONPath:
			// Parse response body as JSON
			var jsonData interface{}
			if err := json.Unmarshal(respBody, &jsonData); err != nil {
				logger.Error("[DEBUG] Failed to parse response body as JSON",
					"error", err,
					"response_body", string(respBody))
				return nil, fmt.Errorf("failed to parse response body as JSON: %w", err)
			}

			path, ok := assertionMap["path"].(string)
			if !ok {
				rawPath := assertionMap["path"]
				logger.Error("[DEBUG] Failed to parse JSONPath",
					"path_type", fmt.Sprintf("%T", rawPath),
					"raw_path", fmt.Sprintf("%+v", rawPath))
				return nil, fmt.Errorf("path is required for JSONPath assertion: got type %T", rawPath)
			}

			logger.Info("[DEBUG] Evaluating jq expression: " + path)

			// Parse and run jq query
			query, err := gojq.Parse(path)
			if err != nil {
				logger.Error("[DEBUG] Failed to parse jq expression",
					"path", path,
					"error", err)
				return nil, fmt.Errorf("failed to parse jq expression %q: %w", path, err)
			}

			expected := assertionMap["expected"]
			iter := query.Run(jsonData)
			var result interface{}
			var found bool

			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := v.(error); ok {
					logger.Error("[DEBUG] Error evaluating jq expression",
						"path", path,
						"error", err)
					return nil, fmt.Errorf("error evaluating jq expression %q: %w", path, err)
				}
				// Take the first result
				if !found {
					result = v
					found = true
				}
			}

			if !found {
				logger.Error("[DEBUG] No results from jq expression",
					"path", path)
				return nil, fmt.Errorf("no results from jq expression %q", path)
			}

			logger.Info("[DEBUG] jq expression result:",
				"result_type", fmt.Sprintf("%T", result),
				"result", fmt.Sprintf("%+v", result),
				"expected_type", fmt.Sprintf("%T", expected),
				"expected", fmt.Sprintf("%+v", expected))

			// Compare result with expected value, handling numeric type conversions
			equal := false
			switch v := result.(type) {
			case int:
				if exp, ok := expected.(float64); ok {
					equal = float64(v) == exp
				}
			case float64:
				if exp, ok := expected.(float64); ok {
					equal = v == exp
				}
			default:
				equal = result == expected
			}

			if !equal {
				logger.Error("[DEBUG] jq assertion failed",
					"path", path,
					"expected_type", fmt.Sprintf("%T", expected),
					"expected", fmt.Sprintf("%+v", expected),
					"result_type", fmt.Sprintf("%T", result),
					"result", fmt.Sprintf("%+v", result))
				return nil, fmt.Errorf("jq assertion failed for expression %q: expected %v (type %T), got %v (type %T)",
					path, expected, expected, result, result)
			}
			logger.Info("[DEBUG] jq assertion passed")

		default:
			logger.Error("[DEBUG] Unknown assertion type", "type", assertionType)
			return nil, fmt.Errorf("unknown assertion type: %s", assertionType)
		}
	}

	logger.Info("[DEBUG] All assertions passed")
	return response, nil
}
