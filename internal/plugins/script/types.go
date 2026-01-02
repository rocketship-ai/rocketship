package script

// ScriptConfig represents the configuration for a script step
type ScriptConfig struct {
	Language string `json:"language" yaml:"language"` // Required: javascript, python, shell, etc.
	Script   string `json:"script" yaml:"script"`     // Inline script content
	File     string `json:"file" yaml:"file"`         // Path to external script file
	Timeout  string `json:"timeout" yaml:"timeout"`   // Execution timeout (default: 30s)
}

// ActivityRequest represents the input to the script activity
type ActivityRequest struct {
	Name   string                 `json:"name"`
	Plugin string                 `json:"plugin"`
	Config map[string]interface{} `json:"config"`
	State  map[string]string      `json:"state"`
	Vars   map[string]interface{} `json:"vars"`
	Env    map[string]string      `json:"env"` // Environment secrets from project environment
}

// ActivityResponse represents the output from the script activity
type ActivityResponse struct {
	Saved map[string]string `json:"saved"` // Values saved by the script
}
