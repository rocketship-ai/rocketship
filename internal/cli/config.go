package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the overall CLI configuration
type Config struct {
	DefaultProfile string            `json:"default_profile"`
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

// DefaultConfig returns a new config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		DefaultProfile: "local",
		Profiles: map[string]Profile{
			"local": {
				Name:          "local",
				EngineAddress: "localhost:7700",
				TLS: TLSConfig{
					Enabled: false,
				},
				Environment: make(map[string]string),
			},
		},
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
		config.DefaultProfile = "local"
	}
	
	// Ensure local profile exists
	if _, exists := config.Profiles["local"]; !exists {
		if config.Profiles == nil {
			config.Profiles = make(map[string]Profile)
		}
		config.Profiles["local"] = Profile{
			Name:          "local",
			EngineAddress: "localhost:7700",
			TLS: TLSConfig{
				Enabled: false,
			},
			Environment: make(map[string]string),
		}
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

// GetProfile returns a profile by name
func (c *Config) GetProfile(name string) (*Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}
	
	profile, exists := c.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}
	
	return &profile, nil
}

// AddProfile adds or updates a profile
func (c *Config) AddProfile(profile Profile) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	c.Profiles[profile.Name] = profile
}

// DeleteProfile removes a profile
func (c *Config) DeleteProfile(name string) error {
	if name == "local" {
		return fmt.Errorf("cannot delete the local profile")
	}
	
	if _, exists := c.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}
	
	delete(c.Profiles, name)
	
	// If we deleted the default profile, reset to local
	if c.DefaultProfile == name {
		c.DefaultProfile = "local"
	}
	
	return nil
}

// SetDefaultProfile sets the default profile
func (c *Config) SetDefaultProfile(name string) error {
	if _, exists := c.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}
	
	c.DefaultProfile = name
	return nil
}

// GetActiveProfile returns the currently active profile
func (c *Config) GetActiveProfile() *Profile {
	profile, err := c.GetProfile(c.DefaultProfile)
	if err != nil {
		// Fallback to local if default profile is broken
		profile, _ = c.GetProfile("local")
	}
	return profile
}

// CreateCloudProfile creates the default cloud profile
func (c *Config) CreateCloudProfile() {
	cloudProfile := Profile{
		Name:          "cloud",
		EngineAddress: "app.rocketship.sh:443",
		TLS: TLSConfig{
			Enabled: true,
			Domain:  "app.rocketship.sh",
		},
		Auth: AuthProfile{
			Issuer:   "https://auth.rocketship.sh",
			ClientID: "rocketship-cloud-cli",
		},
		Environment: make(map[string]string),
	}
	
	c.AddProfile(cloudProfile)
}

// GetProfileKeyringKey returns the keyring key for a profile's auth token
func GetProfileKeyringKey(profileName string) string {
	if profileName == "" || profileName == "local" {
		// Backward compatibility - use the original key for local/default
		return "token"
	}
	return fmt.Sprintf("%s:token", profileName)
}