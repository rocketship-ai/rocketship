package http

// HTTPPlugin represents a single HTTP test step
type HTTPPlugin struct {
	Name       string          `json:"name" yaml:"name"`
	Plugin     string          `json:"plugin" yaml:"plugin"`
	Config     HTTPConfig      `json:"config" yaml:"config"`
	Assertions []HTTPAssertion `json:"assertions" yaml:"assertions"`
	Save       []SaveConfig    `json:"save" yaml:"save,omitempty"`
}

// HTTPConfig contains the HTTP request configuration
type HTTPConfig struct {
	Method  string            `json:"method" yaml:"method"`
	URL     string            `json:"url" yaml:"url"`
	Body    string            `json:"body" yaml:"body,omitempty"`
	Headers map[string]string `json:"headers" yaml:"headers,omitempty"`
	Polling *PollingConfig    `json:"polling" yaml:"polling,omitempty"`
}

// HTTPAssertion represents a test assertion
type HTTPAssertion struct {
	Type     string      `json:"type" yaml:"type"`           // "status_code", "json_path", or "header"
	Path     string      `json:"path" yaml:"path,omitempty"` // Used for json_path assertions
	Name     string      `json:"name" yaml:"name,omitempty"` // Used for header assertions
	Expected interface{} `json:"expected" yaml:"expected"`   // Expected value to match against
	Exists   bool        `json:"exists" yaml:"exists"`       // Used for checking if a value exists
}

// SaveConfig represents a configuration for saving response data
type SaveConfig struct {
	JSONPath string `json:"json_path" yaml:"json_path,omitempty"` // JSONPath to extract from response
	Header   string `json:"header" yaml:"header,omitempty"`       // Header name to extract
	As       string `json:"as" yaml:"as"`                         // Variable name to save as
	Required *bool  `json:"required" yaml:"required,omitempty"`   // Whether the value is required (defaults to true)
}

// Common assertion types
const (
	AssertionTypeStatusCode = "status_code"
	AssertionTypeJSONPath   = "json_path"
	AssertionTypeHeader     = "header"
)

// PollingConfig represents configuration for polling until conditions are met
type PollingConfig struct {
	Interval           string            `json:"interval" yaml:"interval"`                       // Time between polling attempts (e.g., "2s", "500ms")
	Timeout            string            `json:"timeout" yaml:"timeout"`                         // Maximum time to wait (e.g., "5m", "30s")
	MaxAttempts        int               `json:"max_attempts" yaml:"max_attempts,omitempty"`     // Maximum number of polling attempts
	BackoffCoefficient float64           `json:"backoff_coefficient" yaml:"backoff_coefficient,omitempty"` // Exponential backoff multiplier (default: 1.0)
	Conditions         []PollingCondition `json:"conditions" yaml:"conditions"`                   // Conditions to check for polling completion
}

// PollingCondition represents a condition that must be met to stop polling
type PollingCondition struct {
	Type     string      `json:"type" yaml:"type"`           // "status_code", "json_path", or "header"
	Path     string      `json:"path" yaml:"path,omitempty"` // Used for json_path conditions
	Name     string      `json:"name" yaml:"name,omitempty"` // Used for header conditions
	Expected interface{} `json:"expected" yaml:"expected"`   // Expected value to match against
	Exists   bool        `json:"exists" yaml:"exists"`       // Used for checking if a value exists
}

// HTTPResponse represents the response from an HTTP request
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}
