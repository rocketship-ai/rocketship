package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/itchyny/gojq"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&AgentPlugin{})
}

// AgentPlugin implements the agent plugin
type AgentPlugin struct{}

// GetType returns the plugin type identifier
func (ap *AgentPlugin) GetType() string {
	return "agent"
}

// Activity executes the agent operation
func (ap *AgentPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	config := &Config{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse agent config: %w", err)
	}

	// Validate required fields
	if config.Agent == "" {
		return nil, fmt.Errorf("agent is required")
	}

	if config.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Set defaults
	if config.Mode == "" {
		config.Mode = string(ModeSingle)
	}

	if config.MaxTurns == 0 {
		config.MaxTurns = 1
	}

	if config.Timeout == "" {
		config.Timeout = "30s"
	}

	if config.OutputFormat == "" {
		config.OutputFormat = string(FormatJSON)
	}

	config.SaveFullResponse = true // Always default to true

	// Validate agent type
	switch AgentType(config.Agent) {
	case AgentClaudeCode:
		// Valid
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", config.Agent)
	}

	// Validate mode
	switch ExecutionMode(config.Mode) {
	case ModeSingle, ModeContinue, ModeResume:
		// Valid
	default:
		return nil, fmt.Errorf("invalid mode: %s", config.Mode)
	}

	// Validate output format
	switch OutputFormat(config.OutputFormat) {
	case FormatText, FormatJSON, FormatStreamingJSON:
		// Valid
	default:
		return nil, fmt.Errorf("invalid output format: %s", config.OutputFormat)
	}

	// Validate session_id is provided when mode is resume
	if config.Mode == string(ModeResume) && config.SessionID == "" {
		return nil, fmt.Errorf("session_id is required when mode is 'resume'")
	}

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

	// Process templates in the prompt and session_id
	templateContext := dsl.TemplateContext{
		Runtime: stateInterface,
	}

	processedPrompt, err := dsl.ProcessTemplate(config.Prompt, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process prompt template: %w", err)
	}

	processedSessionID := ""
	if config.SessionID != "" {
		processedSessionID, err = dsl.ProcessTemplate(config.SessionID, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to process session_id template: %w", err)
		}
	}

	// Parse timeout
	timeout, err := time.ParseDuration(config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute based on agent type
	var response *Response
	switch AgentType(config.Agent) {
	case AgentClaudeCode:
		executor := NewClaudeCodeExecutor()
		if err := executor.ValidateAvailability(); err != nil {
			return nil, fmt.Errorf("claude-code agent not available: %w", err)
		}

		// Create execution config
		execConfig := &ClaudeCodeConfig{
			Prompt:         processedPrompt,
			Mode:           ExecutionMode(config.Mode),
			SessionID:      processedSessionID,
			MaxTurns:       config.MaxTurns,
			SystemPrompt:   config.SystemPrompt,
			OutputFormat:   OutputFormat(config.OutputFormat),
			ContinueRecent: config.ContinueRecent,
		}

		response, err = executor.Execute(timeoutCtx, execConfig)
		if err != nil {
			return nil, fmt.Errorf("claude-code execution failed: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported agent type: %s", config.Agent)
	}

	logger.Info(fmt.Sprintf("Agent %s executed successfully. Session: %s, Cost: $%.4f, Duration: %v", 
		config.Agent, response.SessionID, response.Cost, response.Duration))

	// Process saves
	saved := make(map[string]string)
	if err := processSaves(p, response, saved); err != nil {
		return nil, fmt.Errorf("failed to process saves: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"success":    response.ExitCode == 0,
		"session_id": response.SessionID,
		"cost":       response.Cost,
		"duration":   response.Duration.String(),
		"exit_code":  response.ExitCode,
		"saved":      saved, // Add saved values to result
	}

	// Save response data to result
	if config.SaveFullResponse {
		result["response"] = response.Content
	}

	if response.Error != "" {
		result["error"] = response.Error
	}

	// Add metadata if available
	for key, value := range response.Metadata {
		result[key] = value
	}

	return result, nil
}

// parseConfig parses the configuration into the Config struct
func parseConfig(configData map[string]interface{}, config *Config) error {
	// Parse each field from the config data
	if agent, ok := configData["agent"].(string); ok {
		config.Agent = agent
	}

	if prompt, ok := configData["prompt"].(string); ok {
		config.Prompt = prompt
	}

	if mode, ok := configData["mode"].(string); ok {
		config.Mode = mode
	}

	if sessionID, ok := configData["session_id"].(string); ok {
		config.SessionID = sessionID
	}

	if maxTurns, ok := configData["max_turns"].(float64); ok {
		config.MaxTurns = int(maxTurns)
	} else if maxTurns, ok := configData["max_turns"].(int); ok {
		config.MaxTurns = maxTurns
	}

	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	if systemPrompt, ok := configData["system_prompt"].(string); ok {
		config.SystemPrompt = systemPrompt
	}

	if outputFormat, ok := configData["output_format"].(string); ok {
		config.OutputFormat = outputFormat
	}

	if continueRecent, ok := configData["continue_recent"].(bool); ok {
		config.ContinueRecent = continueRecent
	}

	if saveFullResponse, ok := configData["save_full_response"].(bool); ok {
		config.SaveFullResponse = saveFullResponse
	}

	return nil
}

// processSaves processes the save configuration and extracts values from the agent response
func processSaves(p map[string]interface{}, response *Response, saved map[string]string) error {
	saves, ok := p["save"].([]interface{})
	if !ok {
		log.Printf("[DEBUG] No saves configured")
		return nil
	}

	log.Printf("[DEBUG] Processing %d saves", len(saves))
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

		// Handle JSON path save from agent response
		if jsonPath, ok := saveMap["json_path"].(string); ok && jsonPath != "" {
			log.Printf("[DEBUG] Processing JSON path save: '%s' as %s", jsonPath, as)
			
			// Create a JSON object with the agent response structure
			agentResult := map[string]interface{}{
				"response":   response.Content,
				"session_id": response.SessionID,
				"cost":       response.Cost,
				"duration":   response.Duration.String(),
				"exit_code":  response.ExitCode,
				"success":    response.ExitCode == 0,
			}

			// Add error if present
			if response.Error != "" {
				agentResult["error"] = response.Error
			}

			// Add metadata if available
			for key, value := range response.Metadata {
				agentResult[key] = value
			}

			query, err := gojq.Parse(jsonPath)
			if err != nil {
				log.Printf("[ERROR] Failed to parse jq expression: %v", err)
				return fmt.Errorf("failed to parse jq expression %q: %w", jsonPath, err)
			}

			iter := query.Run(agentResult)
			v, ok := iter.Next()
			if !ok {
				if required {
					log.Printf("[ERROR] No results from required jq expression %q. Agent result: %+v", jsonPath, agentResult)
					return fmt.Errorf("no results from required jq expression %q", jsonPath)
				}
				log.Printf("[WARN] No results from optional jq expression %q, skipping save", jsonPath)
				continue
			}
			if err, ok := v.(error); ok {
				log.Printf("[ERROR] Error evaluating jq expression: %v", err)
				return fmt.Errorf("error evaluating jq expression %q: %w", jsonPath, err)
			}

			// Handle different value types
			switch val := v.(type) {
			case string:
				saved[as] = val
			case float64:
				saved[as] = fmt.Sprintf("%.0f", val) // Remove decimal for whole numbers
			case bool:
				saved[as] = fmt.Sprintf("%t", val)
			case nil:
				if required {
					return fmt.Errorf("required value for %q is null", as)
				}
				saved[as] = ""
			default:
				// For complex types, use JSON marshaling
				bytes, err := json.Marshal(val)
				if err != nil {
					log.Printf("[ERROR] Failed to marshal value: %v", err)
					return fmt.Errorf("failed to marshal value for %q: %w", as, err)
				}
				saved[as] = string(bytes)
			}

			log.Printf("[DEBUG] Saved value for %s: %s (type: %T)", as, saved[as], v)
		}
	}

	log.Printf("[DEBUG] Final saved values: %+v", saved)
	return nil
}