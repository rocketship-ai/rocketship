package script

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"github.com/rocketship-ai/rocketship/internal/plugins/script/executors"
	"github.com/rocketship-ai/rocketship/internal/plugins/script/runtime"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&ScriptPlugin{})
}

// ScriptPlugin implements the script plugin
type ScriptPlugin struct{}

// GetType returns the plugin type for registration
func (p *ScriptPlugin) GetType() string {
	return "script"
}

// Activity executes the script plugin activity
func (p *ScriptPlugin) Activity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Parse activity request
	req, err := p.parseRequest(params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Parse script configuration
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Get script content
	script, err := p.getScriptContent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get script content: %w", err)
	}

	// Process runtime ({{ key }}) + env ({{ .env.KEY }}) templates in script content.
	runtimeVars := make(map[string]interface{}, len(req.State))
	for key, value := range req.State {
		runtimeVars[key] = value
	}
	script, err = dsl.ProcessTemplate(script, dsl.TemplateContext{
		Runtime: runtimeVars,
		Env:     req.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("script template processing failed: %w", err)
	}

	// Create executor for the specified language
	executor, err := executors.NewExecutor(config.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Validate script
	if err := executor.ValidateScript(script); err != nil {
		return nil, fmt.Errorf("script validation failed: %w", err)
	}

	// Create runtime context
	rtCtx := runtime.NewContext(req.State, req.Vars, req.Env)

	// Set up timeout if specified
	execCtx := ctx
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute script
	if err := executor.Execute(execCtx, script, rtCtx); err != nil {
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	// Return saved values
	return &ActivityResponse{
		Saved: rtCtx.Saved,
	}, nil
}

// parseRequest parses the activity request parameters
func (p *ScriptPlugin) parseRequest(params map[string]interface{}) (ActivityRequest, error) {
	var req ActivityRequest

	// Extract required fields
	if name, ok := params["name"].(string); ok {
		req.Name = name
	}
	if plugin, ok := params["plugin"].(string); ok {
		req.Plugin = plugin
	}
	if config, ok := params["config"].(map[string]interface{}); ok {
		req.Config = config
	}
	if state, ok := params["state"].(map[string]string); ok {
		req.State = state
	} else {
		// Handle the case where state comes as map[string]interface{}
		if stateInterface, ok := params["state"].(map[string]interface{}); ok {
			req.State = make(map[string]string)
			for k, v := range stateInterface {
				if strVal, ok := v.(string); ok {
					req.State[k] = strVal
				} else {
					req.State[k] = fmt.Sprintf("%v", v)
				}
			}
		} else {
			req.State = make(map[string]string)
		}
	}
	if vars, ok := params["vars"].(map[string]interface{}); ok {
		req.Vars = vars
	} else {
		req.Vars = make(map[string]interface{})
	}

	// Extract env secrets from params (for {{ .env.* }} template resolution)
	if envData, ok := params["env"].(map[string]interface{}); ok {
		req.Env = make(map[string]string)
		for k, v := range envData {
			if strVal, ok := v.(string); ok {
				req.Env[k] = strVal
			}
		}
	} else if envData, ok := params["env"].(map[string]string); ok {
		req.Env = envData
	} else {
		req.Env = make(map[string]string)
	}

	return req, nil
}

// parseConfig parses the script configuration
func (p *ScriptPlugin) parseConfig(configMap map[string]interface{}) (ScriptConfig, error) {
	var config ScriptConfig

	// Convert map to JSON and back to struct for easy parsing
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return config, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(jsonData, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if config.Language == "" {
		return config, fmt.Errorf("language is required")
	}

	// Check that either script or file is provided
	if config.Script == "" && config.File == "" {
		return config, fmt.Errorf("either 'script' or 'file' must be provided")
	}

	if config.Script != "" && config.File != "" {
		return config, fmt.Errorf("only one of 'script' or 'file' can be provided")
	}

	return config, nil
}

// getScriptContent retrieves the script content from inline or file
func (p *ScriptPlugin) getScriptContent(config ScriptConfig) (string, error) {
	if config.Script != "" {
		return config.Script, nil
	}

	if config.File != "" {
		content, err := os.ReadFile(config.File)
		if err != nil {
			return "", fmt.Errorf("failed to read script file %s: %w", config.File, err)
		}
		return string(content), nil
	}

	return "", fmt.Errorf("no script content available")
}
