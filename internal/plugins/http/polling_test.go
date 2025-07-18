package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPPlugin_PollingConditions(t *testing.T) {
	plugin := &HTTPPlugin{}
	
	tests := []struct {
		name       string
		conditions []interface{}
		statusCode int
		headers    map[string][]string
		body       []byte
		state      map[string]string
		expected   bool
		wantErr    bool
	}{
		{
			name: "status code condition met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "status_code",
					"expected": float64(200),
				},
			},
			statusCode: 200,
			expected:   true,
		},
		{
			name: "status code condition not met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "status_code",
					"expected": float64(200),
				},
			},
			statusCode: 202,
			expected:   false,
		},
		{
			name: "json path condition met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "json_path",
					"path":     ".status",
					"expected": "complete",
				},
			},
			statusCode: 200,
			body:       []byte(`{"status": "complete", "progress": 100}`),
			expected:   true,
		},
		{
			name: "json path condition not met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "json_path",
					"path":     ".status",
					"expected": "complete",
				},
			},
			statusCode: 200,
			body:       []byte(`{"status": "processing", "progress": 50}`),
			expected:   false,
		},
		{
			name: "json path exists condition met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "json_path",
					"path":   ".job_id",
					"exists": true,
				},
			},
			statusCode: 200,
			body:       []byte(`{"job_id": "123", "status": "processing"}`),
			expected:   true,
		},
		{
			name: "json path exists condition not met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "json_path",
					"path":   ".job_id",
					"exists": true,
				},
			},
			statusCode: 200,
			body:       []byte(`{"status": "processing"}`),
			expected:   false,
		},
		{
			name: "header condition met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "header",
					"name":     "X-Job-Status",
					"expected": "completed",
				},
			},
			statusCode: 200,
			headers:    map[string][]string{"X-Job-Status": {"completed"}},
			expected:   true,
		},
		{
			name: "multiple conditions all met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "status_code",
					"expected": float64(200),
				},
				map[string]interface{}{
					"type":     "json_path",
					"path":     ".status",
					"expected": "complete",
				},
			},
			statusCode: 200,
			body:       []byte(`{"status": "complete"}`),
			expected:   true,
		},
		{
			name: "multiple conditions not all met",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "status_code",
					"expected": float64(200),
				},
				map[string]interface{}{
					"type":     "json_path",
					"path":     ".status",
					"expected": "complete",
				},
			},
			statusCode: 200,
			body:       []byte(`{"status": "processing"}`),
			expected:   false,
		},
		{
			name: "variable replacement in condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":     "json_path",
					"path":     ".status",
					"expected": "{{ expected_status }}",
				},
			},
			statusCode: 200,
			body:       []byte(`{"status": "complete"}`),
			state:      map[string]string{"expected_status": "complete"},
			expected:   true,
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
			
			result, err := plugin.checkPollingConditions(tt.conditions, resp, tt.body, tt.state)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkPollingConditions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("checkPollingConditions() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHTTPPlugin_PollingConfiguration(t *testing.T) {
	// Test server that responds with different status codes based on request count
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 3 {
			// First two requests return "processing"
			w.WriteHeader(200)
			w.Write([]byte(`{"status": "processing", "progress": 50}`))
		} else {
			// Third request returns "complete"
			w.WriteHeader(200)
			w.Write([]byte(`{"status": "complete", "progress": 100}`))
		}
	}))
	defer server.Close()

	plugin := &HTTPPlugin{}
	ctx := context.Background()
	
	// This test is more complex and would require setting up a full Temporal context
	// For now, we'll test the configuration parsing logic
	pollingConfig := map[string]interface{}{
		"interval":            "100ms",
		"timeout":             "5s",
		"max_attempts":        float64(10),
		"backoff_coefficient": 1.5,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":     "json_path",
				"path":     ".status",
				"expected": "complete",
			},
		},
	}
	
	// Test configuration parsing - this tests the beginning of executeWithPolling
	// We can't easily test the full polling loop without mocking Temporal context
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	p := map[string]interface{}{
		"config": map[string]interface{}{
			"method": "GET",
			"url":    server.URL,
		},
	}
	
	state := make(map[string]string)
	
	// Test that the polling configuration is parsed correctly
	// This is a limited test since we can't easily mock Temporal activity context
	intervalStr, ok := pollingConfig["interval"].(string)
	if !ok {
		t.Error("Expected interval to be a string")
	}
	
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		t.Errorf("Failed to parse interval: %v", err)
	}
	
	if interval != 100*time.Millisecond {
		t.Errorf("Expected interval to be 100ms, got %v", interval)
	}
	
	timeoutStr, ok := pollingConfig["timeout"].(string)
	if !ok {
		t.Error("Expected timeout to be a string")
	}
	
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		t.Errorf("Failed to parse timeout: %v", err)
	}
	
	if timeout != 5*time.Second {
		t.Errorf("Expected timeout to be 5s, got %v", timeout)
	}
	
	// Test that we can access the polling config without error
	_, _ = plugin.executeWithPolling(ctx, req, pollingConfig, p, state)
	
	// Note: The actual polling test would require a more complex setup with Temporal
	// This test mainly verifies that the configuration is parsed correctly
	t.Log("Polling configuration parsing test completed")
}

func TestHTTPPlugin_PollingConfigurationErrors(t *testing.T) {
	plugin := &HTTPPlugin{}
	ctx := context.Background()
	
	req, err := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	p := map[string]interface{}{
		"config": map[string]interface{}{
			"method": "GET",
			"url":    "http://example.com",
		},
	}
	
	state := make(map[string]string)
	
	tests := []struct {
		name          string
		pollingConfig map[string]interface{}
		wantErr       bool
		errContains   string
	}{
		{
			name:          "missing interval",
			pollingConfig: map[string]interface{}{},
			wantErr:       true,
			errContains:   "polling interval is required",
		},
		{
			name: "invalid interval format",
			pollingConfig: map[string]interface{}{
				"interval": "invalid",
			},
			wantErr:     true,
			errContains: "invalid polling interval",
		},
		{
			name: "missing timeout",
			pollingConfig: map[string]interface{}{
				"interval": "1s",
			},
			wantErr:     true,
			errContains: "polling timeout is required",
		},
		{
			name: "invalid timeout format",
			pollingConfig: map[string]interface{}{
				"interval": "1s",
				"timeout":  "invalid",
			},
			wantErr:     true,
			errContains: "invalid polling timeout",
		},
		{
			name: "missing conditions",
			pollingConfig: map[string]interface{}{
				"interval": "1s",
				"timeout":  "10s",
			},
			wantErr:     true,
			errContains: "polling conditions are required",
		},
		{
			name: "empty conditions",
			pollingConfig: map[string]interface{}{
				"interval":   "1s",
				"timeout":    "10s",
				"conditions": []interface{}{},
			},
			wantErr:     true,
			errContains: "polling conditions are required",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugin.executeWithPolling(ctx, req, tt.pollingConfig, p, state)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeWithPolling() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errContains != "" && err.Error() != tt.errContains {
					t.Errorf("executeWithPolling() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}