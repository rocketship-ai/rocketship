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
}

// HTTPAssertion represents a test assertion
type HTTPAssertion struct {
	Type     string      `json:"type" yaml:"type"`                   // "status_code", "json_path", or "header"
	Path     string      `json:"path" yaml:"path,omitempty"`         // Used for json_path assertions
	Name     string      `json:"name" yaml:"name,omitempty"`         // Used for header assertions
	Expected interface{} `json:"expected" yaml:"expected"`           // Expected value to match against
	Exists   bool        `json:"exists" yaml:"exists"`               // Used for checking if a value exists
	Contains string      `json:"contains" yaml:"contains,omitempty"` // Used for response_body assertions
}

// SaveConfig represents a configuration for saving response data
type SaveConfig struct {
	JSONPath string `json:"json_path" yaml:"json_path,omitempty"` // JSONPath to extract from response
	Header   string `json:"header" yaml:"header,omitempty"`       // Header name to extract
	As       string `json:"as" yaml:"as"`                         // Variable name to save as
}

// Common assertion types
const (
	AssertionTypeStatusCode   = "status_code"
	AssertionTypeJSONPath     = "json_path"
	AssertionTypeHeader       = "header"
	AssertionTypeResponseBody = "response_body"
)

// HTTPResponse represents the response from an HTTP request
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}
