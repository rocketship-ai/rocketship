package agent

import (
	"encoding/json"
	"testing"

	"github.com/rocketship-ai/rocketship/internal/dsl"
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
				"prompt": "Test prompt",
			},
			expectError: false,
			expected: &Config{
				Prompt: "Test prompt",
			},
		},
		{
			name: "valid full config with mode and session",
			configData: map[string]interface{}{
				"prompt":        "Test prompt",
				"mode":          "continue",
				"session_id":    "session-123",
				"max_turns":     3,
				"timeout":       "60s",
				"system_prompt": "You are a helpful assistant",
				"cwd":           "/tmp",
			},
			expectError: false,
			expected: &Config{
				Prompt:       "Test prompt",
				Mode:         ModeContinue,
				SessionID:    "session-123",
				MaxTurns:     3,
				Timeout:      "60s",
				SystemPrompt: "You are a helpful assistant",
				Cwd:          "/tmp",
			},
		},
		{
			name: "config with allowed_tools wildcard",
			configData: map[string]interface{}{
				"prompt":        "Test prompt",
				"allowed_tools": []interface{}{"*"},
			},
			expectError: false,
			expected: &Config{
				Prompt:       "Test prompt",
				AllowedTools: []string{"*"},
			},
		},
		{
			name: "config with specific allowed_tools",
			configData: map[string]interface{}{
				"prompt": "Test prompt",
				"allowed_tools": []interface{}{
					"mcp__playwright__browser_navigate",
					"mcp__playwright__browser_click",
				},
			},
			expectError: false,
			expected: &Config{
				Prompt: "Test prompt",
				AllowedTools: []string{
					"mcp__playwright__browser_navigate",
					"mcp__playwright__browser_click",
				},
			},
		},
		{
			name: "config with browser capability",
			configData: map[string]interface{}{
				"prompt":       "Test prompt",
				"capabilities": []interface{}{"browser"},
			},
			expectError: false,
			expected: &Config{
				Prompt:       "Test prompt",
				Capabilities: []string{"browser"},
				MCPServers: map[string]MCPServerConfig{
					"playwright": {
						Type:    MCPServerTypeStdio,
						Command: "npx",
						Args:    []string{"@playwright/mcp@0.0.43"},
					},
				},
			},
		},
		{
			name: "config with unknown capability should error",
			configData: map[string]interface{}{
				"prompt":       "Test prompt",
				"capabilities": []interface{}{"invalid"},
			},
			expectError: true,
			expected:    nil,
		},
		{
			name: "config with old mcp_servers field should error",
			configData: map[string]interface{}{
				"prompt": "Test prompt",
				"mcp_servers": map[string]interface{}{
					"playwright": map[string]interface{}{
						"type":    "stdio",
						"command": "npx",
					},
				},
			},
			expectError: true,
			expected:    nil,
		},
		{
			name: "max_turns as float64",
			configData: map[string]interface{}{
				"prompt":    "Test prompt",
				"max_turns": 5.0, // JSON numbers are parsed as float64
			},
			expectError: false,
			expected: &Config{
				Prompt:   "Test prompt",
				MaxTurns: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateContext := dsl.TemplateContext{
				Runtime: make(map[string]interface{}),
			}

			config, err := parseConfig(tt.configData, templateContext)

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
				if config.Prompt != tt.expected.Prompt {
					t.Errorf("Expected prompt %q, got %q", tt.expected.Prompt, config.Prompt)
				}

				// Check optional fields if they were set
				if tt.expected.Mode != "" && config.Mode != tt.expected.Mode {
					t.Errorf("Expected mode %q, got %q", tt.expected.Mode, config.Mode)
				}

				if tt.expected.SessionID != "" && config.SessionID != tt.expected.SessionID {
					t.Errorf("Expected session_id %q, got %q", tt.expected.SessionID, config.SessionID)
				}

				if tt.expected.MaxTurns != 0 && config.MaxTurns != tt.expected.MaxTurns {
					t.Errorf("Expected max_turns %d, got %d", tt.expected.MaxTurns, config.MaxTurns)
				}

				if tt.expected.Timeout != "" && config.Timeout != tt.expected.Timeout {
					t.Errorf("Expected timeout %q, got %q", tt.expected.Timeout, config.Timeout)
				}

				if tt.expected.SystemPrompt != "" && config.SystemPrompt != tt.expected.SystemPrompt {
					t.Errorf("Expected system_prompt %q, got %q", tt.expected.SystemPrompt, config.SystemPrompt)
				}

				if tt.expected.Cwd != "" && config.Cwd != tt.expected.Cwd {
					t.Errorf("Expected cwd %q, got %q", tt.expected.Cwd, config.Cwd)
				}

				if len(tt.expected.AllowedTools) > 0 {
					if len(config.AllowedTools) != len(tt.expected.AllowedTools) {
						t.Errorf("Expected %d allowed_tools, got %d", len(tt.expected.AllowedTools), len(config.AllowedTools))
					} else {
						for i, tool := range tt.expected.AllowedTools {
							if config.AllowedTools[i] != tool {
								t.Errorf("Expected allowed_tool[%d] %q, got %q", i, tool, config.AllowedTools[i])
							}
						}
					}
				}

				if len(tt.expected.MCPServers) > 0 {
					if len(config.MCPServers) != len(tt.expected.MCPServers) {
						t.Errorf("Expected %d mcp_servers, got %d", len(tt.expected.MCPServers), len(config.MCPServers))
					}
					for name, expectedServer := range tt.expected.MCPServers {
						actualServer, ok := config.MCPServers[name]
						if !ok {
							t.Errorf("Expected mcp_server %q not found", name)
							continue
						}
						if actualServer.Type != expectedServer.Type {
							t.Errorf("MCP server %q: expected type %q, got %q", name, expectedServer.Type, actualServer.Type)
						}
						if actualServer.Command != expectedServer.Command {
							t.Errorf("MCP server %q: expected command %q, got %q", name, expectedServer.Command, actualServer.Command)
						}
						if actualServer.URL != expectedServer.URL {
							t.Errorf("MCP server %q: expected url %q, got %q", name, expectedServer.URL, actualServer.URL)
						}
					}
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

func TestMCPServerType_Constants(t *testing.T) {
	if MCPServerTypeStdio != "stdio" {
		t.Errorf("Expected MCPServerTypeStdio to be 'stdio', got '%s'", MCPServerTypeStdio)
	}

	if MCPServerTypeSSE != "sse" {
		t.Errorf("Expected MCPServerTypeSSE to be 'sse', got '%s'", MCPServerTypeSSE)
	}
}

func TestParseMCPServerConfig_Stdio(t *testing.T) {
	serverData := map[string]interface{}{
		"type":    "stdio",
		"command": "npx",
		"args":    []interface{}{"@playwright/mcp@latest", "--cdp-endpoint", "ws://localhost:9222"},
		"env": map[string]interface{}{
			"DEBUG": "true",
		},
	}

	templateContext := dsl.TemplateContext{
		Runtime: make(map[string]interface{}),
	}

	config, err := parseMCPServerConfig(serverData, templateContext)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config.Type != MCPServerTypeStdio {
		t.Errorf("Expected type stdio, got %s", config.Type)
	}

	if config.Command != "npx" {
		t.Errorf("Expected command npx, got %s", config.Command)
	}

	if len(config.Args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(config.Args))
	}

	if config.Env["DEBUG"] != "true" {
		t.Errorf("Expected env DEBUG=true, got %s", config.Env["DEBUG"])
	}
}

func TestParseMCPServerConfig_SSE(t *testing.T) {
	serverData := map[string]interface{}{
		"type": "sse",
		"url":  "https://example.com/mcp",
		"headers": map[string]interface{}{
			"Authorization": "Bearer token123",
		},
	}

	templateContext := dsl.TemplateContext{
		Runtime: make(map[string]interface{}),
	}

	config, err := parseMCPServerConfig(serverData, templateContext)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config.Type != MCPServerTypeSSE {
		t.Errorf("Expected type sse, got %s", config.Type)
	}

	if config.URL != "https://example.com/mcp" {
		t.Errorf("Expected URL https://example.com/mcp, got %s", config.URL)
	}

	if config.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization header, got %s", config.Headers["Authorization"])
	}
}

func TestParseMCPServerConfig_MissingRequired(t *testing.T) {
	tests := []struct {
		name       string
		serverData map[string]interface{}
	}{
		{
			name: "stdio without command",
			serverData: map[string]interface{}{
				"type": "stdio",
			},
		},
		{
			name: "sse without url",
			serverData: map[string]interface{}{
				"type": "sse",
			},
		},
		{
			name:       "missing type",
			serverData: map[string]interface{}{},
		},
	}

	templateContext := dsl.TemplateContext{
		Runtime: make(map[string]interface{}),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMCPServerConfig(tt.serverData, templateContext)
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestConfigJSONMarshaling verifies that MCPServers field is properly serialized to JSON
func TestConfigJSONMarshaling(t *testing.T) {
	cfg := &Config{
		Prompt:       "test prompt",
		Capabilities: []string{"browser"},
		MCPServers: map[string]MCPServerConfig{
			"playwright": {
				Type:    MCPServerTypeStdio,
				Command: "npx",
				Args:    []string{"@playwright/mcp@0.0.43"},
			},
		},
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Unmarshal to verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify mcp_servers is present
	mcpServers, ok := result["mcp_servers"]
	if !ok {
		t.Fatalf("mcp_servers field is missing from JSON output. Got: %v", result)
	}

	// Verify structure of mcp_servers
	servers, ok := mcpServers.(map[string]interface{})
	if !ok {
		t.Fatalf("mcp_servers is not a map. Got type: %T", mcpServers)
	}

	playwright, ok := servers["playwright"]
	if !ok {
		t.Fatalf("playwright server is missing from mcp_servers")
	}

	playwrightMap, ok := playwright.(map[string]interface{})
	if !ok {
		t.Fatalf("playwright config is not a map. Got type: %T", playwright)
	}

	if playwrightMap["type"] != "stdio" {
		t.Errorf("Expected type 'stdio', got '%v'", playwrightMap["type"])
	}

	if playwrightMap["command"] != "npx" {
		t.Errorf("Expected command 'npx', got '%v'", playwrightMap["command"])
	}

	t.Logf("✅ MCPServers successfully serialized to mcp_servers in JSON")
}
