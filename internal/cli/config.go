package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the overall CLI configuration
type Config struct {
	DefaultProfile string             `json:"default_profile"`
	Profiles       map[string]Profile `json:"profiles"`
}

// Profile represents a connection profile
type Profile struct {
	Name          string            `json:"name"`
	EngineAddress string            `json:"engine_address"`
	TLS           TLSConfig         `json:"tls,omitempty"`
	Auth          AuthProfile       `json:"auth,omitempty"`
	TeamContext   *TeamContext      `json:"team_context,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
}

// TLSConfig represents TLS configuration for a profile
type TLSConfig struct {
	Enabled bool   `json:"enabled"`
	Domain  string `json:"domain,omitempty"`
}

// AuthProfile represents authentication configuration for a profile
type AuthProfile struct {
	Issuer       string `json:"issuer,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	AdminEmails  string `json:"admin_emails,omitempty"`
}

// TeamContext represents team-specific context for a profile
type TeamContext struct {
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
}

// GetDefaultProfile returns the built-in default profile for app.rocketship.sh
func GetDefaultProfile() Profile {
	return Profile{
		Name: "default",
		// Default cloud-hosted CLI endpoint
		EngineAddress: "cli.rocketship.sh:443",
		TLS: TLSConfig{
			Enabled: true,
			Domain:  "cli.rocketship.sh",
		},
		Environment: make(map[string]string),
	}
}

// DefaultConfig returns a new config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		DefaultProfile: "default",
		Profiles:       map[string]Profile{}, // User profiles only, default is built-in
	}
}

// ConfigPath returns the path to the configuration file
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".rocketship")

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "config.json"), nil
}

// LoadConfig loads the configuration from disk
func LoadConfig() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Ensure we have a default profile if none is set
	if config.DefaultProfile == "" {
		config.DefaultProfile = "default"
	}

	// Ensure profiles map exists
	if config.Profiles == nil {
		config.Profiles = make(map[string]Profile)
	}

	return &config, nil
}

// SaveConfig saves the configuration to disk
func (c *Config) SaveConfig() error {
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetProfile returns a profile by name, including the built-in default profile
func (c *Config) GetProfile(name string) (Profile, bool) {
	if name == "default" {
		return GetDefaultProfile(), true
	}

	profile, exists := c.Profiles[name]
	return profile, exists
}

// GetCurrentProfile returns the currently active profile
func (c *Config) GetCurrentProfile() Profile {
	profile, exists := c.GetProfile(c.DefaultProfile)
	if !exists {
		// Fallback to built-in default if configured profile doesn't exist
		return GetDefaultProfile()
	}
	return profile
}

// ListAllProfiles returns all profiles including the built-in default
func (c *Config) ListAllProfiles() map[string]Profile {
	allProfiles := make(map[string]Profile)

	// Add built-in default profile
	allProfiles["default"] = GetDefaultProfile()

	// Add user-defined profiles
	for name, profile := range c.Profiles {
		allProfiles[name] = profile
	}

	return allProfiles
}

// AddProfile adds or updates a user-defined profile
func (c *Config) AddProfile(profile Profile) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	c.Profiles[profile.Name] = profile
}

// DeleteProfile removes a user-defined profile
func (c *Config) DeleteProfile(name string) error {
	if name == "default" {
		return fmt.Errorf("cannot delete the built-in default profile")
	}

	if _, exists := c.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(c.Profiles, name)

	// Reset active profile if we deleted it
	if c.DefaultProfile == name {
		c.DefaultProfile = "default"
	}

	return nil
}

// SetDefaultProfile sets the active profile
func (c *Config) SetDefaultProfile(name string) error {
	if name == "default" {
		c.DefaultProfile = name
		return nil
	}

	if _, exists := c.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	c.DefaultProfile = name
	return nil
}
