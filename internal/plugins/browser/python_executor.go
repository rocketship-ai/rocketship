package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

	// Write Python script
	scriptPath := filepath.Join(workDir, "browser_automation.py")
	if err := pe.writePythonScript(scriptPath, config); err != nil {
		return nil, fmt.Errorf("failed to write Python script: %w", err)
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

// writePythonScript generates the Python script for browser automation
func (pe *PythonExecutor) writePythonScript(scriptPath string, config *Config) error {
	log.Printf("[DEBUG] Writing Python script for LLM provider: %s, model: %s",
		config.LLM.Provider, config.LLM.Model)

	// Generate LLM initialization code based on provider
	llmCode := pe.generateLLMCode(config.LLM)

	// Generate browser config
	browserConfigCode := pe.generateBrowserConfigCode(config)

	// Main script template
	script := fmt.Sprintf(`#!/usr/bin/env python3
import asyncio
import json
import sys
import os
import traceback
from datetime import datetime

# Import browser-use
try:
    from browser_use import Agent
    print("Successfully imported browser_use", file=sys.stderr)
except ImportError as e:
    print(f"Failed to import browser_use: {e}", file=sys.stderr)
    sys.exit(1)

# Import LLM provider
%s

async def main():
    try:
        print("Starting browser automation...", file=sys.stderr)
        
        # Initialize LLM
        %s
        print(f"LLM initialized successfully", file=sys.stderr)
        
        # Create browser agent
        agent = Agent(
            task="""%s""",
            llm=llm,
            %s
        )
        print("Agent created successfully", file=sys.stderr)
        
        # Execute the task
        print("Starting task execution...", file=sys.stderr)
        result = await agent.run()
        print("Task execution completed", file=sys.stderr)
        
        # Build response
        response = {
            "success": True,
            "result": str(result) if result else "Task completed successfully",
            "session_id": "",
            "steps": [],
            "screenshots": [],
            "extracted_data": {}
        }
        
        # Try to extract any data that was found
        if hasattr(result, 'extracted_content') and result.extracted_content:
            response["extracted_data"] = result.extracted_content
            response["result"] = str(result.extracted_content)
        
        print(json.dumps(response))
        
    except Exception as e:
        print(f"Error during execution: {e}", file=sys.stderr)
        print(f"Traceback: {traceback.format_exc()}", file=sys.stderr)
        
        error_response = {
            "success": False,
            "error": str(e),
            "result": "",
            "session_id": "",
            "steps": [],
            "screenshots": [],
            "extracted_data": {}
        }
        print(json.dumps(error_response))
        sys.exit(1)

if __name__ == "__main__":
    print("Python script starting...", file=sys.stderr)
    asyncio.run(main())
`,
		pe.generateImports(config.LLM),
		llmCode,
		config.Task,
		browserConfigCode)

	return os.WriteFile(scriptPath, []byte(script), 0755)
}

// generateImports generates the import statements for the LLM provider
func (pe *PythonExecutor) generateImports(llm LLMConfig) string {
	switch llm.Provider {
	case "openai":
		return `try:
    from langchain_openai import ChatOpenAI
    print("OpenAI imports successful", file=sys.stderr)
except ImportError as e:
    print(f"Failed to import OpenAI: {e}", file=sys.stderr)
    print("Please install: pip install langchain-openai", file=sys.stderr)
    sys.exit(1)`
	case "anthropic":
		return `try:
    from langchain_anthropic import ChatAnthropic
    print("Anthropic imports successful", file=sys.stderr)
except ImportError as e:
    print(f"Failed to import Anthropic: {e}", file=sys.stderr)
    print("Please install: pip install langchain-anthropic", file=sys.stderr)
    sys.exit(1)`
	default:
		return `# Unsupported LLM provider
print(f"Unsupported LLM provider: %s", file=sys.stderr)
sys.exit(1)` + llm.Provider
	}
}

// generateLLMCode generates the LLM initialization code
func (pe *PythonExecutor) generateLLMCode(llm LLMConfig) string {
	switch llm.Provider {
	case "openai":
		return fmt.Sprintf(`llm = ChatOpenAI(model="%s")`, llm.Model)
	case "anthropic":
		return fmt.Sprintf(`llm = ChatAnthropic(model="%s")`, llm.Model)
	default:
		return fmt.Sprintf(`raise ValueError("Unsupported LLM provider: %s")`, llm.Provider)
	}
}

// generateBrowserConfigCode generates the browser configuration code
func (pe *PythonExecutor) generateBrowserConfigCode(config *Config) string {
	configParts := []string{}

	if config.Headless {
		configParts = append(configParts, "headless=True")
	} else {
		configParts = append(configParts, "headless=False")
	}

	if config.UseVision {
		configParts = append(configParts, "use_vision=True")
	}

	if config.MaxSteps > 0 {
		configParts = append(configParts, fmt.Sprintf("max_steps=%d", config.MaxSteps))
	}

	// For now, keep it simple and don't include complex browser config
	// We can add more options later as needed

	return strings.Join(configParts, ",\n            ")
}

// buildEnvironment builds the environment variables for the Python process
func (pe *PythonExecutor) buildEnvironment(config *Config) []string {
	env := []string{}

	// Add LLM API keys and config
	for key, value := range config.LLM.Config {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
		log.Printf("[DEBUG] Added environment variable: %s=***", key) // Don't log the actual value
	}

	// Add any other browser-specific environment variables
	env = append(env, "PYTHONUNBUFFERED=1") // Ensure output is not buffered

	return env
}

