package http

// HTTPPlugin represents a single HTTP test step
type HTTPPlugin struct {
	Name       string          `json:"name" yaml:"name"`
	Plugin     string          `json:"plugin" yaml:"plugin"`
	Config     HTTPConfig      `json:"config" yaml:"config"`
	Assertions []HTTPAssertion `json:"assertions" yaml:"assertions"`
}

// HTTPConfig contains the HTTP request configuration
type HTTPConfig struct {
	Method  string            `json:"method" yaml:"method"`
	URL     string            `json:"url" yaml:"url"`
	Body    string            `json:"body" yaml:"body,omitempty"`
	Headers map[string]string `json:"headers" yaml:"headers,omitempty"`
}

// HTTPAssertion represents a test assertion
type HTTPAssertion struct {
	Type     string      `json:"type" yaml:"type"`           // "status_code", "json_path", or "header"
	Path     string      `json:"path" yaml:"path,omitempty"` // Used for json_path assertions
	Name     string      `json:"name" yaml:"name,omitempty"` // Used for header assertions
	Expected interface{} `json:"expected" yaml:"expected"`   // Expected value to match against
}

// Common assertion types
const (
	AssertionTypeStatusCode = "status_code"
	AssertionTypeJSONPath   = "json_path"
	AssertionTypeHeader     = "header"
)

// HTTPResponse represents the response from an HTTP request
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}
