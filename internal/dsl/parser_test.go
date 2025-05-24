package dsl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAML_ValidConfigs(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "simple delay test",
			yaml: `
name: "Simple Delay Test"
description: "A simple test with delay"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "delay"
        config:
          duration: "5s"
`,
		},
		{
			name: "simple HTTP test",
			yaml: `
name: "Simple HTTP Test"
description: "A simple HTTP test"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "GET request"
        plugin: "http"
        config:
          method: "GET"
          url: "https://example.com"
        assertions:
          - type: "status_code"
            expected: 200
`,
		},
		{
			name: "complex HTTP test with chaining",
			yaml: `
name: "Complex HTTP Test"
description: "Complex test with request chaining"
version: "v1.0.0"
tests:
  - name: "Product Test"
    steps:
      - name: "Create product"
        plugin: "http"
        config:
          method: "POST"
          url: "https://api.example.com/products"
          body: '{"name": "Widget", "price": 19.99}'
          headers:
            Content-Type: "application/json"
        assertions:
          - type: "status_code"
            expected: 201
          - type: "json_path"
            path: ".name"
            expected: "Widget"
        save:
          - json_path: ".id"
            as: "product_id"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseYAML([]byte(strings.TrimSpace(tt.yaml)))
			require.NoError(t, err)
			assert.NotEmpty(t, config.Name)
			assert.Equal(t, "v1.0.0", config.Version)
			assert.NotEmpty(t, config.Tests)
		})
	}
}

func TestParseYAML_InvalidConfigs(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectedErr string
	}{
		{
			name: "missing name",
			yaml: `
description: "Test without name"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "delay"
        config:
          duration: "5s"
`,
			expectedErr: "schema validation failed",
		},
		{
			name: "missing version",
			yaml: `
name: "Test Suite"
description: "Test without version"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "delay"
        config:
          duration: "5s"
`,
			expectedErr: "schema validation failed",
		},
		{
			name: "invalid version format",
			yaml: `
name: "Test Suite"
version: "1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "delay"
        config:
          duration: "5s"
`,
			expectedErr: "schema validation failed",
		},
		{
			name: "no tests",
			yaml: `
name: "Test Suite"
version: "v1.0.0"
tests: []
`,
			expectedErr: "schema validation failed",
		},
		{
			name: "invalid plugin",
			yaml: `
name: "Test Suite"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "invalid_plugin"
        config:
          duration: "5s"
`,
			expectedErr: "schema validation failed",
		},
		{
			name: "missing assertion path for json_path type",
			yaml: `
name: "Test Suite"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "http"
        config:
          method: "GET"
          url: "https://example.com"
        assertions:
          - type: "json_path"
            expected: "value"
`,
			expectedErr: "schema validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseYAML([]byte(strings.TrimSpace(tt.yaml)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestParseYAML_BackwardsCompatibility(t *testing.T) {
	// Test that schema validation catches invalid versions
	yaml := `
name: "Test Suite"
version: "2.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "delay"
        config:
          duration: "5s"
`
	_, err := ParseYAML([]byte(strings.TrimSpace(yaml)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
}

func TestValidateWithSchema_DirectTesting(t *testing.T) {
	// Test the schema validation function directly
	validYAML := []byte(`
name: "Test Suite"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Step 1"
        plugin: "delay"
        config:
          duration: "5s"
`)

	err := validateWithSchema(validYAML)
	assert.NoError(t, err)

	invalidYAML := []byte(`
name: "Test Suite"
tests: []
`)

	err = validateWithSchema(invalidYAML)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
}