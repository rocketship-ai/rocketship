package dsl

import (
	"fmt"

	yaml "gopkg.in/yaml.v3"
)

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
}

// TODO: Probably good to maintain a SSOT for YAML validation. For CLI client and engine server.
func ParseYAML(yamlPayload []byte) (RocketshipConfig, error) {
	var config RocketshipConfig
	if err := yaml.Unmarshal(yamlPayload, &config); err != nil {
		return RocketshipConfig{}, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

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

// TODO: Do i even need this? Its not used anywhere. The function is defined in dsl/schema.go
func ValidateYAML(yamlPayload []byte) error {
	return ValidateYAMLWithSchema(yamlPayload)
}
