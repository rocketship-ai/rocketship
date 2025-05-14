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

	configData, ok := pluginData["config"].(map[string]interface{})
	if !ok {
		rawConfig := pluginData["config"]
		return nil, fmt.Errorf("invalid config format: got type %T", rawConfig)
	}

	method, ok := configData["method"].(string)
	if !ok {
		rawMethod := configData["method"]
		return nil, fmt.Errorf("method is required: got type %T", rawMethod)
	}

	url, ok := configData["url"].(string)
	if !ok {
		rawURL := configData["url"]
		return nil, fmt.Errorf("url is required: got type %T", rawURL)
	}

	// Create HTTP request
	var body io.Reader
	if bodyStr, ok := configData["body"].(string); ok && bodyStr != "" {
		body = bytes.NewReader([]byte(bodyStr))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if headers, ok := configData["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Add(key, strValue)
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

	// Process assertions
	assertions, ok := pluginData["assertions"].([]interface{})
	if !ok {
		return response, nil
	}

	for _, assertion := range assertions {
		assertionMap, ok := assertion.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid assertion format: got type %T", assertion)
		}

		assertionType, ok := assertionMap["type"].(string)
		if !ok {
			rawType := assertionMap["type"]
			return nil, fmt.Errorf("assertion type is required: got type %T", rawType)
		}

		switch assertionType {
		case AssertionTypeStatusCode:
			expected, ok := assertionMap["expected"].(float64)
			if !ok {
				rawExpected := assertionMap["expected"]
				return nil, fmt.Errorf("status code assertion expected value must be a number: got type %T", rawExpected)
			}
			if int(expected) != resp.StatusCode {
				return nil, fmt.Errorf("status code assertion failed: expected %d, got %d", int(expected), resp.StatusCode)
			}

		case AssertionTypeHeader:
			headerName, ok := assertionMap["name"].(string)
			if !ok {
				rawName := assertionMap["name"]
				return nil, fmt.Errorf("header name is required for header assertion: got type %T", rawName)
			}

			expected, ok := assertionMap["expected"].(string)
			if !ok {
				rawExpected := assertionMap["expected"]
				return nil, fmt.Errorf("header value must be a string: got type %T", rawExpected)
			}

			actual := resp.Header.Get(headerName)
			if actual != expected {
				return nil, fmt.Errorf("header assertion failed for %q: expected %q, got %q", headerName, expected, actual)
			}

		case AssertionTypeJSONPath:
			// Parse response body as JSON
			var jsonData interface{}
			if err := json.Unmarshal(respBody, &jsonData); err != nil {
				return nil, fmt.Errorf("failed to parse response body as JSON: %w", err)
			}

			path, ok := assertionMap["path"].(string)
			if !ok {
				rawPath := assertionMap["path"]
				return nil, fmt.Errorf("path is required for JSONPath assertion: got type %T", rawPath)
			}

			// Parse and run jq query
			query, err := gojq.Parse(path)
			if err != nil {
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
					return nil, fmt.Errorf("error evaluating jq expression %q: %w", path, err)
				}
				// Take the first result
				if !found {
					result = v
					found = true
				}
			}

			if !found {
				return nil, fmt.Errorf("no results from jq expression %q", path)
			}

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
				return nil, fmt.Errorf("jq assertion failed: path %q expected %v (type %T), got %v (type %T)",
					path, expected, expected, result, result)
			}
		default:
			return nil, fmt.Errorf("unknown assertion type: %s", assertionType)
		}
	}

	return response, nil
}
