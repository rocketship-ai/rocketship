package agent

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		configData  map[string]interface{}
		expectError bool
		expected    *Config
	}{
		{
			name: "valid minimal config",
			configData: map[string]interface{}{
				"agent":  "claude-code",
				"prompt": "Test prompt",
			},
			expectError: false,
			expected: &Config{
				Agent:  "claude-code",
				Prompt: "Test prompt",
			},
		},
		{
			name: "valid full config",
			configData: map[string]interface{}{
				"agent":              "claude-code",
				"prompt":             "Test prompt with {{ variable }}",
				"mode":               "continue",
				"session_id":         "session-123",
				"max_turns":          3,
				"timeout":            "60s",
				"system_prompt":      "You are a helpful assistant",
				"output_format":      "json",
				"continue_recent":    true,
				"save_full_response": true,
			},
			expectError: false,
			expected: &Config{
				Agent:            "claude-code",
				Prompt:           "Test prompt with {{ variable }}",
				Mode:             "continue",
				SessionID:        "session-123",
				MaxTurns:         3,
				Timeout:          "60s",
				SystemPrompt:     "You are a helpful assistant",
				OutputFormat:     "json",
				ContinueRecent:   true,
				SaveFullResponse: true,
			},
		},
		{
			name: "max_turns as float64",
			configData: map[string]interface{}{
				"agent":     "claude-code",
				"prompt":    "Test prompt",
				"max_turns": 5.0, // JSON numbers are parsed as float64
			},
			expectError: false,
			expected: &Config{
				Agent:    "claude-code",
				Prompt:   "Test prompt",
				MaxTurns: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			err := parseConfig(tt.configData, config)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tt.expectError {
				// Check required fields
				if config.Agent != tt.expected.Agent {
					t.Errorf("Expected agent %s, got %s", tt.expected.Agent, config.Agent)
				}

				if config.Prompt != tt.expected.Prompt {
					t.Errorf("Expected prompt %s, got %s", tt.expected.Prompt, config.Prompt)
				}

				// Check optional fields if they were set
				if tt.expected.Mode != "" && config.Mode != tt.expected.Mode {
					t.Errorf("Expected mode %s, got %s", tt.expected.Mode, config.Mode)
				}

				if tt.expected.MaxTurns != 0 && config.MaxTurns != tt.expected.MaxTurns {
					t.Errorf("Expected max_turns %d, got %d", tt.expected.MaxTurns, config.MaxTurns)
				}

				if tt.expected.Timeout != "" && config.Timeout != tt.expected.Timeout {
					t.Errorf("Expected timeout %s, got %s", tt.expected.Timeout, config.Timeout)
				}

				if tt.expected.OutputFormat != "" && config.OutputFormat != tt.expected.OutputFormat {
					t.Errorf("Expected output_format %s, got %s", tt.expected.OutputFormat, config.OutputFormat)
				}

				if config.ContinueRecent != tt.expected.ContinueRecent {
					t.Errorf("Expected continue_recent %v, got %v", tt.expected.ContinueRecent, config.ContinueRecent)
				}

				if config.SaveFullResponse != tt.expected.SaveFullResponse {
					t.Errorf("Expected save_full_response %v, got %v", tt.expected.SaveFullResponse, config.SaveFullResponse)
				}
			}
		})
	}
}

func TestAgentPlugin_GetType(t *testing.T) {
	plugin := &AgentPlugin{}
	if plugin.GetType() != "agent" {
		t.Errorf("Expected plugin type 'agent', got '%s'", plugin.GetType())
	}
}

func TestClaudeCodeExecutor_ValidateAvailability(t *testing.T) {
	executor := NewClaudeCodeExecutor()

	// This test will fail if claude is not installed or ANTHROPIC_API_KEY is not set
	// but that's expected behavior for the validation
	err := executor.ValidateAvailability()

	// We can't assert success/failure here since it depends on the environment
	// This test mainly ensures the method doesn't panic
	if err != nil {
		t.Logf("Expected validation failure in test environment: %v", err)
	} else {
		t.Logf("Claude Code is available and properly configured")
	}
}

func TestAgentType_Constants(t *testing.T) {
	if AgentClaudeCode != "claude-code" {
		t.Errorf("Expected AgentClaudeCode to be 'claude-code', got '%s'", AgentClaudeCode)
	}
}

func TestExecutionMode_Constants(t *testing.T) {
	if ModeSingle != "single" {
		t.Errorf("Expected ModeSingle to be 'single', got '%s'", ModeSingle)
	}

	if ModeContinue != "continue" {
		t.Errorf("Expected ModeContinue to be 'continue', got '%s'", ModeContinue)
	}

	if ModeResume != "resume" {
		t.Errorf("Expected ModeResume to be 'resume', got '%s'", ModeResume)
	}
}

func TestOutputFormat_Constants(t *testing.T) {
	if FormatText != "text" {
		t.Errorf("Expected FormatText to be 'text', got '%s'", FormatText)
	}

	if FormatJSON != "json" {
		t.Errorf("Expected FormatJSON to be 'json', got '%s'", FormatJSON)
	}

	if FormatStreamingJSON != "streaming-json" {
		t.Errorf("Expected FormatStreamingJSON to be 'streaming-json', got '%s'", FormatStreamingJSON)
	}
}
