package dsl

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	yaml "gopkg.in/yaml.v3"
)

//go:embed schema.json
var schemaFS embed.FS

type RocketshipConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Version     string `json:"version" yaml:"version"`
	Tests       []Test `json:"tests" yaml:"tests"`
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
// This addresses the TODO comment above about maintaining a single source of truth for YAML validation
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

	// Note: Basic validation is now handled by the JSON schema, but we keep
	// the existing validation for backwards compatibility and clear error messages
	if config.Version != "v1.0.0" {
		return RocketshipConfig{}, fmt.Errorf("unsupported version: %q", config.Version)
	}

	if len(config.Tests) == 0 {
		return RocketshipConfig{}, fmt.Errorf("no tests defined")
	}

	// test.Name and test.Steps are required for all tests (for clear error messages)
	for i, test := range config.Tests {
		if test.Name == "" {
			return RocketshipConfig{}, fmt.Errorf("test %d: a name is required for each test", i)
		}
		if len(test.Steps) == 0 {
			return RocketshipConfig{}, fmt.Errorf("test %q: no steps defined for this test", test.Name)
		}
	}

	// step.Name and step.Plugin are required for all steps (for clear error messages)
	for _, test := range config.Tests {
		for j, step := range test.Steps {
			if step.Name == "" {
				return RocketshipConfig{}, fmt.Errorf("test %q: step %d: a name is required for each step", test.Name, j)
			}
			if step.Plugin == "" {
				return RocketshipConfig{}, fmt.Errorf("test %q: step %q: a plugin is required for each step", test.Name, step.Name)
			}
		}
	}

	return config, nil
}
