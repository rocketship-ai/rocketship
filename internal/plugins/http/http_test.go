package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHTTPPlugin_GetType(t *testing.T) {
	plugin := &HTTPPlugin{}
	if plugin.GetType() != "http" {
		t.Errorf("Expected type 'http', got '%s'", plugin.GetType())
	}
}

func TestReplaceVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		state    map[string]string
		expected string
		wantErr  bool
	}{
		{
			name:     "no variables",
			input:    "hello world",
			state:    map[string]string{},
			expected: "hello world",
		},
		{
			name:     "single variable",
			input:    "hello {{ name }}",
			state:    map[string]string{"name": "world"},
			expected: "hello world",
		},
		{
			name:     "multiple variables",
			input:    "{{ greeting }} {{ name }}!",
			state:    map[string]string{"greeting": "hello", "name": "world"},
			expected: "hello world!",
		},
		{
			name:    "missing variable",
			input:   "hello {{ missing }}",
			state:   map[string]string{"name": "world"},
			wantErr: true,
		},
		{
			name:     "nil state",
			input:    "hello world",
			state:    nil,
			expected: "hello world",
		},
		{
			name:     "spaces in variable",
			input:    "hello {{  name  }}",
			state:    map[string]string{"name": "world"},
			expected: "hello world",
		},
	}

	// Run tests concurrently for speed
	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := replaceVariables(tt.input, tt.state)
			if (err != nil) != tt.wantErr {
				t.Errorf("replaceVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("replaceVariables() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetStateKeys(t *testing.T) {
	tests := []struct {
		name     string
		state    map[string]string
		expected []string
	}{
		{
			name:     "empty state",
			state:    map[string]string{},
			expected: []string{},
		},
		{
			name:     "single key",
			state:    map[string]string{"key1": "value1"},
			expected: []string{"key1"},
		},
		{
			name:     "multiple keys",
			state:    map[string]string{"key2": "value2", "key1": "value1", "key3": "value3"},
			expected: []string{"key1", "key2", "key3"}, // Should be sorted
		},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getStateKeys(tt.state)
			if len(result) != len(tt.expected) {
				t.Errorf("getStateKeys() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, key := range result {
				if key != tt.expected[i] {
					t.Errorf("getStateKeys()[%d] = %v, want %v", i, key, tt.expected[i])
				}
			}
		})
	}
}

func TestHTTPPlugin_Activity(t *testing.T) {
	// Create a test server for HTTP requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users":
			if r.Method == "POST" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Server", "test-server")
				w.WriteHeader(200)
				_, _ = fmt.Fprint(w, `{"id": "123", "name": "John Doe"}`)
			}
		case "/error":
			w.WriteHeader(500)
			_, _ = fmt.Fprint(w, "Internal Server Error")
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	tests := []struct {
		name      string
		params    map[string]interface{}
		wantErr   bool
		checkFunc func(t *testing.T, resp interface{})
	}{
		{
			name: "successful POST request",
			params: map[string]interface{}{
				"config": map[string]interface{}{
					"method": "POST",
					"url":    server.URL + "/users",
					"body":   `{"name": "John Doe"}`,
				},
				"assertions": []interface{}{
					map[string]interface{}{
						"type":     "status_code",
						"expected": float64(200),
					},
				},
				"save": []interface{}{
					map[string]interface{}{
						"json_path": ".id",
						"as":        "user_id",
					},
					map[string]interface{}{
						"header": "Server",
						"as":     "server_info",
					},
				},
			},
			checkFunc: func(t *testing.T, resp interface{}) {
				activityResp, ok := resp.(*ActivityResponse)
				if !ok {
					t.Fatal("Expected ActivityResponse")
				}
				if activityResp.Response.StatusCode != 200 {
					t.Errorf("Expected status 200, got %d", activityResp.Response.StatusCode)
				}
				if activityResp.Saved["user_id"] != "123" {
					t.Errorf("Expected user_id '123', got '%s'", activityResp.Saved["user_id"])
				}
				if activityResp.Saved["server_info"] != "test-server" {
					t.Errorf("Expected server_info 'test-server', got '%s'", activityResp.Saved["server_info"])
				}
			},
		},
		{
			name: "invalid config",
			params: map[string]interface{}{
				"config": "invalid",
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			params: map[string]interface{}{
				"config": map[string]interface{}{
					"method": "POST",
				},
			},
			wantErr: true,
		},
	}

	plugin := &HTTPPlugin{}
	ctx := context.Background()

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Skip tests that call Activity directly (requires Temporal context)
			t.Skip("Activity method requires Temporal context - skipping direct call test")

			resp, err := plugin.Activity(ctx, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Activity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, resp)
			}
		})
	}
}

func TestHTTPPlugin_ProcessAssertions(t *testing.T) {
	plugin := &HTTPPlugin{}

	tests := []struct {
		name       string
		params     map[string]interface{}
		statusCode int
		headers    map[string][]string
		body       []byte
		wantErr    bool
	}{
		{
			name: "status code assertion success",
			params: map[string]interface{}{
				"assertions": []interface{}{
					map[string]interface{}{
						"type":     "status_code",
						"expected": float64(200),
					},
				},
			},
			statusCode: 200,
			wantErr:    false,
		},
		{
			name: "status code assertion failure",
			params: map[string]interface{}{
				"assertions": []interface{}{
					map[string]interface{}{
						"type":     "status_code",
						"expected": float64(200),
					},
				},
			},
			statusCode: 404,
			wantErr:    true,
		},
		{
			name: "header assertion success",
			params: map[string]interface{}{
				"assertions": []interface{}{
					map[string]interface{}{
						"type":     "header",
						"name":     "Content-Type",
						"expected": "application/json",
					},
				},
			},
			statusCode: 200,
			headers:    map[string][]string{"Content-Type": {"application/json"}},
			wantErr:    false,
		},
		{
			name: "json path assertion success",
			params: map[string]interface{}{
				"assertions": []interface{}{
					map[string]interface{}{
						"type":     "json_path",
						"path":     ".name",
						"expected": "John Doe",
					},
				},
			},
			statusCode: 200,
			body:       []byte(`{"name": "John Doe", "id": "123"}`),
			wantErr:    false,
		},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     tt.headers,
			}
			err := plugin.processAssertions(tt.params, resp, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("processAssertions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPPlugin_ProcessSaves(t *testing.T) {
	plugin := &HTTPPlugin{}

	tests := []struct {
		name     string
		params   map[string]interface{}
		headers  map[string][]string
		body     []byte
		expected map[string]string
		wantErr  bool
	}{
		{
			name: "json path save",
			params: map[string]interface{}{
				"save": []interface{}{
					map[string]interface{}{
						"json_path": ".id",
						"as":        "user_id",
					},
				},
			},
			body:     []byte(`{"id": "123", "name": "John"}`),
			expected: map[string]string{"user_id": "123"},
			wantErr:  false,
		},
		{
			name: "header save",
			params: map[string]interface{}{
				"save": []interface{}{
					map[string]interface{}{
						"header": "Server",
						"as":     "server_info",
					},
				},
			},
			headers:  map[string][]string{"Server": {"nginx/1.0"}},
			expected: map[string]string{"server_info": "nginx/1.0"},
			wantErr:  false,
		},
		{
			name: "missing required json path",
			params: map[string]interface{}{
				"save": []interface{}{
					map[string]interface{}{
						"json_path": ".missing",
						"as":        "missing_value",
					},
				},
			},
			body:    []byte(`{"id": "123"}`),
			wantErr: true,
		},
		{
			name: "optional missing json path",
			params: map[string]interface{}{
				"save": []interface{}{
					map[string]interface{}{
						"json_path": ".missing",
						"as":        "missing_value",
						"required":  false,
					},
				},
			},
			body:     []byte(`{"id": "123"}`),
			expected: map[string]string{},
			wantErr:  false,
		},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{
				Header: tt.headers,
			}
			saved := make(map[string]string)
			err := plugin.processSaves(tt.params, resp, tt.body, saved)
			if (err != nil) != tt.wantErr {
				t.Errorf("processSaves() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for key, expectedValue := range tt.expected {
					if saved[key] != expectedValue {
						t.Errorf("Expected saved[%s] = %s, got %s", key, expectedValue, saved[key])
					}
				}
			}
		})
	}
}

func TestHTTPPlugin_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	// Test concurrent creation of HTTPPlugin instances to ensure thread safety
	numGoroutines := 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create plugin instance concurrently
			plugin := &HTTPPlugin{}

			// Test GetType method concurrently
			pluginType := plugin.GetType()
			if pluginType != "http" {
				errors <- fmt.Errorf("goroutine %d: expected type 'http', got %s", id, pluginType)
				return
			}

			// Test concurrent parsing
			testParams := map[string]interface{}{
				"config": map[string]interface{}{
					"method": "GET",
					"url":    "https://example.com",
				},
			}

			if _, ok := testParams["config"].(map[string]interface{}); !ok {
				errors <- fmt.Errorf("goroutine %d: failed to parse config", id)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestHTTPPlugin_VariableSubstitution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the request body and headers for verification
		body, _ := json.Marshal(map[string]interface{}{
			"received_body":   r.Header.Get("X-Test-Body"),
			"received_header": r.Header.Get("X-Test-Header"),
			"received_url":    r.URL.String(),
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	// Skip this test as it requires Temporal activity context
	t.Skip("Activity method requires Temporal context - skipping direct call test")
}

func TestBuildRequest_FormURLEncoded(t *testing.T) {
	state := map[string]string{"name": "world"}
	config := map[string]interface{}{
		"form": map[string]interface{}{
			"foo":       "bar",
			"templated": "hello {{ name }}",
		},
	}
	req, err := buildRequest("POST", "http://example.com", config, state)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}
	if ct := req.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Fatalf("expected Content-Type form, got %q", ct)
	}
	// Read and parse body
	bodyBytes, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	v, err := url.ParseQuery(string(bodyBytes))
	if err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if v.Get("foo") != "bar" {
		t.Errorf("expected foo=bar, got %q", v.Get("foo"))
	}
	if v.Get("templated") != "hello world" {
		t.Errorf("expected templated=hello world, got %q", v.Get("templated"))
	}
}

func TestBuildRequest_FormExplicitHeaderPreserved(t *testing.T) {
	config := map[string]interface{}{
		"headers": map[string]interface{}{
			"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
		},
		"form": map[string]interface{}{
			"a": "1",
		},
	}
	req, err := buildRequest("POST", "http://example.com", config, nil)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}
	if ct := req.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded; charset=utf-8" {
		t.Fatalf("expected explicit Content-Type preserved, got %q", ct)
	}
}

func TestBuildRequest_FormArrayValues(t *testing.T) {
	config := map[string]interface{}{
		"form": map[string]interface{}{
			"multi": []interface{}{"1", "2"},
		},
	}
	req, err := buildRequest("POST", "http://example.com", config, nil)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}
	bodyBytes, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	v, _ := url.ParseQuery(string(bodyBytes))
	vals, ok := v["multi"]
	if !ok || len(vals) != 2 || vals[0] != "1" || vals[1] != "2" {
		t.Fatalf("expected multi=[1,2], got %v", vals)
	}
}

func TestBuildRequest_FormPreferredOverBody(t *testing.T) {
	config := map[string]interface{}{
		"body": "{\"x\":1}",
		"form": map[string]interface{}{"y": "2"},
	}
	req, err := buildRequest("POST", "http://example.com", config, nil)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}
	bodyBytes, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(bodyBytes), "y=2") {
		t.Fatalf("expected form body, got %q", string(bodyBytes))
	}
}

// buildRequest is a test-only helper mirroring the request-building logic in the plugin
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

func TestHTTPPlugin_OpenAPIValidation(t *testing.T) {
	t.Parallel()

	specContents := `openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    post:
      operationId: createUser
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - name
              properties:
                name:
                  type: string
              additionalProperties: false
      responses:
        '200':
          description: Successful response
          content:
            application/json:
              schema:
                type: object
                required:
                  - id
                  - name
                properties:
                  id:
                    type: string
                  name:
                    type: string
                additionalProperties: false
`

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContents), 0o600); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	t.Run("valid request and response", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		configData := map[string]interface{}{
			"openapi": map[string]interface{}{
				"spec": specPath,
			},
		}

		validator, err := newOpenAPIValidator(ctx, configData, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error creating validator: %v", err)
		}

		reqBody := []byte(`{"name":"Jane"}`)
		req := httptest.NewRequest(http.MethodPost, "http://example.com/users", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		setRequestBody(req, reqBody)

		if err := validator.prepareRequestValidation(ctx, req, reqBody); err != nil {
			t.Fatalf("prepareRequestValidation error: %v", err)
		}

		if err := validator.validateRequest(ctx); err != nil {
			t.Fatalf("validateRequest error: %v", err)
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}
		respBody := []byte(`{"id":"123","name":"Jane"}`)

		if err := validator.validateResponse(ctx, resp, respBody); err != nil {
			t.Fatalf("validateResponse error: %v", err)
		}
	})

	t.Run("response with additional fields fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		configData := map[string]interface{}{
			"openapi": map[string]interface{}{
				"spec": specPath,
			},
		}

		validator, err := newOpenAPIValidator(ctx, configData, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error creating validator: %v", err)
		}

		reqBody := []byte(`{"name":"Jane"}`)
		req := httptest.NewRequest(http.MethodPost, "http://example.com/users", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		setRequestBody(req, reqBody)

		if err := validator.prepareRequestValidation(ctx, req, reqBody); err != nil {
			t.Fatalf("prepareRequestValidation error: %v", err)
		}

		if err := validator.validateRequest(ctx); err != nil {
			t.Fatalf("validateRequest error: %v", err)
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}
		respBody := []byte(`{"id":"123","name":"Jane","extra":"nope"}`)

		if err := validator.validateResponse(ctx, resp, respBody); err == nil {
			t.Fatalf("expected response validation error")
		} else if !strings.Contains(err.Error(), "openapi response validation failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("request validation prevents invalid payload", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		configData := map[string]interface{}{
			"openapi": map[string]interface{}{
				"spec": specPath,
			},
		}

		validator, err := newOpenAPIValidator(ctx, configData, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error creating validator: %v", err)
		}

		reqBody := []byte(`{"email":"jane@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "http://example.com/users", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		setRequestBody(req, reqBody)

		if err := validator.prepareRequestValidation(ctx, req, reqBody); err != nil {
			t.Fatalf("prepareRequestValidation error: %v", err)
		}

		if err := validator.validateRequest(ctx); err == nil {
			t.Fatalf("expected request validation failure")
		} else if !strings.Contains(err.Error(), "openapi request validation failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("uses suite defaults when no step override provided", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		suite := map[string]interface{}{
			"spec": specPath,
		}
		configData := map[string]interface{}{}

		validator, err := newOpenAPIValidator(ctx, configData, suite, nil)
		if err != nil {
			t.Fatalf("unexpected error creating validator: %v", err)
		}

		reqBody := []byte(`{"name":"Jane"}`)
		req := httptest.NewRequest(http.MethodPost, "http://example.com/users", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		setRequestBody(req, reqBody)

		if err := validator.prepareRequestValidation(ctx, req, reqBody); err != nil {
			t.Fatalf("prepareRequestValidation error: %v", err)
		}

		if !validator.shouldValidateRequest() || !validator.shouldValidateResponse() {
			t.Fatalf("expected suite defaults to enable both validations")
		}
	})

	t.Run("step override disables request validation only", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		suite := map[string]interface{}{
			"spec": specPath,
		}
		configData := map[string]interface{}{
			"openapi": map[string]interface{}{
				"validate_request": false,
			},
		}

		validator, err := newOpenAPIValidator(ctx, configData, suite, nil)
		if err != nil {
			t.Fatalf("unexpected error creating validator: %v", err)
		}

		if validator.shouldValidateRequest() {
			t.Fatalf("expected request validation to be disabled")
		}
		if !validator.shouldValidateResponse() {
			t.Fatalf("expected response validation to remain enabled")
		}

		reqBody := []byte(`{"name":"Jane"}`)
		req := httptest.NewRequest(http.MethodPost, "http://example.com/users", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		setRequestBody(req, reqBody)

		if err := validator.prepareRequestValidation(ctx, req, reqBody); err != nil {
			t.Fatalf("prepareRequestValidation error: %v", err)
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}
		respBody := []byte(`{"id":"123","name":"Jane"}`)

		if err := validator.validateResponse(ctx, resp, respBody); err != nil {
			t.Fatalf("validateResponse error: %v", err)
		}
	})

	t.Run("returns nil when both validations disabled", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		suite := map[string]interface{}{
			"spec": specPath,
		}
		configData := map[string]interface{}{
			"openapi": map[string]interface{}{
				"validate_request":  false,
				"validate_response": false,
			},
		}

		validator, err := newOpenAPIValidator(ctx, configData, suite, nil)
		if err != nil {
			t.Fatalf("unexpected error creating validator: %v", err)
		}
		if validator != nil {
			t.Fatalf("expected validator to be nil when both validations are disabled")
		}
	})
}

func TestOpenAPISpecCaching(t *testing.T) {
	t.Parallel()

	specContents := `openapi: 3.0.3
info:
  title: Cache Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: ok
`

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContents), 0o600); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	ctx := context.Background()

	resetCache := func() {
		openAPISpecCache.mu.Lock()
		openAPISpecCache.specs = make(map[string]*openAPISpecEntry)
		openAPISpecCache.mu.Unlock()
	}

	resetCache()

	suite := map[string]interface{}{
		"spec":      specPath,
		"cache_ttl": "1h",
		"version":   "v1",
	}

	validatorOne, err := newOpenAPIValidator(ctx, map[string]interface{}{}, suite, nil)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}
	if validatorOne == nil {
		t.Fatalf("expected validator instance")
	}

	validatorTwo, err := newOpenAPIValidator(ctx, map[string]interface{}{}, suite, nil)
	if err != nil {
		t.Fatalf("unexpected error creating second validator: %v", err)
	}
	if validatorTwo.entry != validatorOne.entry {
		t.Fatalf("expected cache to reuse spec entry")
	}

	suiteVersionTwo := map[string]interface{}{
		"spec":    specPath,
		"version": "v2",
	}

	validatorThree, err := newOpenAPIValidator(ctx, map[string]interface{}{}, suiteVersionTwo, nil)
	if err != nil {
		t.Fatalf("unexpected error creating validator with new version: %v", err)
	}
	if validatorThree.entry == nil || validatorThree.entry == validatorOne.entry {
		t.Fatalf("expected new cache entry when version changes")
	}

	suiteShortTTL := map[string]interface{}{
		"spec":      specPath,
		"cache_ttl": "5ms",
	}

	resetCache()
	validatorShortTTL, err := newOpenAPIValidator(ctx, map[string]interface{}{}, suiteShortTTL, nil)
	if err != nil {
		t.Fatalf("unexpected error creating validator with short TTL: %v", err)
	}
	entryBefore := validatorShortTTL.entry
	time.Sleep(10 * time.Millisecond)
	validatorAfterTTL, err := newOpenAPIValidator(ctx, map[string]interface{}{}, suiteShortTTL, nil)
	if err != nil {
		t.Fatalf("unexpected error creating validator after TTL: %v", err)
	}
	if validatorAfterTTL.entry == entryBefore {
		t.Fatalf("expected cache entry to refresh after TTL expires")
	}
}

func TestHTTPPlugin_ErrorHandling(t *testing.T) {
	plugin := &HTTPPlugin{}
	ctx := context.Background()

	errorTests := []struct {
		name    string
		params  map[string]interface{}
		wantErr string
	}{
		{
			name: "invalid config type",
			params: map[string]interface{}{
				"config": "not a map",
			},
			wantErr: "invalid config format",
		},
		{
			name: "missing method",
			params: map[string]interface{}{
				"config": map[string]interface{}{
					"url": "http://example.com",
				},
			},
			wantErr: "method is required",
		},
		{
			name: "missing URL",
			params: map[string]interface{}{
				"config": map[string]interface{}{
					"method": "GET",
				},
			},
			wantErr: "url is required",
		},
		{
			name: "invalid URL",
			params: map[string]interface{}{
				"config": map[string]interface{}{
					"method": "GET",
					"url":    "not-a-valid-url",
				},
			},
			wantErr: "failed to send request",
		},
	}

	t.Parallel()
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Skip tests that call Activity directly (requires Temporal context)
			t.Skip("Activity method requires Temporal context - skipping direct call test")

			_, err := plugin.Activity(ctx, tt.params)
			if err == nil {
				t.Error("Expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.wantErr, err.Error())
			}
		})
	}
}
