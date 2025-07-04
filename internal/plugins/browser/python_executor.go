package browser

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed browser_automation.py
var embeddedPythonScript []byte

// PythonExecutor implements browser automation using Python and browser-use
type PythonExecutor struct{}

// NewPythonExecutor creates a new Python executor
func NewPythonExecutor() *PythonExecutor {
	return &PythonExecutor{}
}

// ValidateAvailability checks if Python and browser-use are available
func (pe *PythonExecutor) ValidateAvailability() error {

	// Check Python version
	cmd := exec.Command("python3", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("python3 not found: %w. Please install Python 3.11+", err)
	}

	version := strings.TrimSpace(string(output))

	// For now, just check that python3 exists - we'll be more strict about version later
	if !strings.Contains(version, "Python 3.") {
		return fmt.Errorf("python 3.x required, found: %s", version)
	}

	// Check if browser-use is installed
	cmd = exec.Command("python3", "-c", "import browser_use; print('browser-use installed')")
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("browser-use not installed. Please run: pip install browser-use")
	}

	// Check if required LLM libraries are available
	cmd = exec.Command("python3", "-c", "import langchain_openai, langchain_anthropic; print('LLM libraries available')")
	_, err = cmd.Output()
	if err != nil {
		log.Printf("[WARN] Some LLM libraries may not be installed: %v", err)
		// Don't fail here - we'll handle specific providers as needed
	}
	return nil
}

// Execute runs browser automation using Python and browser-use
func (pe *PythonExecutor) Execute(ctx context.Context, config *Config) (*BrowserResponse, error) {
	startTime := time.Now()

	// Create temporary directory for this execution
	workDir, err := os.MkdirTemp("", "rocketship-browser-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			log.Printf("[WARN] Failed to clean up work directory: %v", err)
		}
	}()

	// Copy Python script to work directory
	scriptPath := filepath.Join(workDir, "browser_automation.py")
	if err := pe.copyPythonScript(scriptPath); err != nil {
		return nil, fmt.Errorf("failed to copy Python script: %w", err)
	}

	// Execute Python script
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), pe.buildEnvironment(config)...)
	
	// Set up process group (platform-specific)
	setupProcessGroup(cmd)

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Ensure the process and its children are killed if context is cancelled
	go func() {
		<-ctx.Done()
		killProcessGroup(cmd)
	}()
	
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start Python process: %w", err)
	}
	
	err = cmd.Wait()
	duration := time.Since(startTime)

	// Log stderr if there are errors
	if stderr.Len() > 0 && err != nil {
		log.Printf("[ERROR] Python stderr: %s", stderr.String())
	}

	if err != nil {
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("Python execution failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String()),
			Duration: duration,
		}, nil
	}


	// Parse response from Python script (stdout contains JSON)
	// Extract the JSON from stdout - it should be the last valid JSON object
	stdoutStr := stdout.String()
	
	// Find the last occurrence of a JSON object starting with {
	lastBraceIndex := strings.LastIndex(stdoutStr, "{")
	if lastBraceIndex == -1 {
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("No JSON found in response\nStdout: %s\nStderr: %s", stdoutStr, stderr.String()),
			Duration: duration,
		}, nil
	}
	
	jsonStr := stdoutStr[lastBraceIndex:]
	
	var response BrowserResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("Failed to parse response: %v\nStdout: %s\nStderr: %s", err, stdoutStr, stderr.String()),
			Duration: duration,
		}, nil
	}
	
	response.Duration = duration

	return &response, nil
}

// copyPythonScript copies the Python automation script to the work directory
func (pe *PythonExecutor) copyPythonScript(scriptPath string) error {
	// Write the embedded Python script to destination
	return os.WriteFile(scriptPath, embeddedPythonScript, 0755)
}

// buildEnvironment builds the environment variables for the Python process
func (pe *PythonExecutor) buildEnvironment(config *Config) []string {
	env := []string{}

	// Add LLM API keys and config
	for key, value := range config.LLM.Config {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add Rocketship-specific configuration
	env = append(env, fmt.Sprintf("ROCKETSHIP_TASK=%s", config.Task))
	env = append(env, fmt.Sprintf("ROCKETSHIP_LLM_PROVIDER=%s", config.LLM.Provider))
	env = append(env, fmt.Sprintf("ROCKETSHIP_LLM_MODEL=%s", config.LLM.Model))
	env = append(env, fmt.Sprintf("ROCKETSHIP_HEADLESS=%s", strconv.FormatBool(config.Headless)))
	env = append(env, fmt.Sprintf("ROCKETSHIP_BROWSER_TYPE=%s", config.BrowserType))
	env = append(env, fmt.Sprintf("ROCKETSHIP_USE_VISION=%s", strconv.FormatBool(config.UseVision)))
	env = append(env, fmt.Sprintf("ROCKETSHIP_MAX_STEPS=%d", config.MaxSteps))
	
	// Add allowed domains as comma-separated string
	if len(config.AllowedDomains) > 0 {
		env = append(env, fmt.Sprintf("ROCKETSHIP_ALLOWED_DOMAINS=%s", strings.Join(config.AllowedDomains, ",")))
	}

	// Add viewport settings
	env = append(env, fmt.Sprintf("ROCKETSHIP_VIEWPORT_WIDTH=%d", config.Viewport.Width))
	env = append(env, fmt.Sprintf("ROCKETSHIP_VIEWPORT_HEIGHT=%d", config.Viewport.Height))

	// Add any other browser-specific environment variables
	env = append(env, "PYTHONUNBUFFERED=1") // Ensure output is not buffered

	return env
}
