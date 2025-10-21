package agent

import "time"

// Config represents the configuration for the agent plugin using Claude Agent SDK
type Config struct {
	// Required: prompt to send to the agent
	Prompt string `json:"prompt" yaml:"prompt"`

	// Execution mode: single, continue, resume (default: single)
	Mode ExecutionMode `json:"mode,omitempty" yaml:"mode,omitempty"`

	// Session ID for continue/resume modes (required for resume mode)
	SessionID string `json:"session_id,omitempty" yaml:"session_id,omitempty"`

	// Maximum number of turns (default: unlimited)
	MaxTurns int `json:"max_turns,omitempty" yaml:"max_turns,omitempty"`

	// Timeout for agent execution (default: unlimited)
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// System prompt to prepend to the conversation
	SystemPrompt string `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`

	// Working directory for agent execution (defaults to where rocketship run was executed)
	// NOTE: Permission mode is hardcoded to 'bypassPermissions' because this is a QA testing agent.
	// The agent should use MCP tools to interact with systems, but never ask for user permission
	// or modify files. It's job is to execute test tasks and return pass/fail results.
	Cwd string `json:"cwd,omitempty" yaml:"cwd,omitempty"`

	// MCP servers configuration (key: server name, value: server config)
	MCPServers map[string]MCPServerConfig `json:"mcp_servers,omitempty" yaml:"mcp_servers,omitempty"`

	// Tool permissions: list of allowed tool names, or ["*"] for all tools (default: ["*"])
	AllowedTools []string `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`
}

// MCPServerConfig represents configuration for an MCP server
type MCPServerConfig struct {
	// Server type: stdio or sse
	Type MCPServerType `json:"type" yaml:"type"`

	// For stdio servers: command to execute
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// For stdio servers: command arguments
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// For stdio servers: environment variables
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// For sse servers: HTTP/SSE endpoint URL
	URL string `json:"url,omitempty" yaml:"url,omitempty"`

	// For sse servers: HTTP headers
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// Response represents the response from the agent
type Response struct {
	Success   bool              `json:"ok"`
	Error     string            `json:"error,omitempty"`
	Result    string            `json:"result,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Mode      string            `json:"mode,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Traceback string            `json:"traceback,omitempty"`
}

// ExecutionMode represents the different ways an agent can be executed
type ExecutionMode string

const (
	// ModeSingle is a single prompt execution (no session)
	ModeSingle ExecutionMode = "single"

	// ModeContinue continues the most recent conversation
	ModeContinue ExecutionMode = "continue"

	// ModeResume resumes a specific session
	ModeResume ExecutionMode = "resume"
)

// MCPServerType represents the type of MCP server
type MCPServerType string

const (
	// MCPServerTypeStdio is a subprocess-based MCP server
	MCPServerTypeStdio MCPServerType = "stdio"

	// MCPServerTypeSSE is an HTTP/SSE-based MCP server
	MCPServerTypeSSE MCPServerType = "sse"
)

// ExecutorResult is returned by the Python executor
type ExecutorResult struct {
	Success    bool
	Response   *Response
	Duration   time.Duration
	RawOutput  string
	ExitCode   int
	ErrorTrace string
}
