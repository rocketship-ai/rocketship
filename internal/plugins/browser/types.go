package browser

import (
	"context"
	"time"
)

// BrowserExecutor interface for different execution strategies
type BrowserExecutor interface {
	Execute(ctx context.Context, config *Config) (*BrowserResponse, error)
	ValidateAvailability() error
}

// Config represents the configuration for browser automation
type Config struct {
	Task            string         `json:"task"`
	LLM             LLMConfig      `json:"llm"`
	ExecutorType    string         `json:"executor_type"`
	Timeout         string         `json:"timeout"`
	MaxSteps        int            `json:"max_steps"`
	BrowserType     string         `json:"browser_type"`
	Headless        bool           `json:"headless"`
	UseVision       bool           `json:"use_vision"`
	SessionID       string         `json:"session_id"`
	SaveScreenshots bool           `json:"save_screenshots"`
	AllowedDomains  []string       `json:"allowed_domains"`
	Viewport        ViewportConfig `json:"viewport"`
}

// LLMConfig represents the LLM configuration
type LLMConfig struct {
	Provider string            `json:"provider"`
	Model    string            `json:"model"`
	Config   map[string]string `json:"config"`
}

// ViewportConfig represents the browser viewport configuration
type ViewportConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// BrowserResponse represents the response from browser automation
type BrowserResponse struct {
	Success       bool          `json:"success"`
	Result        string        `json:"result"`
	SessionID     string        `json:"session_id"`
	Steps         []BrowserStep `json:"steps"`
	Screenshots   []string      `json:"screenshots"`
	ExtractedData interface{}   `json:"extracted_data"`
	Error         string        `json:"error,omitempty"`
	Duration      time.Duration `json:"duration"`
}

// BrowserStep represents a single step in browser automation
type BrowserStep struct {
	StepNumber int    `json:"step_number"`
	Action     string `json:"action"`
	Element    string `json:"element,omitempty"`
	Text       string `json:"text,omitempty"`
	Screenshot string `json:"screenshot,omitempty"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Timestamp  string `json:"timestamp"`
}
