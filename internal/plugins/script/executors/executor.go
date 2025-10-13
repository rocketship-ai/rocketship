package executors

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/plugins/script/runtime"
)

// Executor defines the interface for all script language executors
type Executor interface {
	// Execute runs the script in the given runtime context
	Execute(ctx context.Context, script string, runtime *runtime.Context) error

	// Language returns the language identifier for this executor
	Language() string

	// ValidateScript performs static validation of the script
	ValidateScript(script string) error
}

// NewExecutor creates a new executor for the specified language
func NewExecutor(language string) (Executor, error) {
	switch language {
	case "javascript":
		return NewJavaScriptExecutor(), nil
	case "shell":
		return NewShellExecutor(), nil
	case "python":
		return nil, fmt.Errorf("python executor not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

// GetSupportedLanguages returns a list of all supported languages
func GetSupportedLanguages() []string {
	return []string{"javascript", "shell"} // Will expand as we add more languages
}
