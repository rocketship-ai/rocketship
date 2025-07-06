package dsl

import (
	"embed"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	yaml "gopkg.in/yaml.v3"
)

//go:embed schema.json
var schemaFS embed.FS

type RocketshipConfig struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Vars        map[string]interface{} `json:"vars" yaml:"vars,omitempty"`
	Tests       []Test                 `json:"tests" yaml:"tests"`
}

type Test struct {
	Name  string `json:"name" yaml:"name"`
	Steps []Step `json:"steps" yaml:"steps"`
}

type Step struct {
	Name       string                   `json:"name" yaml:"name"`
	Plugin     string                   `json:"plugin" yaml:"plugin"`
	Config     map[string]interface{}   `json:"config" yaml:"config"`
	Assertions []map[string]interface{} `json:"assertions" yaml:"assertions"`
	Save       []map[string]interface{} `json:"save" yaml:"save,omitempty"`
	Retry      *RetryPolicy             `json:"retry" yaml:"retry,omitempty"`
}

type RetryPolicy struct {
	InitialInterval    string   `json:"initial_interval" yaml:"initial_interval,omitempty"`
	MaximumInterval    string   `json:"maximum_interval" yaml:"maximum_interval,omitempty"`
	MaximumAttempts    int      `json:"maximum_attempts" yaml:"maximum_attempts,omitempty"`
	BackoffCoefficient float64  `json:"backoff_coefficient" yaml:"backoff_coefficient,omitempty"`
	NonRetryableErrors []string `json:"non_retryable_errors" yaml:"non_retryable_errors,omitempty"`
}

// validateWithSchema validates the YAML data against the embedded JSON schema
func validateWithSchema(yamlData []byte) error {
	// Load the embedded schema
	schemaData, err := schemaFS.ReadFile("schema.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded schema: %w", err)
	}

	// Parse YAML to interface{} for schema validation
	var yamlDoc interface{}
	if err := yaml.Unmarshal(yamlData, &yamlDoc); err != nil {
		return fmt.Errorf("failed to parse YAML for validation: %w", err)
	}

	// Create schema loader and document loader
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	documentLoader := gojsonschema.NewGoLoader(yamlDoc)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return fmt.Errorf("schema validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// ParseYAML provides comprehensive YAML validation and parsing using JSON schema
func ParseYAML(yamlPayload []byte) (RocketshipConfig, error) {
	// First, validate against JSON schema for comprehensive validation
	if err := validateWithSchema(yamlPayload); err != nil {
		return RocketshipConfig{}, err
	}

	// Parse the YAML into our config struct
	var config RocketshipConfig
	if err := yaml.Unmarshal(yamlPayload, &config); err != nil {
		return RocketshipConfig{}, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return config, nil
}
