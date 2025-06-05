package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PythonExecutor implements browser automation using Python and browser-use
type PythonExecutor struct{}

// NewPythonExecutor creates a new Python executor
func NewPythonExecutor() *PythonExecutor {
	return &PythonExecutor{}
}

// ValidateAvailability checks if Python and browser-use are available
func (pe *PythonExecutor) ValidateAvailability() error {
	log.Printf("[DEBUG] Validating Python executor availability")

	// Check Python version
	cmd := exec.Command("python3", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("python3 not found: %w. Please install Python 3.11+", err)
	}

	version := strings.TrimSpace(string(output))
	log.Printf("[DEBUG] Found Python version: %s", version)

	// For now, just check that python3 exists - we'll be more strict about version later
	if !strings.Contains(version, "Python 3.") {
		return fmt.Errorf("python 3.x required, found: %s", version)
	}

	// Check if browser-use is installed
	log.Printf("[DEBUG] Checking browser-use installation")
	cmd = exec.Command("python3", "-c", "import browser_use; print('browser-use installed')")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("browser-use not installed. Please run: pip install browser-use")
	}
	log.Printf("[DEBUG] %s", strings.TrimSpace(string(output)))

	// Check if required LLM libraries are available
	log.Printf("[DEBUG] Checking LLM library availability")
	cmd = exec.Command("python3", "-c", "import langchain_openai, langchain_anthropic; print('LLM libraries available')")
	output, err = cmd.Output()
	if err != nil {
		log.Printf("[WARN] Some LLM libraries may not be installed: %v", err)
		// Don't fail here - we'll handle specific providers as needed
	} else {
		log.Printf("[DEBUG] %s", strings.TrimSpace(string(output)))
	}

	log.Printf("[DEBUG] Python executor validation completed successfully")
	return nil
}

// Execute runs browser automation using Python and browser-use
func (pe *PythonExecutor) Execute(ctx context.Context, config *Config) (*BrowserResponse, error) {
	startTime := time.Now()
	log.Printf("[DEBUG] Starting Python executor with task: %s", config.Task)

	// Create temporary directory for this execution
	workDir, err := os.MkdirTemp("", "rocketship-browser-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}
	defer func() {
		log.Printf("[DEBUG] Cleaning up work directory: %s", workDir)
		if err := os.RemoveAll(workDir); err != nil {
			log.Printf("[WARN] Failed to clean up work directory: %v", err)
		}
	}()

	log.Printf("[DEBUG] Created work directory: %s", workDir)

	// Copy Python script to work directory
	scriptPath := filepath.Join(workDir, "browser_automation.py")
	if err := pe.copyPythonScript(scriptPath); err != nil {
		return nil, fmt.Errorf("failed to copy Python script: %w", err)
	}

	log.Printf("[DEBUG] Python script written to: %s", scriptPath)

	// Execute Python script
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), pe.buildEnvironment(config)...)

	// Capture both stdout and stderr for debugging
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	log.Printf("[DEBUG] Python execution completed in %v", duration)

	if err != nil {
		log.Printf("[ERROR] Python execution failed: %v", err)
		log.Printf("[ERROR] Python output: %s", string(output))
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("Python execution failed: %v\nOutput: %s", err, string(output)),
			Duration: duration,
		}, nil
	}

	log.Printf("[DEBUG] Python execution successful, parsing response")
	log.Printf("[DEBUG] Raw Python output: %s", string(output))

	// Parse response from Python script
	var response BrowserResponse
	if err := json.Unmarshal(output, &response); err != nil {
		log.Printf("[ERROR] Failed to parse Python response: %v", err)
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("Failed to parse response: %v\nOutput: %s", err, string(output)),
			Duration: duration,
		}, nil
	}

	response.Duration = duration
	log.Printf("[DEBUG] Successfully parsed browser response: success=%t, steps=%d",
		response.Success, len(response.Steps))

	return &response, nil
}

// copyPythonScript copies the Python automation script to the work directory
func (pe *PythonExecutor) copyPythonScript(scriptPath string) error {
	// Get the directory of the current Go file
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get current file path")
	}
	
	// Path to the Python script relative to this Go file
	sourceScript := filepath.Join(filepath.Dir(currentFile), "browser_automation.py")
	
	// Read the source script
	scriptContent, err := os.ReadFile(sourceScript)
	if err != nil {
		return fmt.Errorf("failed to read Python script from %s: %w", sourceScript, err)
	}
	
	// Write to destination
	return os.WriteFile(scriptPath, scriptContent, 0755)
}

// buildEnvironment builds the environment variables for the Python process
func (pe *PythonExecutor) buildEnvironment(config *Config) []string {
	env := []string{}

	// Add LLM API keys and config
	for key, value := range config.LLM.Config {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
		log.Printf("[DEBUG] Added environment variable: %s=***", key) // Don't log the actual value
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

	// Add any other browser-specific environment variables
	env = append(env, "PYTHONUNBUFFERED=1") // Ensure output is not buffered

	return env
}
