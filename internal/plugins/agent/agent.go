package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/rocketship-ai/rocketship/internal/browser/sessionfile"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&AgentPlugin{})
}

// AgentPlugin implements the agent plugin using Claude Agent SDK
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

	// Get state for template processing
	stateInterface := extractState(p)

	// Add run metadata to state for template processing
	if runData, ok := p["run"].(map[string]interface{}); ok {
		stateInterface["run"] = runData
	}

	// Process templates in configuration
	templateContext := dsl.TemplateContext{
		Runtime: stateInterface,
	}

	config, err := parseConfig(configData, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent config: %w", err)
	}

	// Validate required fields
	if config.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Set defaults
	if config.Mode == "" {
		config.Mode = ModeSingle
	}

	// Add default system prompt for QA testing if user didn't provide one
	if config.SystemPrompt == "" {
		config.SystemPrompt = `You are a QA testing agent executing automated test tasks.

Ultrathink. Use the MCP servers available to you if applicable.

ALWAYS return your final result as JSON with this EXACT schema:
- If the task succeeds: {"ok": true, "result": "<a string describing what you found/did>", "variables": {"var_name": <value>, ...}}
- If the task fails: {"ok": false, "error": "<a string describing the specific reason for failure>"}

IMPORTANT: When the user asks to "save X as 'var_name'", you MUST include a "variables" object.
Example: "save the heading as 'page_heading'" → {"ok": true, "result": "Found heading", "variables": {"page_heading": "Example Domain"}}

No code changing. No awaiting user input. If you need file writing as a scratchpad, write to mkdir -p .rocketship/tmp directory.
Clean it up after you are done with the task.`
	}

	// Default to wildcard tools - if you're using MCP servers, you want to use them
	if len(config.AllowedTools) == 0 {
		config.AllowedTools = []string{"*"}
	}

	// max_turns and timeout default to "no limit" - don't set defaults
	// The SDK will handle unlimited turns if not specified

	// Validate mode
	switch config.Mode {
	case ModeSingle, ModeContinue, ModeResume:
		// Valid
	default:
		return nil, fmt.Errorf("invalid mode: %s (must be: single, continue, or resume)", config.Mode)
	}

	// Validate session_id is provided when mode is resume
	if config.Mode == ModeResume && config.SessionID == "" {
		return nil, fmt.Errorf("session_id is required when mode is 'resume'")
	}

	// Create context with timeout if specified, otherwise use unlimited
	var timeoutCtx context.Context
	var cancel context.CancelFunc
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
		timeoutCtx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		// No timeout specified - use context without timeout
		timeoutCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Execute agent
	logMsg := fmt.Sprintf("Executing Claude agent (mode=%s", config.Mode)
	if config.MaxTurns > 0 {
		logMsg += fmt.Sprintf(", max_turns=%d", config.MaxTurns)
	}
	if config.Timeout != "" {
		logMsg += fmt.Sprintf(", timeout=%s", config.Timeout)
	}
	logMsg += ")"
	logger.Info(logMsg)

	result, err := ap.execute(timeoutCtx, config, stateInterface)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	// Check if agent execution was successful - fail the test step if not
	if !result.Success {
		if result.Response.Traceback != "" {
			logger.Debug("Agent traceback", "traceback", result.Response.Traceback)
		}
		if result.Response.Error != "" {
			return nil, fmt.Errorf("agent execution failed: %s", result.Response.Error)
		}
		return nil, fmt.Errorf("agent execution failed with no error message")
	}

	// Process saves
	saved := make(map[string]string)
	if err := processSaves(p, result, saved); err != nil {
		return nil, fmt.Errorf("failed to process saves: %w", err)
	}

	// Build final result
	finalResult := map[string]interface{}{
		"success": result.Success,
		"result":  result.Response.Result,
	}

	if result.Response.SessionID != "" {
		finalResult["session_id"] = result.Response.SessionID
	}

	if result.Response.Mode != "" {
		finalResult["mode"] = result.Response.Mode
	}

	if !result.Success && result.Response.Error != "" {
		finalResult["error"] = result.Response.Error
	}

	if len(saved) > 0 {
		finalResult["saved"] = saved
	}

	// Add metadata if available
	for key, value := range result.Response.Metadata {
		finalResult[key] = value
	}

	logger.Info(fmt.Sprintf("Agent execution completed successfully (duration=%s)", result.Duration))

	return finalResult, nil
}

// execute runs the Python executor with the agent configuration
func (ap *AgentPlugin) execute(ctx context.Context, cfg *Config, state map[string]interface{}) (*ExecutorResult, error) {
	startTime := time.Now()

	// Prepare executor script
	executorPath, cleanup, err := prepareExecutorScript()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Set default cwd to current working directory if not specified
	if cfg.Cwd == "" {
		if cwd, err := os.Getwd(); err == nil {
			cfg.Cwd = cwd
			log.Printf("[DEBUG] Agent cwd defaulting to: %s", cwd)
		}
	}

	// Check if agent needs Playwright MCP with CDP connection
	if cfg.MCPServers != nil {
		if playwrightServer, ok := cfg.MCPServers["playwright"]; ok {
			// Ensure the server config has args array
			if playwrightServer.Args == nil {
				playwrightServer.Args = []string{}
			}

			// Configure output directory for MCP screenshots
			// All MCP-generated artifacts go to .rocketship/tmp/mcp-<server-name>/
			outputDir := ".rocketship/tmp/mcp-playwright"
			playwrightServer.Args = append(playwrightServer.Args, "--output-dir", outputDir)
			log.Printf("[DEBUG] Configuring Playwright MCP with output directory: %s", outputDir)

			// If session_id is specified in config, try to get CDP endpoint
			if cfg.SessionID != "" {
				wsEndpoint, _, err := sessionfile.Read(ctx, cfg.SessionID)
				if err == nil && wsEndpoint != "" {
					// Configure Playwright MCP server with CDP endpoint
					log.Printf("[DEBUG] Configuring Playwright MCP with CDP endpoint: %s", wsEndpoint)

					// Add CDP endpoint flag
					playwrightServer.Args = append(playwrightServer.Args, "--cdp-endpoint", wsEndpoint)
				} else if err != nil {
					log.Printf("[WARN] Failed to read session file for CDP connection: %v", err)
				}
			}

			// Update the config
			cfg.MCPServers["playwright"] = playwrightServer
		}
	}

	// Build configuration JSON for Python executor
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Build command
	args := []string{
		executorPath,
		"--config-json", string(configJSON),
	}

	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	// Capture stdout and stderr separately
	// Python executor logs to stderr, JSON response to stdout
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err = cmd.Run()
	duration := time.Since(startTime)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute agent: %w", err)
		}
	}

	// Get outputs
	stdoutStr := strings.TrimSpace(stdout.String())
	stderrStr := strings.TrimSpace(stderr.String())

	// DEBUG: Log stderr (Python logs) if present
	if stderrStr != "" {
		log.Printf("[DEBUG] Python executor logs:\n%s", stderrStr)
	}

	// DEBUG: Log stdout (JSON response)
	log.Printf("[DEBUG] Python executor JSON output: %s", func() string {
		if len(stdoutStr) > 500 {
			return stdoutStr[:500] + "..."
		}
		return stdoutStr
	}())

	// Parse response from stdout (JSON only, no log mixing)
	response := &Response{}

	if stdoutStr == "" {
		errorMsg := "agent executor returned no output"
		if stderrStr != "" {
			errorMsg += "\nStderr: " + stderrStr
		}
		return &ExecutorResult{
			Success:   false,
			Response:  &Response{Success: false, Error: errorMsg},
			Duration:  duration,
			RawOutput: stdoutStr,
			ExitCode:  exitCode,
		}, nil
	}

	// Parse JSON from stdout (clean, no log mixing)
	if err := json.Unmarshal([]byte(stdoutStr), response); err != nil {
		// JSON parsing failed - include both stdout and stderr for debugging
		errorMsg := fmt.Sprintf("failed to parse JSON from executor output: %s", err)
		if stderrStr != "" {
			errorMsg += fmt.Sprintf("\n\nStderr logs:\n%s", stderrStr)
		}
		errorMsg += fmt.Sprintf("\n\nStdout (expected JSON):\n%s", stdoutStr)

		return &ExecutorResult{
			Success: false,
			Response: &Response{
				Success: false,
				Error:   errorMsg,
			},
			Duration:  duration,
			RawOutput: stdoutStr,
			ExitCode:  exitCode,
		}, nil
	}

	// Check if execution was successful
	if !response.Success {
		log.Printf("[DEBUG] Agent execution failed: %s", response.Error)
		if response.Traceback != "" {
			log.Printf("[DEBUG] Traceback:\n%s", response.Traceback)
		}
	}

	return &ExecutorResult{
		Success:    response.Success,
		Response:   response,
		Duration:   duration,
		RawOutput:  stdoutStr,
		ExitCode:   exitCode,
		ErrorTrace: response.Traceback,
	}, nil
}

// parseConfig parses the configuration and applies template processing
func parseConfig(configData map[string]interface{}, templateContext dsl.TemplateContext) (*Config, error) {
	config := &Config{}

	// Parse prompt (required)
	if prompt, ok := configData["prompt"].(string); ok {
		processed, err := dsl.ProcessTemplate(prompt, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to process prompt template: %w", err)
		}
		config.Prompt = processed
	}

	// Parse mode
	if mode, ok := configData["mode"].(string); ok {
		config.Mode = ExecutionMode(mode)
	}

	// Parse session_id
	if sessionID, ok := configData["session_id"].(string); ok {
		processed, err := dsl.ProcessTemplate(sessionID, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to process session_id template: %w", err)
		}
		config.SessionID = processed
	}

	// Parse max_turns
	if maxTurns, ok := configData["max_turns"].(float64); ok {
		config.MaxTurns = int(maxTurns)
	} else if maxTurns, ok := configData["max_turns"].(int); ok {
		config.MaxTurns = maxTurns
	}

	// Parse timeout
	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	// Parse system_prompt
	if systemPrompt, ok := configData["system_prompt"].(string); ok {
		processed, err := dsl.ProcessTemplate(systemPrompt, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to process system_prompt template: %w", err)
		}
		config.SystemPrompt = processed
	}

	// NOTE: permission_mode is hardcoded to 'bypassPermissions' in the Python executor
	// This is a QA testing agent - it should never ask for permission or modify files

	// Parse cwd
	if cwd, ok := configData["cwd"].(string); ok {
		config.Cwd = cwd
	}

	// Parse MCP servers
	if mcpServers, ok := configData["mcp_servers"].(map[string]interface{}); ok {
		config.MCPServers = make(map[string]MCPServerConfig)
		for name, serverData := range mcpServers {
			serverMap, ok := serverData.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid mcp_servers config for %q", name)
			}

			serverConfig, err := parseMCPServerConfig(serverMap, templateContext)
			if err != nil {
				return nil, fmt.Errorf("failed to parse mcp_server %q: %w", name, err)
			}

			config.MCPServers[name] = *serverConfig
		}
	}

	// Parse allowed_tools
	if allowedTools, ok := configData["allowed_tools"].([]interface{}); ok {
		config.AllowedTools = make([]string, len(allowedTools))
		for i, tool := range allowedTools {
			if toolStr, ok := tool.(string); ok {
				config.AllowedTools[i] = toolStr
			}
		}
	} else if allowedToolsStr, ok := configData["allowed_tools"].(string); ok {
		// Support single string (e.g., "*")
		config.AllowedTools = []string{allowedToolsStr}
	}

	return config, nil
}

// parseMCPServerConfig parses an MCP server configuration
func parseMCPServerConfig(serverData map[string]interface{}, templateContext dsl.TemplateContext) (*MCPServerConfig, error) {
	config := &MCPServerConfig{}

	// Parse type (required)
	if serverType, ok := serverData["type"].(string); ok {
		config.Type = MCPServerType(serverType)
	} else {
		return nil, fmt.Errorf("type is required for MCP server")
	}

	// Parse fields based on server type
	switch config.Type {
	case MCPServerTypeStdio:
		// Parse command (required for stdio)
		if command, ok := serverData["command"].(string); ok {
			config.Command = command
		} else {
			return nil, fmt.Errorf("command is required for stdio MCP server")
		}

		// Parse args
		if args, ok := serverData["args"].([]interface{}); ok {
			config.Args = make([]string, len(args))
			for i, arg := range args {
				config.Args[i] = fmt.Sprint(arg)
			}
		}

		// Parse env
		if env, ok := serverData["env"].(map[string]interface{}); ok {
			config.Env = make(map[string]string)
			for k, v := range env {
				value := fmt.Sprint(v)
				processed, err := dsl.ProcessTemplate(value, templateContext)
				if err != nil {
					return nil, fmt.Errorf("failed to process env %q: %w", k, err)
				}
				config.Env[k] = processed
			}
		}

	case MCPServerTypeSSE:
		// Parse URL (required for sse)
		if url, ok := serverData["url"].(string); ok {
			config.URL = url
		} else {
			return nil, fmt.Errorf("url is required for sse MCP server")
		}

		// Parse headers
		if headers, ok := serverData["headers"].(map[string]interface{}); ok {
			config.Headers = make(map[string]string)
			for k, v := range headers {
				value := fmt.Sprint(v)
				processed, err := dsl.ProcessTemplate(value, templateContext)
				if err != nil {
					return nil, fmt.Errorf("failed to process header %q: %w", k, err)
				}
				config.Headers[k] = processed
			}
		}

	default:
		return nil, fmt.Errorf("unsupported MCP server type: %s", config.Type)
	}

	return config, nil
}

// extractState extracts state from parameters
func extractState(p map[string]interface{}) map[string]interface{} {
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
	return stateInterface
}

// processSaves processes the save configuration and extracts values from the agent response
func processSaves(p map[string]interface{}, execResult *ExecutorResult, saved map[string]string) error {
	// First: Auto-save from agent's variables field (natural language saves)
	if len(execResult.Response.Variables) > 0 {
		log.Printf("[DEBUG] Auto-saving %d variables from agent response", len(execResult.Response.Variables))
		for key, value := range execResult.Response.Variables {
			saved[key] = value
			log.Printf("[DEBUG] Auto-saved variable: %s = %s", key, value)
		}
	}

	// Second: Process explicit save configurations (can override auto-saved variables)
	saves, ok := p["save"].([]interface{})
	if !ok {
		return nil
	}

	log.Printf("[DEBUG] Processing %d explicit save configs", len(saves))
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
				"result":     execResult.Response.Result,
				"session_id": execResult.Response.SessionID,
				"mode":       execResult.Response.Mode,
				"success":    execResult.Response.Success,
			}

			// Add error if present
			if execResult.Response.Error != "" {
				agentResult["error"] = execResult.Response.Error
			}

			// Add metadata if available
			for key, value := range execResult.Response.Metadata {
				agentResult[key] = value
			}

			query, err := gojq.Parse(jsonPath)
			if err != nil {
				return fmt.Errorf("failed to parse jq expression %q: %w", jsonPath, err)
			}

			iter := query.Run(agentResult)
			v, ok := iter.Next()
			if !ok {
				if required {
					return fmt.Errorf("no results from required jq expression %q", jsonPath)
				}
				log.Printf("[WARN] No results from optional jq expression %q, skipping save", jsonPath)
				continue
			}
			if err, ok := v.(error); ok {
				return fmt.Errorf("error evaluating jq expression %q: %w", jsonPath, err)
			}

			// Handle different value types
			switch val := v.(type) {
			case string:
				saved[as] = val
			case float64:
				saved[as] = fmt.Sprintf("%.0f", val)
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
