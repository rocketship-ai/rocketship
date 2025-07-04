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
	"syscall"
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
	
	// Set process group ID so we can kill the entire process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	log.Printf("[DEBUG] Starting Python process with PID tracking enabled")

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Ensure the process and its children are killed if context is cancelled
	go func() {
		log.Printf("[DEBUG] Setting up context cancellation handler")
		<-ctx.Done()
		log.Printf("[DEBUG] Context cancelled! Reason: %v", ctx.Err())
		if cmd.Process != nil {
			log.Printf("[DEBUG] Python process PID: %d", cmd.Process.Pid)
			log.Printf("[DEBUG] Starting process group termination")
			// Kill the entire process group to ensure child processes (browsers) are also killed
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err != nil {
				log.Printf("[DEBUG] Failed to get process group ID for PID %d: %v", cmd.Process.Pid, err)
				// Fallback to killing just the process
				log.Printf("[DEBUG] Falling back to killing just the Python process")
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("[DEBUG] Failed to kill Python process: %v", err)
				} else {
					log.Printf("[DEBUG] Successfully killed Python process")
				}
				return
			}
			
			log.Printf("[DEBUG] Process group ID: %d", pgid)
			
			// Send SIGTERM to the process group first (graceful)
			log.Printf("[DEBUG] Sending SIGTERM to process group -%d", pgid)
			if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
				log.Printf("[DEBUG] Failed to send SIGTERM to process group -%d: %v", pgid, err)
			} else {
				log.Printf("[DEBUG] Successfully sent SIGTERM to process group -%d", pgid)
			}
			
			// Wait a bit, then send SIGKILL if needed
			go func() {
				log.Printf("[DEBUG] Waiting 2 seconds before sending SIGKILL")
				time.Sleep(2 * time.Second)
				log.Printf("[DEBUG] Sending SIGKILL to process group -%d", pgid)
				if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
					log.Printf("[DEBUG] Failed to send SIGKILL to process group -%d: %v", pgid, err)
				} else {
					log.Printf("[DEBUG] Successfully sent SIGKILL to process group -%d", pgid)
				}
			}()
		} else {
			log.Printf("[DEBUG] No Python process to kill (cmd.Process is nil)")
		}
	}()
	
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start Python process: %w", err)
	}
	
	log.Printf("[DEBUG] Python process started with PID: %d", cmd.Process.Pid)
	
	err = cmd.Wait()
	duration := time.Since(startTime)

	log.Printf("[DEBUG] Python execution completed in %v, exit status: %v", duration, err)

	// Always log stderr for debugging (contains debug info)
	if stderr.Len() > 0 {
		log.Printf("[DEBUG] Python stderr: %s", stderr.String())
	}

	if err != nil {
		log.Printf("[ERROR] Python execution failed: %v", err)
		log.Printf("[ERROR] Python stdout: %s", stdout.String())
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("Python execution failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String()),
			Duration: duration,
		}, nil
	}

	log.Printf("[DEBUG] Python execution successful, parsing response")
	log.Printf("[DEBUG] Python stdout (JSON): %s", stdout.String())

	// Parse response from Python script (stdout contains JSON)
	// Extract the JSON from stdout - it should be the last valid JSON object
	stdoutStr := stdout.String()
	
	// Find the last occurrence of a JSON object starting with {
	lastBraceIndex := strings.LastIndex(stdoutStr, "{")
	if lastBraceIndex == -1 {
		log.Printf("[ERROR] No JSON found in Python response")
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("No JSON found in response\nStdout: %s\nStderr: %s", stdoutStr, stderr.String()),
			Duration: duration,
		}, nil
	}
	
	jsonStr := stdoutStr[lastBraceIndex:]
	log.Printf("[DEBUG] Extracted JSON string: %s", jsonStr)
	
	var response BrowserResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		log.Printf("[ERROR] Failed to parse Python response: %v", err)
		log.Printf("[ERROR] JSON string was: %s", jsonStr)
		return &BrowserResponse{
			Success:  false,
			Error:    fmt.Sprintf("Failed to parse response: %v\nStdout: %s\nStderr: %s", err, stdoutStr, stderr.String()),
			Duration: duration,
		}, nil
	}
	
	log.Printf("[DEBUG] Parsed response: success=%t, result=%s", response.Success, response.Result)

	response.Duration = duration
	log.Printf("[DEBUG] Successfully parsed browser response: success=%t, steps=%d",
		response.Success, len(response.Steps))

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

	// Add viewport settings
	env = append(env, fmt.Sprintf("ROCKETSHIP_VIEWPORT_WIDTH=%d", config.Viewport.Width))
	env = append(env, fmt.Sprintf("ROCKETSHIP_VIEWPORT_HEIGHT=%d", config.Viewport.Height))

	// Add any other browser-specific environment variables
	env = append(env, "PYTHONUNBUFFERED=1") // Ensure output is not buffered

	return env
}
