package dsl

import (
	"testing"
)

func TestParseYAML(t *testing.T) {
	validYAML := []byte(`
version: 1
tests:
  - name: Test Example
    steps:
      - op: http.send
        params:
          method: GET
          url: http://example.com
        expect:
          status: 200
      - op: sleep
        duration: 1s
`)

	test, err := ParseYAML(validYAML)
	if err != nil {
		t.Fatalf("Failed to parse valid YAML: %v", err)
	}

	if test.Name != "Test Example" {
		t.Errorf("Expected test name 'Test Example', got '%s'", test.Name)
	}

	if len(test.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(test.Steps))
	}

	if test.Steps[0].Op != "http.send" {
		t.Errorf("Expected first step op 'http.send', got '%s'", test.Steps[0].Op)
	}

	if test.Steps[1].Op != "sleep" {
		t.Errorf("Expected second step op 'sleep', got '%s'", test.Steps[1].Op)
	}

	if test.Steps[1].Duration != "1s" {
		t.Errorf("Expected duration '1s', got '%s'", test.Steps[1].Duration)
	}
}

func TestParseYAMLInvalid(t *testing.T) {
	invalidYAML := []byte(`
version: 2  # Invalid version
tests:
  - name: Test Example
    steps:
      - op: invalid.op
`)

	_, err := ParseYAML(invalidYAML)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestValidateYAML(t *testing.T) {
	validYAML := []byte(`
version: 1
tests:
  - name: Test Example
    steps:
      - op: http.send
        params:
          method: GET
          url: http://example.com
        expect:
          status: 200
`)

	if err := ValidateYAML(validYAML); err != nil {
		t.Errorf("Expected valid YAML to pass validation, got error: %v", err)
	}

	invalidYAML := []byte(`
version: 1
tests:
  - name: Test Example
    steps:
      - op: http.send
        # Missing required params
`)

	if err := ValidateYAML(invalidYAML); err == nil {
		t.Error("Expected invalid YAML to fail validation, got nil error")
	}
}
