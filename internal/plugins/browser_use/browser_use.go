package browser_use

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/browser/sessionfile"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
)

func init() {
	plugins.RegisterPlugin(&Plugin{})
}

type Plugin struct{}

func (p *Plugin) GetType() string {
	return "browser_use"
}

type Config struct {
	SessionID      string
	Task           string
	AllowedDomains []string
	MaxSteps       int
	UseVision      bool
	Temperature    *float64
	Timeout        string
	LLM            LLMConfig
}

type LLMConfig struct {
	Provider string
	Model    string
	Config   map[string]string
}

type runnerResponse struct {
	Success   bool                   `json:"ok"`
	Error     string                 `json:"error,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
	FinalURL  string                 `json:"finalUrl,omitempty"`
	Artifacts map[string]interface{} `json:"artifacts,omitempty"`
	Traceback string                 `json:"traceback,omitempty"`
}

func (p *Plugin) Activity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	logger := getLogger(ctx)

	configData, ok := params["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	state := extractState(params)

	// Add run metadata to state for template processing
	if runData, ok := params["run"].(map[string]interface{}); ok {
		state["run"] = runData
	}

	templateContext := dsl.TemplateContext{Runtime: state}

	cfg, err := parseConfig(configData, templateContext)
	if err != nil {
		return nil, err
	}

	// Parse and apply timeout
	timeoutDuration, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format %q: %w (use duration strings like '5m', '30s')", cfg.Timeout, err)
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	wsEndpoint, _, err := sessionfile.Read(timeoutCtx, cfg.SessionID)
	if err != nil {
		return nil, fmt.Errorf("session %q is not active: %w", cfg.SessionID, err)
	}

	logger.Info("Executing browser-use task", "session_id", cfg.SessionID, "timeout", cfg.Timeout)

	runnerPath, cleanup, err := prepareRunnerScript()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	args := []string{
		runnerPath,
		"--ws-endpoint", wsEndpoint,
		"--task", cfg.Task,
	}

	// Add LLM config
	if cfg.LLM.Provider != "" {
		args = append(args, "--llm-provider", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "" {
		args = append(args, "--llm-model", cfg.LLM.Model)
	}

	for _, domain := range cfg.AllowedDomains {
		args = append(args, "--allowed-domain", domain)
	}

	if cfg.MaxSteps > 0 {
		args = append(args, "--max-steps", strconv.Itoa(cfg.MaxSteps))
	}

	if cfg.UseVision {
		args = append(args, "--use-vision")
	}

	if cfg.Temperature != nil {
		args = append(args, "--temperature", fmt.Sprintf("%f", *cfg.Temperature))
	}

	// Build environment with LLM API keys
	env := append(os.Environ(), "PYTHONUNBUFFERED=1")
	for key, value := range cfg.LLM.Config {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.CommandContext(timeoutCtx, "python3", args...)
	cmd.Env = env
	setupProcessGroup(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start browser-use runner: %w", err)
	}

	go func() {
		<-timeoutCtx.Done()
		killProcessGroup(cmd)
	}()

	outputBytes, readErr := io.ReadAll(stdoutPipe)
	waitErr := cmd.Wait()

	if readErr != nil {
		return nil, fmt.Errorf("failed to read runner output: %w", readErr)
	}

	output := strings.TrimSpace(string(outputBytes))

	if output == "" {
		if waitErr != nil {
			return nil, fmt.Errorf("browser-use execution failed: %w", waitErr)
		}
		return nil, errors.New("browser-use runner returned no output")
	}

	startIdx := strings.Index(output, "{")
	endIdx := strings.LastIndex(output, "}")
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		if waitErr != nil {
			return nil, fmt.Errorf("browser-use execution failed: %w\nstdout: %s", waitErr, output)
		}
		return nil, fmt.Errorf("no JSON found in runner output: %s", output)
	}

	response := runnerResponse{}
	if err := json.Unmarshal([]byte(output[startIdx:endIdx+1]), &response); err != nil {
		if waitErr != nil {
			return nil, fmt.Errorf("browser-use execution failed: %w\nstdout: %s", waitErr, output)
		}
		return nil, fmt.Errorf("failed to parse runner output: %w\nstdout: %s", err, output)
	}

	if !response.Success {
		if response.Traceback != "" {
			logger.Debug("browser_use traceback", "traceback", response.Traceback)
		}
		if response.Error != "" {
			return nil, fmt.Errorf("browser-use execution failed: %s", response.Error)
		}
		if waitErr != nil {
			return nil, fmt.Errorf("browser-use execution failed: %w\nstdout: %s", waitErr, output)
		}
		return nil, errors.New("browser-use execution failed with no error message")
	}

	if waitErr != nil {
		logger.Warn("browser-use runner exited with warning", "error", waitErr)
	}

	result := map[string]interface{}{
		"success":    true,
		"task":       cfg.Task,
		"final_url":  response.FinalURL,
		"result":     response.Result,
		"artifacts":  response.Artifacts,
		"session_id": cfg.SessionID,
		"max_steps":  cfg.MaxSteps,
		"use_vision": cfg.UseVision,
		"domains":    cfg.AllowedDomains,
	}

	saved := make(map[string]string)
	if err := processSaves(params, result, saved); err != nil {
		return nil, err
	}
	if len(saved) > 0 {
		result["saved"] = saved
	}

	if err := processAssertions(params, result, state); err != nil {
		return nil, err
	}

	logger.Info("browser-use task completed", "session_id", cfg.SessionID)

	return result, nil
}

func parseConfig(config map[string]interface{}, ctx dsl.TemplateContext) (*Config, error) {
	sessionID, err := templateStringField(config, "session_id", ctx)
	if err != nil {
		return nil, err
	}

	task, err := templateStringField(config, "task", ctx)
	if err != nil {
		return nil, err
	}

	allowed := []string{}
	if raw, ok := config["allowed_domains"]; ok {
		switch val := raw.(type) {
		case []interface{}:
			for _, item := range val {
				s := fmt.Sprint(item)
				rendered, err := dsl.ProcessTemplate(s, ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to process allowed_domain %q: %w", s, err)
				}
				allowed = append(allowed, rendered)
			}
		case []string:
			for _, item := range val {
				rendered, err := dsl.ProcessTemplate(item, ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to process allowed_domain %q: %w", item, err)
				}
				allowed = append(allowed, rendered)
			}
		default:
			return nil, fmt.Errorf("allowed_domains must be an array, got %T", raw)
		}
	}

	maxSteps := 10
	if raw, ok := config["max_steps"]; ok {
		switch val := raw.(type) {
		case float64:
			maxSteps = int(val)
		case int:
			maxSteps = val
		case string:
			parsed, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid max_steps %q: %w", val, err)
			}
			maxSteps = parsed
		default:
			return nil, fmt.Errorf("invalid max_steps: %v", raw)
		}
	}

	useVision := false
	if raw, ok := config["use_vision"]; ok {
		switch val := raw.(type) {
		case bool:
			useVision = val
		case string:
			useVision = strings.EqualFold(val, "true")
		default:
			return nil, fmt.Errorf("invalid use_vision value: %v", raw)
		}
	}

	var temperature *float64
	if raw, ok := config["temperature"]; ok {
		switch val := raw.(type) {
		case float64:
			temperature = &val
		case int:
			f := float64(val)
			temperature = &f
		case string:
			parsed, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid temperature %q: %w", val, err)
			}
			temperature = &parsed
		default:
			return nil, fmt.Errorf("invalid temperature value: %v", raw)
		}
	}

	// Parse LLM config
	llmCfg := LLMConfig{}
	if llmRaw, ok := config["llm"].(map[string]interface{}); ok {
		if provider, ok := llmRaw["provider"].(string); ok {
			llmCfg.Provider = provider
		}
		if model, ok := llmRaw["model"].(string); ok {
			llmCfg.Model = model
		}
		if cfgMap, ok := llmRaw["config"].(map[string]interface{}); ok {
			llmCfg.Config = make(map[string]string)
			for k, v := range cfgMap {
				if strVal, ok := v.(string); ok {
					// Process template variables in config values
					processed, err := dsl.ProcessTemplate(strVal, ctx)
					if err != nil {
						return nil, fmt.Errorf("failed to process template in LLM config %q: %w", k, err)
					}
					llmCfg.Config[k] = processed
				}
			}
		}
	}

	// Parse timeout
	timeout := "5m" // default to 5 minutes (matching legacy browser plugin)
	if raw, ok := config["timeout"]; ok {
		if timeoutStr, ok := raw.(string); ok {
			timeout = timeoutStr
		} else {
			return nil, fmt.Errorf("invalid timeout value: %v (must be a duration string like '5m', '30s')", raw)
		}
	}

	return &Config{
		SessionID:      sessionID,
		Task:           task,
		AllowedDomains: allowed,
		MaxSteps:       maxSteps,
		UseVision:      useVision,
		Temperature:    temperature,
		Timeout:        timeout,
		LLM:            llmCfg,
	}, nil
}
