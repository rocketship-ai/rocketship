package agent

import "time"

// Config represents the configuration for the agent plugin
type Config struct {
	Agent            string `json:"agent" yaml:"agent"`                         // Required: type of agent (claude-code, etc.)
	Prompt           string `json:"prompt" yaml:"prompt"`                       // Required: prompt to send to agent
	Mode             string `json:"mode,omitempty" yaml:"mode,omitempty"`       // single, continue, resume (default: single)
	SessionID        string `json:"session_id,omitempty" yaml:"session_id,omitempty"` // for resume mode
	MaxTurns         int    `json:"max_turns,omitempty" yaml:"max_turns,omitempty"`   // default: 1
	Timeout          string `json:"timeout,omitempty" yaml:"timeout,omitempty"`       // default: 30s
	SystemPrompt     string `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	OutputFormat     string `json:"output_format,omitempty" yaml:"output_format,omitempty"` // text, json, streaming-json (default: json)
	ContinueRecent   bool   `json:"continue_recent,omitempty" yaml:"continue_recent,omitempty"`
	SaveFullResponse bool   `json:"save_full_response,omitempty" yaml:"save_full_response,omitempty"` // default: true
}

// Response represents the response from an agent
type Response struct {
	Content    string            `json:"content"`
	SessionID  string            `json:"session_id,omitempty"`
	Cost       float64           `json:"cost,omitempty"`
	Duration   time.Duration     `json:"duration"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	RawOutput  string            `json:"raw_output"`
	ExitCode   int               `json:"exit_code"`
	Error      string            `json:"error,omitempty"`
}

// ExecutionMode represents the different ways an agent can be executed
type ExecutionMode string

const (
	ModeSingle   ExecutionMode = "single"   // Single prompt execution
	ModeContinue ExecutionMode = "continue" // Continue most recent conversation
	ModeResume   ExecutionMode = "resume"   // Resume specific session
)

// OutputFormat represents the output format options
type OutputFormat string

const (
	FormatText          OutputFormat = "text"
	FormatJSON          OutputFormat = "json"
	FormatStreamingJSON OutputFormat = "streaming-json"
)

// AgentType represents the supported agent types
type AgentType string

const (
	AgentClaudeCode AgentType = "claude-code"
	// Future: AgentCursor, AgentCopilot, etc.
)