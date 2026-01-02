package playwright

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	return "playwright"
}

const (
	defaultLaunchTimeoutMS = 45000
	waitBufferDuration     = 5 * time.Second
)

type startConfig struct {
	SessionID     string
	Headless      bool
	SlowMoMS      int
	LaunchArgs    []string
	LaunchTimeout int
	WindowWidth   int
	WindowHeight  int
}

type scriptConfig struct {
	SessionID string
	Language  string
	Script    string
	Env       map[string]string
}

type stopConfig struct {
	SessionID string
}

type pythonResult struct {
	Success   bool        `json:"ok"`
	Error     string      `json:"error,omitempty"`
	Result    interface{} `json:"result,omitempty"`
	Traceback string      `json:"traceback,omitempty"`
}

type startResponse struct {
	Success    bool   `json:"ok"`
	WSEndpoint string `json:"wsEndpoint"`
	PID        int    `json:"pid"`
	Error      string `json:"error,omitempty"`
	UserData   string `json:"userDataDir,omitempty"`
	Port       int    `json:"port,omitempty"`
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

	// Extract env secrets from params (for {{ .env.* }} template resolution)
	env := make(map[string]string)
	if envData, ok := params["env"].(map[string]interface{}); ok {
		for k, v := range envData {
			if strVal, ok := v.(string); ok {
				env[k] = strVal
			}
		}
	} else if envData, ok := params["env"].(map[string]string); ok {
		env = envData
	}

	templateContext := dsl.TemplateContext{
		Runtime: state,
		Env:     env,
	}

	role, err := getRole(configData)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}

	switch role {
	case "start":
		cfg, err := parseStartConfig(configData, templateContext)
		if err != nil {
			return nil, err
		}
		logger.Info("Launching Playwright browser server", "session_id", cfg.SessionID)
		result, err = p.handleStart(ctx, cfg)
		if err != nil {
			return nil, err
		}
	case "script":
		cfg, err := parseScriptConfig(configData, templateContext)
		if err != nil {
			return nil, err
		}
		logger.Info("Executing Playwright script", "session_id", cfg.SessionID)
		result, err = p.handleScript(ctx, cfg)
		if err != nil {
			return nil, err
		}
	case "stop":
		cfg, err := parseStopConfig(configData, templateContext)
		if err != nil {
			return nil, err
		}
		logger.Info("Stopping Playwright session", "session_id", cfg.SessionID)
		result, err = p.handleStop(ctx, cfg)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported role: %s", role)
	}

	success := true
	if v, ok := result["success"].(bool); ok {
		success = v
	}

	if success {
		saved := make(map[string]string)
		if err := processSaves(params, result, saved); err != nil {
			return nil, err
		}
		if len(saved) > 0 {
			result["saved"] = saved
		}

		if err := processAssertions(params, result, state, env); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (p *Plugin) handleStart(ctx context.Context, cfg *startConfig) (map[string]interface{}, error) {
	if err := sessionfile.EnsureDir(); err != nil {
		return nil, err
	}

	if _, _, err := sessionfile.Read(ctx, cfg.SessionID); err == nil {
		return nil, fmt.Errorf("session %q already exists; stop it before starting a new browser", cfg.SessionID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to check session state: %w", err)
	}

	runnerPath, cleanup, err := prepareRunnerScript()
	if err != nil {
		return nil, err
	}
	// Keep runner file for the lifetime of the process; cleanup only removes temp dir stub.
	defer cleanup()

	baseDir, err := sessionfile.BaseDir()
	if err != nil {
		return nil, err
	}

	sessionDir := filepath.Join(baseDir, "tmp", "browser_sessions", cfg.SessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	userDataDir := filepath.Join(sessionDir, "profile")
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create user data directory: %w", err)
	}

	launchTimeout := cfg.LaunchTimeout
	if launchTimeout <= 0 {
		launchTimeout = defaultLaunchTimeoutMS
	}

	args := []string{
		runnerPath,
		"start",
		"--headless", fmt.Sprintf("%t", cfg.Headless),
		"--user-data-dir", userDataDir,
		"--launch-timeout", fmt.Sprintf("%d", launchTimeout),
	}

	if cfg.WindowWidth > 0 {
		args = append(args, "--window-width", fmt.Sprintf("%d", cfg.WindowWidth))
	}
	if cfg.WindowHeight > 0 {
		args = append(args, "--window-height", fmt.Sprintf("%d", cfg.WindowHeight))
	}
	for _, arg := range cfg.LaunchArgs {
		args = append(args, fmt.Sprintf("--launch-arg=%s", arg))
	}

	cmd := exec.Command("python3", args...)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	setupProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Playwright runner: %w", err)
	}

	streamLogs(ctx, stderr, cfg.SessionID)

	waitTimeout := time.Duration(launchTimeout)*time.Millisecond + waitBufferDuration
	response, err := readStartResponse(ctx, stdout, waitTimeout)
	if err != nil {
		killProcessGroup(cmd)
		return nil, err
	}

	// Ensure python process exits cleanly
	waitErr := cmd.Wait()

	if !response.Success {
		if waitErr != nil {
			log.Printf("[WARN] playwright start runner exited with error: %v", waitErr)
		}
		return nil, fmt.Errorf("failed to start browser: %s", response.Error)
	}

	if response.PID <= 0 {
		return nil, fmt.Errorf("playwright start runner returned invalid pid: %d", response.PID)
	}
	if response.WSEndpoint == "" {
		return nil, errors.New("playwright start runner did not provide wsEndpoint")
	}
	if waitErr != nil {
		log.Printf("[WARN] playwright start runner exited with warning: %v", waitErr)
	}

	pid := response.PID
	if err := sessionfile.Write(ctx, cfg.SessionID, response.WSEndpoint, pid); err != nil {
		// Best effort terminate launched browser if we fail to persist session metadata
		if killErr := terminateProcessTree(pid); killErr != nil {
			log.Printf("[WARN] failed to terminate browser after session write error: %v", killErr)
		}
		return nil, err
	}

	result := map[string]interface{}{
		"success":    true,
		"session_id": cfg.SessionID,
		"wsEndpoint": response.WSEndpoint,
		"pid":        pid,
	}

	return result, nil
}

func (p *Plugin) handleScript(ctx context.Context, cfg *scriptConfig) (map[string]interface{}, error) {
	logger := getLogger(ctx)
	log.Printf("[DEBUG] handleScript: Reading session file for session_id=%s", cfg.SessionID)
	wsEndpoint, _, err := sessionfile.Read(ctx, cfg.SessionID)
	if err != nil {
		log.Printf("[DEBUG] handleScript: Failed to read session file: %v", err)
		return nil, fmt.Errorf("session %q is not active: %w", cfg.SessionID, err)
	}
	log.Printf("[DEBUG] handleScript: Got wsEndpoint=%s from session file", wsEndpoint)

	if cfg.Language != "" && strings.ToLower(cfg.Language) != "python" {
		return nil, fmt.Errorf("unsupported language %q: only python is supported", cfg.Language)
	}

	runnerPath, cleanup, err := prepareRunnerScript()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tempDir, err := os.MkdirTemp("", "rocketship-playwright-script-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("[WARN] failed to remove temp dir %s: %v", tempDir, err)
		}
	}()

	scriptPath := filepath.Join(tempDir, "user_script.py")
	if err := os.WriteFile(scriptPath, []byte(cfg.Script), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}

	envJSON := ""
	if len(cfg.Env) > 0 {
		bytes, err := json.Marshal(cfg.Env)
		if err != nil {
			return nil, fmt.Errorf("failed to encode env: %w", err)
		}
		envJSON = string(bytes)
	}

	args := []string{
		runnerPath,
		"script",
		"--ws-endpoint", wsEndpoint,
		"--script-file", scriptPath,
	}
	if envJSON != "" {
		args = append(args, "--env-json", envJSON)
	}

	log.Printf("[DEBUG] handleScript: Executing python runner with args: %v", args)

	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	setupProcessGroup(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start python runner: %w", err)
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
	out := strings.TrimSpace(string(outputBytes))

	if out == "" {
		return nil, errors.New("playwright runner returned no output")
	}

	// Try to parse JSON output first (even if waitErr is not nil)
	startIdx := strings.Index(out, "{")
	endIdx := strings.LastIndex(out, "}")
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		// No JSON found - return raw output
		if waitErr != nil {
			return nil, fmt.Errorf("python execution failed: %w\nstdout: %s", waitErr, out)
		}
		return nil, fmt.Errorf("no JSON found in runner output: %s", out)
	}

	jsonPart := out[startIdx : endIdx+1]

	response := pythonResult{}
	if err := json.Unmarshal([]byte(jsonPart), &response); err != nil {
		// JSON parsing failed - return raw output
		if waitErr != nil {
			return nil, fmt.Errorf("python execution failed: %w\nstdout: %s", waitErr, out)
		}
		return nil, fmt.Errorf("failed to parse runner output: %w\nstdout: %s", err, out)
	}

	// JSON parsed successfully - prefer the clean error message
	if !response.Success {
		if response.Traceback != "" {
			logger.Debug("python traceback", "traceback", response.Traceback)
		}
		// If we have a clean error message in the JSON, use it (newlines already unescaped by json.Unmarshal)
		if response.Error != "" {
			return nil, fmt.Errorf("python execution failed: %s", response.Error)
		}
		// Fallback: no error field or empty
		if waitErr != nil {
			return nil, fmt.Errorf("python execution failed: %w\nstdout: %s", waitErr, out)
		}
		return nil, errors.New("python execution failed with no error message")
	}

	result := map[string]interface{}{
		"success": true,
	}
	switch val := response.Result.(type) {
	case map[string]interface{}:
		for k, v := range val {
			result[k] = v
		}
	case nil:
		// no-op
	default:
		result["result"] = val
	}

	return result, nil
}

func (p *Plugin) handleStop(ctx context.Context, cfg *stopConfig) (map[string]interface{}, error) {
	wsEndpoint, pid, err := sessionfile.Read(ctx, cfg.SessionID)
	if err != nil {
		return nil, fmt.Errorf("session %q is not active: %w", cfg.SessionID, err)
	}

	if err := terminateProcessTree(pid); err != nil {
		return nil, fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}

	// terminateProcessTree now waits synchronously for process to exit
	// Safe to remove session file immediately after
	if err := sessionfile.Remove(ctx, cfg.SessionID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success":        true,
		"session_id":     cfg.SessionID,
		"wsEndpoint":     wsEndpoint,
		"terminated":     true,
		"terminated_pid": pid,
	}, nil
}

func getRole(config map[string]interface{}) (string, error) {
	roleVal, ok := config["role"]
	if !ok {
		return "", errors.New("role is required")
	}
	role, ok := roleVal.(string)
	if !ok || role == "" {
		return "", fmt.Errorf("invalid role: %v", roleVal)
	}
	return strings.ToLower(role), nil
}

func parseStartConfig(config map[string]interface{}, ctx dsl.TemplateContext) (*startConfig, error) {
	sessionID, err := templateStringField(config, "session_id", ctx)
	if err != nil {
		return nil, err
	}

	headless := false // Default to non-headless (show browser for local testing)
	if v, ok := config["headless"]; ok {
		switch val := v.(type) {
		case bool:
			headless = val
		case string:
			headless = strings.ToLower(val) == "true"
		default:
			return nil, fmt.Errorf("invalid headless value: %v", v)
		}
	}

	slowMo := 0
	if v, ok := config["slow_mo_ms"]; ok {
		switch val := v.(type) {
		case float64:
			slowMo = int(val)
		case int:
			slowMo = val
		case string:
			parsed, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid slow_mo_ms %q: %w", val, err)
			}
			slowMo = parsed
		default:
			return nil, fmt.Errorf("invalid slow_mo_ms: %v", v)
		}
	}

	launchArgs := []string{}
	if v, ok := config["launch_args"]; ok {
		switch val := v.(type) {
		case []interface{}:
			for _, arg := range val {
				launchArgs = append(launchArgs, fmt.Sprint(arg))
			}
		case []string:
			launchArgs = append(launchArgs, val...)
		default:
			return nil, fmt.Errorf("launch_args must be an array, got %T", v)
		}
	}

	timeout := 0
	if v, ok := config["launch_timeout_ms"]; ok {
		switch val := v.(type) {
		case float64:
			timeout = int(val)
		case int:
			timeout = val
		case string:
			parsed, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid launch_timeout_ms %q: %w", val, err)
			}
			timeout = parsed
		default:
			return nil, fmt.Errorf("invalid launch_timeout_ms: %v", v)
		}
	}

	windowWidth := 1280
	if v, ok := config["window_width"]; ok {
		switch val := v.(type) {
		case float64:
			windowWidth = int(val)
		case int:
			windowWidth = val
		case string:
			parsed, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid window_width %q: %w", val, err)
			}
			windowWidth = parsed
		default:
			return nil, fmt.Errorf("invalid window_width: %v", v)
		}
	}

	windowHeight := 720
	if v, ok := config["window_height"]; ok {
		switch val := v.(type) {
		case float64:
			windowHeight = int(val)
		case int:
			windowHeight = val
		case string:
			parsed, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid window_height %q: %w", val, err)
			}
			windowHeight = parsed
		default:
			return nil, fmt.Errorf("invalid window_height: %v", v)
		}
	}

	return &startConfig{
		SessionID:     sessionID,
		Headless:      headless,
		SlowMoMS:      slowMo,
		LaunchArgs:    launchArgs,
		LaunchTimeout: timeout,
		WindowWidth:   windowWidth,
		WindowHeight:  windowHeight,
	}, nil
}

func parseScriptConfig(config map[string]interface{}, ctx dsl.TemplateContext) (*scriptConfig, error) {
	sessionID, err := templateStringField(config, "session_id", ctx)
	if err != nil {
		return nil, err
	}

	language := "python"
	if v, ok := config["language"].(string); ok && v != "" {
		language = strings.ToLower(v)
	}

	script, err := templateStringField(config, "script", ctx)
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	if raw, ok := config["env"]; ok {
		envMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("env must be an object, got %T", raw)
		}
		for k, v := range envMap {
			env[k] = fmt.Sprint(v)
		}
		// Template values
		for k, v := range env {
			rendered, err := dsl.ProcessTemplate(v, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to process env %s: %w", k, err)
			}
			env[k] = rendered
		}
	}

	return &scriptConfig{
		SessionID: sessionID,
		Language:  language,
		Script:    script,
		Env:       env,
	}, nil
}

func parseStopConfig(config map[string]interface{}, ctx dsl.TemplateContext) (*stopConfig, error) {
	sessionID, err := templateStringField(config, "session_id", ctx)
	if err != nil {
		return nil, err
	}
	return &stopConfig{SessionID: sessionID}, nil
}

func readStartResponse(ctx context.Context, reader io.Reader, timeout time.Duration) (*startResponse, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	type result struct {
		resp *startResponse
		err  error
	}

	ch := make(chan result, 1)

	go func() {
		if scanner.Scan() {
			line := scanner.Text()
			resp := &startResponse{}
			if err := json.Unmarshal([]byte(line), resp); err != nil {
				ch <- result{nil, fmt.Errorf("failed to parse start response: %w. raw: %s", err, line)}
				return
			}
			ch <- result{resp, nil}
			return
		}
		if err := scanner.Err(); err != nil {
			ch <- result{nil, fmt.Errorf("failed to read start response: %w", err)}
		} else {
			ch <- result{nil, errors.New("start runner exited without output")}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.resp, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out waiting for Playwright start response after %s", timeout)
	}
}

func streamLogs(ctx context.Context, reader io.Reader, sessionID string) {
	logger := getLogger(ctx)

	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			logger.Info("playwright runner", "session_id", sessionID, "log", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			logger.Info("playwright runner stderr error", "session_id", sessionID, "error", err.Error())
		}
	}()
}
