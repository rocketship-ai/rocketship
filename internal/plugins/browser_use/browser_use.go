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
}

type runnerResponse struct {
	Success   bool                   `json:"ok"`
	Error     string                 `json:"error,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
	FinalURL  string                 `json:"finalUrl,omitempty"`
	Artifacts map[string]interface{} `json:"artifacts,omitempty"`
}

func (p *Plugin) Activity(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	logger := getLogger(ctx)

	configData, ok := params["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	state := extractState(params)
	templateContext := dsl.TemplateContext{Runtime: state}

	cfg, err := parseConfig(configData, templateContext)
	if err != nil {
		return nil, err
	}

	wsEndpoint, _, err := sessionfile.Read(ctx, cfg.SessionID)
	if err != nil {
		return nil, fmt.Errorf("session %q is not active: %w", cfg.SessionID, err)
	}

	logger.Info("Executing browser-use task", "session_id", cfg.SessionID)

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

	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
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
		<-ctx.Done()
		killProcessGroup(cmd)
	}()

	outputBytes, readErr := io.ReadAll(stdoutPipe)
	waitErr := cmd.Wait()

	if readErr != nil {
		return nil, fmt.Errorf("failed to read runner output: %w", readErr)
	}

	output := strings.TrimSpace(string(outputBytes))

	if waitErr != nil {
		return nil, fmt.Errorf("browser-use execution failed: %w\nstdout: %s", waitErr, output)
	}

	if output == "" {
		return nil, errors.New("browser-use runner returned no output")
	}

	startIdx := strings.Index(output, "{")
	endIdx := strings.LastIndex(output, "}")
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return nil, fmt.Errorf("no JSON found in runner output: %s", output)
	}

	response := runnerResponse{}
	if err := json.Unmarshal([]byte(output[startIdx:endIdx+1]), &response); err != nil {
		return nil, fmt.Errorf("failed to parse runner output: %w\nstdout: %s", err, output)
	}

	if !response.Success {
		return map[string]interface{}{
			"success": false,
			"error":   response.Error,
		}, nil
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

	return &Config{
		SessionID:      sessionID,
		Task:           task,
		AllowedDomains: allowed,
		MaxSteps:       maxSteps,
		UseVision:      useVision,
		Temperature:    temperature,
	}, nil
}
