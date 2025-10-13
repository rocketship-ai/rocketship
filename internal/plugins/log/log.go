package log

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&LogPlugin{})
}

// GetType returns the plugin type identifier
func (lp *LogPlugin) GetType() string {
	return "log"
}

// Activity executes the log operation
func (lp *LogPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	config := &LogConfig{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse log config: %w", err)
	}

	// Validate required fields
	if config.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	// Get state for template processing
	// Convert state to map[string]interface{} for template processing
	stateInterface := make(map[string]interface{})
	if stateStr, ok := p["state"].(map[string]string); ok {
		// Handle map[string]string format
		for k, v := range stateStr {
			stateInterface[k] = v
		}
	} else if stateInt, ok := p["state"].(map[string]interface{}); ok {
		// Handle map[string]interface{} format
		stateInterface = stateInt
	}

	// Process templates in the message (config vars already processed by CLI)
	context := dsl.TemplateContext{
		Runtime: stateInterface,
	}

	processedMessage, err := dsl.ProcessTemplate(config.Message, context)
	if err != nil {
		return nil, fmt.Errorf("failed to process message template: %w", err)
	}

	// Log the message for debugging purposes
	logger.Info(processedMessage)

	// Return result with log information for the workflow to send to engine
	result := map[string]interface{}{
		"message":     processedMessage,
		"logged":      true,
		"log_message": processedMessage,
		"log_color":   "n/a",
		"log_bold":    false,
	}

	return result, nil
}

// parseConfig parses the configuration into the LogConfig struct
func parseConfig(configData map[string]interface{}, config *LogConfig) error {
	if message, ok := configData["message"].(string); ok {
		config.Message = message
	}

	return nil
}
