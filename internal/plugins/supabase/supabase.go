package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&SupabasePlugin{})
}

// GetType returns the plugin type identifier
func (sp *SupabasePlugin) GetType() string {
	return "supabase"
}

// Activity executes Supabase operations and returns results
func (sp *SupabasePlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Debug: Log all parameter keys
	paramKeys := make([]string, 0, len(p))
	for k := range p {
		paramKeys = append(paramKeys, k)
	}
	logger.Info("SUPABASE Activity called", "paramKeys", paramKeys)
	if saveData, ok := p["save"]; ok {
		saveDataJSON, _ := json.Marshal(saveData)
		logger.Info("SUPABASE save parameter exists", "save", string(saveDataJSON))
	} else {
		logger.Info("SUPABASE save parameter NOT FOUND")
	}

	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	// Get state for variable replacement
	state := make(map[string]string)
	if stateInterface, ok := p["state"]; ok {
		if stateMap, ok := stateInterface.(map[string]interface{}); ok {
			for k, v := range stateMap {
				state[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	config := &SupabaseConfig{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse Supabase config: %w", err)
	}

	// Process runtime variables in string fields
	config.URL = replaceVariables(config.URL, state)
	config.Key = replaceVariables(config.Key, state)
	if config.Table != "" {
		config.Table = replaceVariables(config.Table, state)
	}

	// Process runtime variables in RPC parameters
	if config.RPC != nil && config.RPC.Params != nil {
		config.RPC.Params = processVariablesInMap(config.RPC.Params, state)
	}

	// Process runtime variables in other operation types
	if config.Insert != nil {
		if config.Insert.Data != nil {
			if dataMap, ok := config.Insert.Data.(map[string]interface{}); ok {
				config.Insert.Data = processVariablesInMap(dataMap, state)
			}
		}
	}

	if config.Update != nil {
		if config.Update.Data != nil {
			config.Update.Data = processVariablesInMap(config.Update.Data, state)
		}
		config.Update.Filters = processFilters(config.Update.Filters, state)
	}

	if config.Select != nil {
		config.Select.Filters = processFilters(config.Select.Filters, state)
	}

	if config.Delete != nil {
		config.Delete.Filters = processFilters(config.Delete.Filters, state)
	}

	// Process runtime variables in Auth config
	if config.Auth != nil {
		config.Auth.Email = replaceVariables(config.Auth.Email, state)
		config.Auth.Password = replaceVariables(config.Auth.Password, state)
		config.Auth.UserID = replaceVariables(config.Auth.UserID, state)
		if config.Auth.UserMetadata != nil {
			config.Auth.UserMetadata = processVariablesInMap(config.Auth.UserMetadata, state)
		}
		if config.Auth.AppMetadata != nil {
			config.Auth.AppMetadata = processVariablesInMap(config.Auth.AppMetadata, state)
		}
	}

	// Log parsed config for debugging
	logger.Info("Parsed Supabase config",
		"operation", config.Operation,
		"hasRPC", config.RPC != nil,
		"rpcFunction", func() string {
			if config.RPC != nil {
				return config.RPC.Function
			}
			return "nil"
		}(),
		"rpcParams", func() string {
			if config.RPC != nil && config.RPC.Params != nil {
				paramsJSON, _ := json.Marshal(config.RPC.Params)
				return string(paramsJSON)
			}
			return "nil"
		}())

	// Validate required fields
	if config.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if config.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if config.Operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	logger.Info("Executing Supabase operation", "operation", config.Operation, "table", config.Table)

	// Set default timeout
	timeout := 30 * time.Second
	if config.Timeout != "" {
		if parsedTimeout, err := time.ParseDuration(config.Timeout); err == nil {
			timeout = parsedTimeout
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{Timeout: timeout}

	startTime := time.Now()
	response, err := executeSupabaseOperation(ctx, client, config)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("Supabase operation failed", "error", err, "duration", duration)
		return nil, err
	}

	// Check if the response contains an API error (HTTP 4xx/5xx)
	if response.Error != nil {
		statusCode := 0
		if response.Metadata != nil {
			statusCode = response.Metadata.StatusCode
		}
		logger.Error("Supabase API returned error",
			"error_message", response.Error.Message,
			"error_code", response.Error.Code,
			"status_code", statusCode,
			"duration", duration)

		return nil, fmt.Errorf("supabase api error (status %d): %s", statusCode, response.Error.Message)
	}

	// Add metadata
	if response.Metadata == nil {
		response.Metadata = &ResponseMetadata{}
	}
	response.Metadata.Operation = config.Operation
	response.Metadata.Table = config.Table
	response.Metadata.Duration = duration.String()

	logger.Info("Supabase operation completed", "operation", config.Operation, "duration", duration)

	// Process assertions
	if assertions, ok := p["assertions"].([]interface{}); ok {
		if err := processAssertions(response, assertions, p); err != nil {
			logger.Error("Assertion failed", "error", err)
			return nil, fmt.Errorf("assertion failed: %w", err)
		}
	}

	// Handle save operations
	saved := make(map[string]string)
	if saveConfigs, ok := p["save"].([]interface{}); ok {
		logger.Info("Processing save configs", "count", len(saveConfigs))
		for _, saveConfigInterface := range saveConfigs {
			if saveConfig, ok := saveConfigInterface.(map[string]interface{}); ok {
				logger.Info("Processing save config", "config", saveConfig)
				if err := processSave(ctx, response, saveConfig, saved); err != nil {
					return nil, fmt.Errorf("failed to save value: %w", err)
				}
				logger.Info("Successfully saved value", "saved", saved)
			}
		}
	}
	logger.Info("Final saved values", "saved", saved)

	return &ActivityResponse{
		Response: response,
		Saved:    saved,
	}, nil
}
