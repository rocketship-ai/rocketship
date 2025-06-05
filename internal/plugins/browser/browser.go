package browser

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&BrowserPlugin{})
}

// BrowserPlugin implements the browser plugin
type BrowserPlugin struct{}

// GetType returns the plugin type identifier
func (bp *BrowserPlugin) GetType() string {
	return "browser"
}

// Activity executes the browser operation
func (bp *BrowserPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting browser plugin execution")

	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	config := &Config{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse browser config: %w", err)
	}

	// Validate required fields
	if config.Task == "" {
		return nil, fmt.Errorf("task is required")
	}

	if config.LLM.Provider == "" {
		return nil, fmt.Errorf("LLM provider is required")
	}

	if config.LLM.Model == "" {
		return nil, fmt.Errorf("LLM model is required")
	}

	// Set defaults
	if config.ExecutorType == "" {
		config.ExecutorType = "python"
	}

	if config.Timeout == "" {
		config.Timeout = "5m"
	}

	if config.MaxSteps == 0 {
		config.MaxSteps = 50
	}

	if config.BrowserType == "" {
		config.BrowserType = "chromium"
	}

	// Set viewport defaults if not specified
	if config.Viewport.Width == 0 {
		config.Viewport.Width = 1920
	}
	if config.Viewport.Height == 0 {
		config.Viewport.Height = 1080
	}

	// Don't override these values - they should come from the YAML config
	// Only set defaults if they weren't specified
	// Note: Go's bool zero value is false, so we can't distinguish between
	// "not set" and "explicitly set to false" without using pointers

	logger.Info("Browser config parsed",
		"task", config.Task,
		"executor_type", config.ExecutorType,
		"llm_provider", config.LLM.Provider,
		"llm_model", config.LLM.Model,
		"timeout", config.Timeout,
		"max_steps", config.MaxSteps,
		"headless", config.Headless,
		"use_vision", config.UseVision,
		"viewport_width", config.Viewport.Width,
		"viewport_height", config.Viewport.Height)

	// Get state for template processing
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

	// Process templates in the task and other config fields
	templateContext := dsl.TemplateContext{
		Runtime: stateInterface,
	}

	processedTask, err := dsl.ProcessTemplate(config.Task, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process task template: %w", err)
	}
	config.Task = processedTask

	logger.Info("Processed task template", "processed_task", processedTask)

	// Parse timeout
	timeout, err := time.ParseDuration(config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Initialize executor
	var executor BrowserExecutor
	switch config.ExecutorType {
	case "python":
		executor = NewPythonExecutor()
	default:
		return nil, fmt.Errorf("unsupported executor type: %s", config.ExecutorType)
	}

	// Validate executor availability
	logger.Info("Validating executor availability")
	if err := executor.ValidateAvailability(); err != nil {
		return nil, fmt.Errorf("browser executor not available: %w", err)
	}

	// Execute browser automation
	logger.Info("Starting browser execution")
	response, err := executor.Execute(timeoutCtx, config)
	if err != nil {
		return nil, fmt.Errorf("browser execution failed: %w", err)
	}

	logger.Info("Browser execution completed",
		"success", response.Success,
		"steps_count", len(response.Steps),
		"duration", response.Duration,
		"has_error", response.Error != "")

	// Process saves
	saved := make(map[string]string)
	if err := processSaves(p, response, saved); err != nil {
		return nil, fmt.Errorf("failed to process saves: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"success":        response.Success,
		"result":         response.Result,
		"session_id":     response.SessionID,
		"steps":          response.Steps,
		"screenshots":    response.Screenshots,
		"extracted_data": response.ExtractedData,
		"duration":       response.Duration.String(),
		"saved":          saved,
	}

	if response.Error != "" {
		result["error"] = response.Error
	}

	logger.Info("Browser plugin execution completed successfully")
	return result, nil
}

// parseConfig parses the configuration into the Config struct
func parseConfig(configData map[string]interface{}, config *Config) error {
	// Parse task
	if task, ok := configData["task"].(string); ok {
		config.Task = task
	}

	// Parse LLM config
	if llmData, ok := configData["llm"].(map[string]interface{}); ok {
		if provider, ok := llmData["provider"].(string); ok {
			config.LLM.Provider = provider
		}
		if model, ok := llmData["model"].(string); ok {
			config.LLM.Model = model
		}
		if configMap, ok := llmData["config"].(map[string]interface{}); ok {
			config.LLM.Config = make(map[string]string)
			for k, v := range configMap {
				if strVal, ok := v.(string); ok {
					config.LLM.Config[k] = strVal
				}
			}
		}
	}

	// Parse other fields
	if executorType, ok := configData["executor_type"].(string); ok {
		config.ExecutorType = executorType
	}

	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	if maxSteps, ok := configData["max_steps"].(float64); ok {
		config.MaxSteps = int(maxSteps)
	} else if maxSteps, ok := configData["max_steps"].(int); ok {
		config.MaxSteps = maxSteps
	}

	if browserType, ok := configData["browser_type"].(string); ok {
		config.BrowserType = browserType
	}

	if headless, ok := configData["headless"].(bool); ok {
		config.Headless = headless
	}

	if useVision, ok := configData["use_vision"].(bool); ok {
		config.UseVision = useVision
	}

	if sessionID, ok := configData["session_id"].(string); ok {
		config.SessionID = sessionID
	}

	if saveScreenshots, ok := configData["save_screenshots"].(bool); ok {
		config.SaveScreenshots = saveScreenshots
	}

	// Parse allowed domains
	if domainsInterface, ok := configData["allowed_domains"].([]interface{}); ok {
		for _, domain := range domainsInterface {
			if domainStr, ok := domain.(string); ok {
				config.AllowedDomains = append(config.AllowedDomains, domainStr)
			}
		}
	}

	// Parse viewport
	if viewportData, ok := configData["viewport"].(map[string]interface{}); ok {
		if width, ok := viewportData["width"].(float64); ok {
			config.Viewport.Width = int(width)
		} else if width, ok := viewportData["width"].(int); ok {
			config.Viewport.Width = width
		}
		if height, ok := viewportData["height"].(float64); ok {
			config.Viewport.Height = int(height)
		} else if height, ok := viewportData["height"].(int); ok {
			config.Viewport.Height = height
		}
	}

	return nil
}

// processSaves processes the save configuration and extracts values from the browser response
func processSaves(p map[string]interface{}, response *BrowserResponse, saved map[string]string) error {
	saves, ok := p["save"].([]interface{})
	if !ok {
		log.Printf("[DEBUG] No saves configured for browser plugin")
		return nil
	}

	log.Printf("[DEBUG] Processing %d saves for browser plugin", len(saves))
	for _, save := range saves {
		saveMap, ok := save.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid save format: got type %T", save)
		}

		as, ok := saveMap["as"].(string)
		if !ok {
			return fmt.Errorf("'as' field is required for save")
		}

		// Check if required is explicitly set to false
		required := true
		if req, ok := saveMap["required"].(bool); ok {
			required = req
		}

		// Handle JSON path save from browser response
		if jsonPath, ok := saveMap["json_path"].(string); ok && jsonPath != "" {
			log.Printf("[DEBUG] Processing JSON path save: '%s' as %s", jsonPath, as)

			// Create a JSON object with the browser response structure
			browserResult := map[string]interface{}{
				"success":        response.Success,
				"result":         response.Result,
				"session_id":     response.SessionID,
				"steps":          response.Steps,
				"screenshots":    response.Screenshots,
				"extracted_data": response.ExtractedData,
				"duration":       response.Duration.String(),
			}

			// Add error if present
			if response.Error != "" {
				browserResult["error"] = response.Error
			}

			// For now, use simple field access instead of jq
			// TODO: Implement proper jq processing later
			value, err := extractSimpleValue(browserResult, jsonPath)
			if err != nil {
				if required {
					return fmt.Errorf("failed to extract required value for %q: %w", as, err)
				}
				log.Printf("[WARN] Failed to extract optional value for %q: %v", as, err)
				continue
			}

			saved[as] = value
			log.Printf("[DEBUG] Saved value for %s: %s", as, value)
		}
	}

	log.Printf("[DEBUG] Final saved values from browser plugin: %+v", saved)
	return nil
}

// extractSimpleValue extracts values using simple field access (temporary implementation)
func extractSimpleValue(data map[string]interface{}, path string) (string, error) {
	// Handle simple cases like ".result", ".success", etc.
	if len(path) > 0 && path[0] == '.' {
		fieldName := path[1:]
		if value, exists := data[fieldName]; exists {
			return fmt.Sprintf("%v", value), nil
		}
		return "", fmt.Errorf("field %q not found", fieldName)
	}
	return "", fmt.Errorf("unsupported path format: %s", path)
}
