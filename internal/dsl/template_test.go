package dsl

import (
	"os"
	"testing"
)

func TestEscapedHandlebars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		runtime  map[string]interface{}
		expected string
	}{
		{
			name:     "escaped handlebars with already processed config var",
			input:    `API endpoint: https://api.example.com, literal handlebars: \{{ not_a_variable }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `API endpoint: https://api.example.com, literal handlebars: {{ not_a_variable }}`,
		},
		{
			name:     "escaped handlebars with runtime var",
			input:    `User ID: {{ user_id }}, literal template: \{{ .vars.secret }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user_id": "12345"},
			expected: `User ID: 12345, literal template: {{ .vars.secret }}`,
		},
		{
			name:     "multiple escaped handlebars",
			input:    `\{{ first }} and \{{ second }} are literals, but {{ runtime_var }} is processed`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"runtime_var": "actual_value"},
			expected: `{{ first }} and {{ second }} are literals, but actual_value is processed`,
		},
		{
			name:     "already processed config with escaped",
			input:    `Config: production, escaped: \{{ .vars.secret }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Config: production, escaped: {{ .vars.secret }}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := TemplateContext{

				Runtime: tt.runtime,
			}

			result, err := ProcessTemplate(tt.input, context)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessTemplate result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestEscapedHandlebarsConfigOnly(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "config only with escaped",
			input:    `Config: {{ .vars.env }}, escaped: \{{ .vars.secret }}, runtime: {{ user_id }}`,
			vars:     map[string]interface{}{"env": "production"},
			expected: `Config: production, escaped: {_{ .vars.secret }_}, runtime: {{ user_id }}`,
		},
		{
			name:     "only escaped handlebars",
			input:    `All escaped: \{{ .vars.secret }} and \{{ user_id }}`,
			vars:     map[string]interface{}{},
			expected: `All escaped: {_{ .vars.secret }_} and {_{ user_id }_}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessConfigVariablesOnly(tt.input, tt.vars)
			if err != nil {
				t.Fatalf("ProcessConfigVariablesOnly failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessConfigVariablesOnly result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestMultiLevelEscapedHandlebars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		runtime  map[string]interface{}
		expected string
	}{
		{
			name:     "single escape (1 backslash)",
			input:    `Escaped: \{{ var }} and processed: processed_value`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Escaped: {{ var }} and processed: processed_value`,
		},
		{
			name:     "double escape (2 backslashes)",
			input:    `Literal backslash: \\{{ var }} and processed: processed_value`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"var": "runtime_value"},
			expected: `Literal backslash: \runtime_value and processed: processed_value`,
		},
		{
			name:     "triple escape (3 backslashes)",
			input:    `Triple: \\\{{ var }} and processed: processed_value`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Triple: \{{ var }} and processed: processed_value`,
		},
		{
			name:     "quadruple escape (4 backslashes)",
			input:    `Quad: \\\\{{ var }} and processed: processed_value`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"var": "runtime_value"},
			expected: `Quad: \\runtime_value and processed: processed_value`,
		},
		{
			name:     "five escapes (5 backslashes)",
			input:    `Five: \\\\\{{ var }} and processed: processed_value`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Five: \\{{ var }} and processed: processed_value`,
		},
		{
			name:     "mixed levels with runtime",
			input:    `One: \{{ var }}, Two: \\{{ var }}, Three: \\\{{ var }}, Runtime: {{ user_id }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user_id": "12345", "var": "test_value"},
			expected: `One: {{ var }}, Two: \test_value, Three: \{{ var }}, Runtime: 12345`,
		},
		{
			name:     "complex mixed scenario",
			input:    `API: https://api.com, Escaped: \{{ template }}, Double: \\{{ literal }}, User: {{ user }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user": "alice", "literal": "test_literal"},
			expected: `API: https://api.com, Escaped: {{ template }}, Double: \test_literal, User: alice`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := TemplateContext{

				Runtime: tt.runtime,
			}

			result, err := ProcessTemplate(tt.input, context)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessTemplate result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestMultiLevelEscapedHandlebarsConfigOnly(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "config only with multiple escape levels",
			input:    `Config: {{ .vars.env }}, Single: \{{ .vars.secret }}, Double: \\{{ .vars.literal }}, Triple: \\\{{ .vars.triple }}, Runtime: {{ user_id }}`,
			vars:     map[string]interface{}{"env": "production"},
			expected: `Config: production, Single: {_{ .vars.secret }_}, Double: \{{ .vars.literal }}, Triple: \{_{ .vars.triple }_}, Runtime: {{ user_id }}`,
		},
		{
			name:     "various escape levels",
			input:    `One: \{{ .vars.a }}, Two: \\{{ .vars.b }}, Three: \\\{{ .vars.c }}, Four: \\\\{{ .vars.d }}`,
			vars:     map[string]interface{}{"b": "value_b", "d": "value_d"},
			expected: `One: {_{ .vars.a }_}, Two: \value_b, Three: \{_{ .vars.c }_}, Four: \\value_d`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessConfigVariablesOnly(tt.input, tt.vars)
			if err != nil {
				t.Fatalf("ProcessConfigVariablesOnly failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessConfigVariablesOnly result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Set up test environment variables
	originalValues := map[string]string{}
	testEnvVars := map[string]string{
		"TEST_API_KEY":     "test_key_12345",
		"TEST_BASE_URL":    "https://test.example.com",
		"TEST_DB_HOST":     "localhost:5432",
		"TEST_DB_USER":     "testuser",
		"TEST_TIMEOUT":     "30",
		"TEST_EMPTY_VAR":   "",
		"TEST_COMPLEX_VAL": "value-with-dashes_and_underscores",
	}

	// Store original values and set test values
	for key, value := range testEnvVars {
		if originalValue, exists := os.LookupEnv(key); exists {
			originalValues[key] = originalValue
		}
		_ = os.Setenv(key, value)
	}

	// Clean up after test
	defer func() {
		for key := range testEnvVars {
			if originalValue, exists := originalValues[key]; exists {
				_ = os.Setenv(key, originalValue)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		runtime  map[string]interface{}
		expected string
	}{
		{
			name:     "basic environment variable",
			input:    `API Key: {{ .env.TEST_API_KEY }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `API Key: test_key_12345`,
		},
		{
			name:     "multiple environment variables",
			input:    `URL: {{ .env.TEST_BASE_URL }}, DB: {{ .env.TEST_DB_HOST }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `URL: https://test.example.com, DB: localhost:5432`,
		},
		{
			name:     "mixed env, already processed config, and runtime variables",
			input:    `API: {{ .env.TEST_API_KEY }}, Config: auth_service, User: {{ user_id }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user_id": "12345"},
			expected: `API: test_key_12345, Config: auth_service, User: 12345`,
		},
		{
			name:     "environment variable in connection string",
			input:    `postgres://{{ .env.TEST_DB_USER }}:password@{{ .env.TEST_DB_HOST }}/testdb`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `postgres://testuser:password@localhost:5432/testdb`,
		},
		{
			name:     "environment variable with escaping",
			input:    `Real env: {{ .env.TEST_API_KEY }}, Escaped: \{{ .env.FAKE_VAR }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Real env: test_key_12345, Escaped: {{ .env.FAKE_VAR }}`,
		},
		{
			name:     "numeric environment variable",
			input:    `Timeout: {{ .env.TEST_TIMEOUT }} seconds`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Timeout: 30 seconds`,
		},
		{
			name:     "empty environment variable",
			input:    `Empty: "{{ .env.TEST_EMPTY_VAR }}", Non-empty: "{{ .env.TEST_API_KEY }}"`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Empty: "", Non-empty: "test_key_12345"`,
		},
		{
			name:     "complex environment variable value",
			input:    `Complex: {{ .env.TEST_COMPLEX_VAL }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Complex: value-with-dashes_and_underscores`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := TemplateContext{

				Runtime: tt.runtime,
			}

			result, err := ProcessTemplate(tt.input, context)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessTemplate result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestEnvironmentVariablesConfigOnly(t *testing.T) {
	// Set up test environment variables
	originalValues := map[string]string{}
	testEnvVars := map[string]string{
		"TEST_CONFIG_API_KEY": "config_test_key",
		"TEST_CONFIG_HOST":    "config.example.com",
	}

	// Store original values and set test values
	for key, value := range testEnvVars {
		if originalValue, exists := os.LookupEnv(key); exists {
			originalValues[key] = originalValue
		}
		_ = os.Setenv(key, value)
	}

	// Clean up after test
	defer func() {
		for key := range testEnvVars {
			if originalValue, exists := originalValues[key]; exists {
				_ = os.Setenv(key, originalValue)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "config processing with env vars",
			input:    `Config: {{ .vars.service }}, Env: {{ .env.TEST_CONFIG_API_KEY }}, Runtime: {{ user_id }}`,
			vars:     map[string]interface{}{"service": "auth"},
			expected: `Config: auth, Env: config_test_key, Runtime: {{ user_id }}`,
		},
		{
			name:     "env vars only in config processing",
			input:    `Host: {{ .env.TEST_CONFIG_HOST }}, Runtime: {{ runtime_var }}`,
			vars:     map[string]interface{}{},
			expected: `Host: config.example.com, Runtime: {{ runtime_var }}`,
		},
		{
			name:     "escaped env vars in config processing",
			input:    `Real: {{ .env.TEST_CONFIG_API_KEY }}, Escaped: \{{ .env.FAKE_ENV }}`,
			vars:     map[string]interface{}{},
			expected: `Real: config_test_key, Escaped: {_{ .env.FAKE_ENV }_}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessConfigVariablesOnly(tt.input, tt.vars)
			if err != nil {
				t.Fatalf("ProcessConfigVariablesOnly failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessConfigVariablesOnly result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}

func TestEnvironmentVariablesWithEscaping(t *testing.T) {
	// Set up test environment variables
	originalValues := map[string]string{}
	testEnvVars := map[string]string{
		"TEST_ESCAPE_VAR": "escape_test_value",
	}

	// Store original values and set test values
	for key, value := range testEnvVars {
		if originalValue, exists := os.LookupEnv(key); exists {
			originalValues[key] = originalValue
		}
		_ = os.Setenv(key, value)
	}

	// Clean up after test
	defer func() {
		for key := range testEnvVars {
			if originalValue, exists := originalValues[key]; exists {
				_ = os.Setenv(key, originalValue)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name     string
		input    string
		vars     map[string]interface{}
		runtime  map[string]interface{}
		expected string
	}{
		{
			name:     "single escape with env var",
			input:    `Escaped: \{{ .env.FAKE_VAR }}, Real: {{ .env.TEST_ESCAPE_VAR }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Escaped: {{ .env.FAKE_VAR }}, Real: escape_test_value`,
		},
		{
			name:     "double escape with env var",
			input:    `Double: \\{{ .env.TEST_ESCAPE_VAR }}, Real: config_value`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{},
			expected: `Double: \escape_test_value, Real: config_value`,
		},
		{
			name:     "triple escape with env var",
			input:    `Triple: \\\{{ .env.TEST_ESCAPE_VAR }}, Real: {{ runtime_var }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"runtime_var": "runtime_value"},
			expected: `Triple: \{{ .env.TEST_ESCAPE_VAR }}, Real: runtime_value`,
		},
		{
			name:     "mixed variable types with escaping",
			input:    `Env: {{ .env.TEST_ESCAPE_VAR }}, Config: config_test, Runtime: {{ user }}, Escaped: \{{ .env.LITERAL }}`,
			vars:     map[string]interface{}{},
			runtime:  map[string]interface{}{"user": "alice"},
			expected: `Env: escape_test_value, Config: config_test, Runtime: alice, Escaped: {{ .env.LITERAL }}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := TemplateContext{

				Runtime: tt.runtime,
			}

			result, err := ProcessTemplate(tt.input, context)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ProcessTemplate result mismatch:\nexpected: %q\ngot:      %q", tt.expected, result)
			}
		})
	}
}
