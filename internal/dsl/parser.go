package dsl

import (
	"fmt"
	yaml "gopkg.in/yaml.v3"
)

type Test struct {
	Name  string `json:"name" yaml:"name"`
	Steps []Step `json:"steps" yaml:"steps"`
}

type Step struct {
	Op       string                 `json:"op" yaml:"op"`
	Params   map[string]interface{} `json:"params" yaml:"params"`
	Expect   map[string]interface{} `json:"expect" yaml:"expect"`
	Save     *SaveConfig            `json:"save" yaml:"save"`
	Duration string                 `json:"duration" yaml:"duration"`
}

type SaveConfig struct {
	JSONPath string `json:"jsonPath" yaml:"jsonPath"`
	As       string `json:"as" yaml:"as"`
}

type RocketshipConfig struct {
	Version int    `json:"version" yaml:"version"`
	Tests   []Test `json:"tests" yaml:"tests"`
}

func ParseYAML(yamlPayload []byte) (Test, error) {
	var config RocketshipConfig
	if err := yaml.Unmarshal(yamlPayload, &config); err != nil {
		return Test{}, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	if config.Version != 1 {
		return Test{}, fmt.Errorf("unsupported version: %d", config.Version)
	}

	if len(config.Tests) == 0 {
		return Test{}, fmt.Errorf("no tests defined")
	}

	return config.Tests[0], nil
}

func ValidateYAML(yamlPayload []byte) error {
	return ValidateYAMLWithSchema(yamlPayload)
}
