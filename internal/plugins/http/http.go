package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/PaesslerAG/jsonpath"
	"go.temporal.io/sdk/activity"
)

func (hp *HTTPPlugin) GetType() string {
	return "http"
}

func (hp *HTTPPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("[DEBUG] Starting HTTP request: " + hp.Config.Method + " " + hp.Config.URL)

	// Create HTTP request
	var body io.Reader
	if hp.Config.Body != "" {
		body = bytes.NewReader([]byte(hp.Config.Body))
		logger.Info("[DEBUG] Request body: " + hp.Config.Body)
	}

	req, err := http.NewRequestWithContext(ctx, hp.Config.Method, hp.Config.URL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range hp.Config.Headers {
		req.Header.Add(key, value)
		logger.Info(fmt.Sprintf("[DEBUG] Request header: %s: %s", key, value))
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

	logger.Info(fmt.Sprintf("[DEBUG] Response status code: %d", resp.StatusCode))
	logger.Info("[DEBUG] Response body: " + string(respBody))

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
	logger.Info(fmt.Sprintf("[DEBUG] Processing %d assertions", len(hp.Assertions)))
	for i, assertion := range hp.Assertions {
		logger.Info(fmt.Sprintf("[DEBUG] Processing assertion %d: type=%s", i+1, assertion.Type))

		switch assertion.Type {
		case AssertionTypeStatusCode:
			expectedStatus, ok := assertion.Expected.(float64)
			if !ok {
				logger.Error("[DEBUG] Status code assertion failed: expected value is not a number", "expected", assertion.Expected)
				return nil, fmt.Errorf("status code assertion expected value must be a number")
			}
			logger.Info(fmt.Sprintf("[DEBUG] Comparing status code: expected=%d, got=%d", int(expectedStatus), resp.StatusCode))
			if int(expectedStatus) != resp.StatusCode {
				logger.Error("[DEBUG] Status code assertion failed", "expected", int(expectedStatus), "got", resp.StatusCode)
				return nil, fmt.Errorf("status code assertion failed: expected %d, got %d", int(expectedStatus), resp.StatusCode)
			}
			logger.Info("[DEBUG] Status code assertion passed")

		case AssertionTypeJSONPath:
			// Parse response body as JSON
			var jsonData interface{}
			if err := json.Unmarshal(respBody, &jsonData); err != nil {
				logger.Error("[DEBUG] Failed to parse response body as JSON", "error", err)
				return nil, fmt.Errorf("failed to parse response body as JSON: %w", err)
			}

			logger.Info("[DEBUG] Evaluating JSONPath: " + assertion.Path)
			// Evaluate JSONPath
			result, err := jsonpath.Get(assertion.Path, jsonData)
			if err != nil {
				logger.Error("[DEBUG] Failed to evaluate JSONPath", "path", assertion.Path, "error", err)
				return nil, fmt.Errorf("failed to evaluate JSONPath %q: %w", assertion.Path, err)
			}

			logger.Info(fmt.Sprintf("[DEBUG] JSONPath result: %v, expected: %v", result, assertion.Expected))
			// Compare result with expected value
			if result != assertion.Expected {
				logger.Error("[DEBUG] JSONPath assertion failed", "path", assertion.Path, "expected", assertion.Expected, "got", result)
				return nil, fmt.Errorf("JSONPath assertion failed for path %q: expected %v, got %v", assertion.Path, assertion.Expected, result)
			}
			logger.Info("[DEBUG] JSONPath assertion passed")

		default:
			logger.Error("[DEBUG] Unknown assertion type", "type", assertion.Type)
			return nil, fmt.Errorf("unknown assertion type: %s", assertion.Type)
		}
	}

	logger.Info("[DEBUG] All assertions passed")
	return response, nil
}
