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
	OpenAPI     *OpenAPISuiteConfig    `json:"openapi" yaml:"openapi,omitempty"`
	Init        []Step                 `json:"init" yaml:"init,omitempty"`
	Tests       []Test                 `json:"tests" yaml:"tests"`
	Cleanup     *CleanupSpec           `json:"cleanup" yaml:"cleanup,omitempty"`
}

// OpenAPISuiteConfig represents OpenAPI settings applied to all HTTP steps unless overridden per step
type OpenAPISuiteConfig struct {
	Spec             string `json:"spec" yaml:"spec"`
	Version          string `json:"version" yaml:"version,omitempty"`
	ValidateRequest  *bool  `json:"validate_request" yaml:"validate_request,omitempty"`
	ValidateResponse *bool  `json:"validate_response" yaml:"validate_response,omitempty"`
	CacheTTL         string `json:"cache_ttl" yaml:"cache_ttl,omitempty"`
}

type Test struct {
	Name    string       `json:"name" yaml:"name"`
	Init    []Step       `json:"init" yaml:"init,omitempty"`
	Steps   []Step       `json:"steps" yaml:"steps"`
	Cleanup *CleanupSpec `json:"cleanup" yaml:"cleanup,omitempty"`
}

type Step struct {
	Name       string                   `json:"name" yaml:"name"`
	Plugin     string                   `json:"plugin" yaml:"plugin"`
	Config     map[string]interface{}   `json:"config" yaml:"config"`
	Assertions []map[string]interface{} `json:"assertions" yaml:"assertions"`
	Save       []map[string]interface{} `json:"save" yaml:"save,omitempty"`
	Retry      *RetryPolicy             `json:"retry" yaml:"retry,omitempty"`
}

type CleanupSpec struct {
	Always    []Step `json:"always" yaml:"always,omitempty"`
	OnFailure []Step `json:"on_failure" yaml:"on_failure,omitempty"`
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

	// Process browser sessions (auto-inject start/stop steps)
	if err := processBrowserSessions(&config); err != nil {
		return RocketshipConfig{}, fmt.Errorf("failed to process browser sessions: %w", err)
	}

	return config, nil
}

// processBrowserSessions scans tests for browser-using plugins and auto-injects start/stop steps
func processBrowserSessions(config *RocketshipConfig) error {
	for i := range config.Tests {
		test := &config.Tests[i]

		// Scan test steps for browser-using plugins
		needsBrowser, headless, err := scanForBrowserUsage(test.Steps)
		if err != nil {
			return fmt.Errorf("test %q: %w", test.Name, err)
		}

		if !needsBrowser {
			continue
		}

		// Generate session ID for this test
		sessionID := fmt.Sprintf("test-{{ .run.id }}-%d", i)

		// Inject browser start at the beginning of steps
		startStep := Step{
			Name:   "__auto_browser_start__",
			Plugin: "playwright",
			Config: map[string]interface{}{
				"role":       "start",
				"session_id": sessionID,
				"headless":   headless,
			},
		}
		test.Steps = append([]Step{startStep}, test.Steps...)

		// Inject browser stop in cleanup.always
		stopStep := Step{
			Name:   "__auto_browser_stop__",
			Plugin: "playwright",
			Config: map[string]interface{}{
				"role":       "stop",
				"session_id": sessionID,
			},
		}

		if test.Cleanup == nil {
			test.Cleanup = &CleanupSpec{}
		}
		test.Cleanup.Always = append(test.Cleanup.Always, stopStep)

		// Inject session_id into all browser-using steps
		for j := range test.Steps {
			step := &test.Steps[j]
			if usesBrowser(*step) {
				// Skip the auto-injected start step
				if step.Name == "__auto_browser_start__" {
					continue
				}
				// Only inject if session_id not already set
				if _, hasSessionID := step.Config["session_id"]; !hasSessionID {
					step.Config["session_id"] = sessionID
				}
			}
		}
	}

	return nil
}

// scanForBrowserUsage scans steps to determine if browser is needed and headless setting
func scanForBrowserUsage(steps []Step) (needsBrowser bool, headless bool, err error) {
	headless = true // Default to headless

	for _, step := range steps {
		// Error if user explicitly uses playwright start/stop
		if step.Plugin == "playwright" {
			if role, ok := step.Config["role"].(string); ok {
				if role == "start" || role == "stop" {
					return false, false, fmt.Errorf("step %q: invalid role %q. Browser sessions are now auto-managed. Only 'script' role is supported", step.Name, role)
				}
			}
		}

		// Check for browser usage
		if usesBrowser(step) {
			needsBrowser = true

			// Check headless setting
			if h, ok := step.Config["headless"]; ok {
				switch v := h.(type) {
				case bool:
					if !v {
						headless = false
					}
				case string:
					if strings.ToLower(v) == "false" || v == "{{ .env.HEADLESS }}" {
						// For template vars, we'll default to headless unless explicitly false
						if strings.ToLower(v) == "false" {
							headless = false
						}
					}
				}
			}
		}
	}

	return needsBrowser, headless, nil
}

// usesBrowser returns true if the step uses browser sessions
// This includes always-browser plugins (playwright, browser_use) and
// agent steps configured with browser capability
func usesBrowser(step Step) bool {
	// Always-browser plugins
	if step.Plugin == "playwright" || step.Plugin == "browser_use" {
		return true
	}

	// Agent with browser capability
	if step.Plugin == "agent" {
		if caps, ok := step.Config["capabilities"].([]interface{}); ok {
			for _, cap := range caps {
				if capStr, ok := cap.(string); ok && capStr == "browser" {
					return true
				}
			}
		}
	}

	return false
}
