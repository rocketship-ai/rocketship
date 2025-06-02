package executors

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/plugins/script/runtime"
)

// ShellExecutor executes shell scripts using bash/sh
type ShellExecutor struct{}

// NewShellExecutor creates a new shell executor
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

// Language returns the language identifier
func (s *ShellExecutor) Language() string {
	return "shell"
}

// ValidateScript performs basic validation on the shell script
func (s *ShellExecutor) ValidateScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("shell script cannot be empty")
	}
	return nil
}

// Execute runs the shell script in the current working directory
func (s *ShellExecutor) Execute(ctx context.Context, script string, rtCtx *runtime.Context) error {
	startTime := time.Now()
	
	// Process template variables in the script
	processedScript, err := s.processVariables(script, rtCtx)
	if err != nil {
		return fmt.Errorf("failed to process variables: %w", err)
	}
	
	// Create the command - try bash first, fallback to sh
	var cmd *exec.Cmd
	if s.commandExists("bash") {
		cmd = exec.CommandContext(ctx, "bash", "-c", processedScript)
	} else if s.commandExists("sh") {
		cmd = exec.CommandContext(ctx, "sh", "-c", processedScript)
	} else {
		// Debug: Try absolute paths
		if s.commandExistsAbsolute("/bin/bash") {
			cmd = exec.CommandContext(ctx, "/bin/bash", "-c", processedScript)
		} else if s.commandExistsAbsolute("/bin/sh") {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", processedScript)
		} else {
			return fmt.Errorf("neither bash nor sh is available on this system")
		}
	}
	
	// Set up environment with runtime state and current environment
	cmd.Env = s.buildEnvironment(rtCtx)
	
	// Set working directory to current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	cmd.Dir = wd
	
	// Capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start shell command: %w", err)
	}
	
	// Read output
	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return fmt.Errorf("failed to read stdout: %w", err)
	}
	
	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return fmt.Errorf("failed to read stderr: %w", err)
	}
	
	// Wait for completion
	err = cmd.Wait()
	duration := time.Since(startTime)
	
	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return fmt.Errorf("command execution failed: %w", err)
		}
	}
	
	// Save results to runtime context
	rtCtx.Save("exit_code", fmt.Sprintf("%d", exitCode))
	rtCtx.Save("stdout", string(stdoutBytes))
	rtCtx.Save("stderr", string(stderrBytes))
	rtCtx.Save("duration", duration.String())
	
	// If exit code is non-zero, return an error with details
	if exitCode != 0 {
		return fmt.Errorf("shell command failed with exit code %d. stderr: %s", exitCode, string(stderrBytes))
	}
	
	return nil
}

// processVariables replaces template variables in the script
func (s *ShellExecutor) processVariables(script string, rtCtx *runtime.Context) (string, error) {
	processed := script
	
	// Replace state variables: {{ variable_name }}
	for key, value := range rtCtx.State {
		placeholder := fmt.Sprintf("{{ %s }}", key)
		processed = strings.ReplaceAll(processed, placeholder, value)
	}
	
	// Replace config variables: {{ .vars.variable_name }}
	for key, value := range rtCtx.Vars {
		placeholder := fmt.Sprintf("{{ .vars.%s }}", key)
		processed = strings.ReplaceAll(processed, placeholder, fmt.Sprintf("%v", value))
	}
	
	return processed, nil
}

// buildEnvironment creates the environment for the shell command
func (s *ShellExecutor) buildEnvironment(rtCtx *runtime.Context) []string {
	// Start with current environment
	env := os.Environ()
	
	// Add runtime state as environment variables with ROCKETSHIP_ prefix
	for key, value := range rtCtx.State {
		envVar := fmt.Sprintf("ROCKETSHIP_%s=%s", strings.ToUpper(key), value)
		env = append(env, envVar)
	}
	
	// Add config variables as environment variables with ROCKETSHIP_VAR_ prefix
	for key, value := range rtCtx.Vars {
		envVar := fmt.Sprintf("ROCKETSHIP_VAR_%s=%v", strings.ToUpper(key), value)
		env = append(env, envVar)
	}
	
	return env
}

// commandExists checks if a command is available in the system PATH
func (s *ShellExecutor) commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// commandExistsAbsolute checks if a command exists at an absolute path
func (s *ShellExecutor) commandExistsAbsolute(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

