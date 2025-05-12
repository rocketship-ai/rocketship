package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/PaesslerAG/jsonpath"
)

func (hp *HTTPPlugin) GetType() string {
	return "http"
}

func (hp *HTTPPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	// Create HTTP request
	var body io.Reader
	if hp.Config.Body != "" {
		body = bytes.NewReader([]byte(hp.Config.Body))
	}

	req, err := http.NewRequestWithContext(ctx, hp.Config.Method, hp.Config.URL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range hp.Config.Headers {
		req.Header.Add(key, value)
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
	for _, assertion := range hp.Assertions {
		switch assertion.Type {
		case AssertionTypeStatusCode:
			expectedStatus, ok := assertion.Expected.(float64)
			if !ok {
				return nil, fmt.Errorf("status code assertion expected value must be a number")
			}
			if int(expectedStatus) != resp.StatusCode {
				return nil, fmt.Errorf("status code assertion failed: expected %d, got %d", int(expectedStatus), resp.StatusCode)
			}

		case AssertionTypeJSONPath:
			// Parse response body as JSON
			var jsonData interface{}
			if err := json.Unmarshal(respBody, &jsonData); err != nil {
				return nil, fmt.Errorf("failed to parse response body as JSON: %w", err)
			}

			// Evaluate JSONPath
			result, err := jsonpath.Get(assertion.Path, jsonData)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate JSONPath %q: %w", assertion.Path, err)
			}

			// Compare result with expected value
			if result != assertion.Expected {
				return nil, fmt.Errorf("JSONPath assertion failed for path %q: expected %v, got %v", assertion.Path, assertion.Expected, result)
			}
		default:
			return nil, fmt.Errorf("unknown assertion type: %s", assertion.Type)
		}
	}

	return response, nil
}
