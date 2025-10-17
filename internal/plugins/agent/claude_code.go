package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCodeExecutor handles execution of Claude Code agent
type ClaudeCodeExecutor struct{}

// ClaudeCodeConfig represents configuration specific to Claude Code
type ClaudeCodeConfig struct {
	Prompt         string
	Mode           ExecutionMode
	SessionID      string
	MaxTurns       int
	SystemPrompt   string
	OutputFormat   OutputFormat
	ContinueRecent bool
}

// ClaudeCodeResponse represents the JSON response from Claude Code
type ClaudeCodeResponse struct {
	Type          string  `json:"type"`
	Subtype       string  `json:"subtype"`
	Result        string  `json:"result"` // This contains the actual Claude response
	SessionID     string  `json:"session_id"`
	CostUSD       float64 `json:"cost_usd"`
	IsError       bool    `json:"is_error"`
	DurationMS    int     `json:"duration_ms"`
	DurationAPIMS int     `json:"duration_api_ms"`
	NumTurns      int     `json:"num_turns"`
}

// NewClaudeCodeExecutor creates a new Claude Code executor
func NewClaudeCodeExecutor() *ClaudeCodeExecutor {
	return &ClaudeCodeExecutor{}
}

// ValidateAvailability checks if Claude Code is available and properly configured
func (e *ClaudeCodeExecutor) ValidateAvailability() error {
	// Check if claude command is available
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude command not found in PATH. Please install Claude Code: npm install -g @anthropic-ai/claude-code")
	}

	// Check if ANTHROPIC_API_KEY is set
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}

	return nil
}

// Execute runs Claude Code with the given configuration
func (e *ClaudeCodeExecutor) Execute(ctx context.Context, config *ClaudeCodeConfig) (*Response, error) {
	startTime := time.Now()

	// Build command arguments
	args := e.buildArgs(config)

	// Create command
	cmd := exec.CommandContext(ctx, "claude", args...)

	// Set up environment
	cmd.Env = os.Environ()

	// Set up stdin for the prompt
	cmd.Stdin = strings.NewReader(config.Prompt)

	// Capture output
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute claude command: %w", err)
		}
	}

	// Parse response based on output format
	response := &Response{
		Duration:  duration,
		RawOutput: string(output),
		ExitCode:  exitCode,
	}

	if exitCode != 0 {
		response.Error = string(output)
		return response, nil
	}

	// Parse output based on format
	switch config.OutputFormat {
	case FormatJSON, FormatStreamingJSON:
		if err := e.parseJSONResponse(string(output), response); err != nil {
			// If JSON parsing fails, treat as text
			response.Content = string(output)
		}
	case FormatText:
		response.Content = string(output)
	default:
		response.Content = string(output)
	}

	return response, nil
}

// buildArgs constructs the command line arguments for Claude Code
func (e *ClaudeCodeExecutor) buildArgs(config *ClaudeCodeConfig) []string {
	args := []string{}

	// Add mode-specific flags
	switch config.Mode {
	case ModeSingle:
		args = append(args, "-p") // print mode for single execution
	case ModeContinue:
		if config.ContinueRecent {
			args = append(args, "--continue")
		} else {
			args = append(args, "-p") // Default to single if continue_recent is false
		}
	case ModeResume:
		if config.SessionID != "" {
			args = append(args, "--resume", config.SessionID)
		} else {
			args = append(args, "-p") // Fallback to single
		}
	}

	// Add max turns if specified and greater than 1
	if config.MaxTurns > 1 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", config.MaxTurns))
	}

	// Add system prompt if specified
	if config.SystemPrompt != "" {
		args = append(args, "--system-prompt", config.SystemPrompt)
	}

	// Add output format
	switch config.OutputFormat {
	case FormatJSON:
		args = append(args, "--output-format", "json")
	case FormatStreamingJSON:
		args = append(args, "--output-format", "streaming-json")
	case FormatText:
		args = append(args, "--output-format", "text")
	}

	return args
}

// parseJSONResponse parses the JSON response from Claude Code
func (e *ClaudeCodeExecutor) parseJSONResponse(output string, response *Response) error {
	// Handle streaming JSON by taking the last complete JSON object
	if strings.Contains(output, "\n") {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				output = lines[i]
				break
			}
		}
	}

	var claudeResp ClaudeCodeResponse
	if err := json.Unmarshal([]byte(output), &claudeResp); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	response.Content = claudeResp.Result
	response.SessionID = claudeResp.SessionID
	response.Cost = claudeResp.CostUSD

	// Convert duration from milliseconds to time.Duration
	if claudeResp.DurationMS > 0 {
		response.Duration = time.Duration(claudeResp.DurationMS) * time.Millisecond
	}

	// Add additional metadata from Claude Code response
	response.Metadata = map[string]string{
		"type":            claudeResp.Type,
		"subtype":         claudeResp.Subtype,
		"num_turns":       fmt.Sprintf("%d", claudeResp.NumTurns),
		"duration_api_ms": fmt.Sprintf("%d", claudeResp.DurationAPIMS),
		"is_error":        fmt.Sprintf("%t", claudeResp.IsError),
	}

	return nil
}
