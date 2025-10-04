package authbroker

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddr      string
	Issuer          string
	Audience        string
	ClientID        string
	SigningKeyPath  string
	SigningKeyID    string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Scopes          []string
	GitHub          GitHubConfig
	DatabaseURL     string
	RefreshTokenKey []byte
}

type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	DeviceURL    string
	TokenURL     string
	UserURL      string
	EmailsURL    string
	Scopes       []string
}

const (
	defaultListenAddr   = ":8080"
	defaultAccessTTL    = time.Hour
	defaultRefreshTTL   = 30 * 24 * time.Hour
	defaultGitHubDevice = "https://github.com/login/device/code"
	defaultGitHubToken  = "https://github.com/login/oauth/access_token"
	defaultGitHubUser   = "https://api.github.com/user"
	defaultGitHubEmails = "https://api.github.com/user/emails"
)

func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:      getEnvDefault("ROCKETSHIP_BROKER_LISTEN_ADDR", defaultListenAddr),
		Issuer:          strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_ISSUER")),
		Audience:        strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_AUDIENCE")),
		ClientID:        strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_CLIENT_ID")),
		SigningKeyPath:  strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_SIGNING_KEY_FILE")),
		SigningKeyID:    strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_SIGNING_KEY_ID")),
		AccessTokenTTL:  defaultAccessTTL,
		RefreshTokenTTL: defaultRefreshTTL,
		GitHub: GitHubConfig{
			ClientID:     strings.TrimSpace(os.Getenv("ROCKETSHIP_GITHUB_CLIENT_ID")),
			ClientSecret: strings.TrimSpace(os.Getenv("ROCKETSHIP_GITHUB_CLIENT_SECRET")),
			DeviceURL:    getEnvDefault("ROCKETSHIP_GITHUB_DEVICE_URL", defaultGitHubDevice),
			TokenURL:     getEnvDefault("ROCKETSHIP_GITHUB_TOKEN_URL", defaultGitHubToken),
			UserURL:      getEnvDefault("ROCKETSHIP_GITHUB_USER_URL", defaultGitHubUser),
			EmailsURL:    getEnvDefault("ROCKETSHIP_GITHUB_EMAILS_URL", defaultGitHubEmails),
		},
	}

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_DATABASE_URL"))

	if ttlStr := strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_ACCESS_TTL")); ttlStr != "" {
		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ROCKETSHIP_BROKER_ACCESS_TTL: %w", err)
		}
		if ttl <= 0 {
			return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_ACCESS_TTL must be positive")
		}
		cfg.AccessTokenTTL = ttl
	}

	if ttlStr := strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_REFRESH_TTL")); ttlStr != "" {
		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ROCKETSHIP_BROKER_REFRESH_TTL: %w", err)
		}
		if ttl <= 0 {
			return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_REFRESH_TTL must be positive")
		}
		cfg.RefreshTokenTTL = ttl
	}

	cfg.Scopes = defaultScopes(os.Getenv("ROCKETSHIP_BROKER_SCOPES"))
	cfg.GitHub.Scopes = defaultGitHubScopes(os.Getenv("ROCKETSHIP_GITHUB_SCOPES"))

	key := strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_REFRESH_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("ROCKETSHIP_BROKER_STORE_KEY"))
	}
	if key != "" {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return Config{}, fmt.Errorf("failed to decode refresh token key: %w", err)
		}
		if len(decoded) != 32 {
			return Config{}, fmt.Errorf("refresh token key must decode to 32 bytes")
		}
		cfg.RefreshTokenKey = decoded
	}

	if cfg.Issuer == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_ISSUER is required")
	}
	if cfg.Audience == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_AUDIENCE is required")
	}
	if cfg.ClientID == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_CLIENT_ID is required")
	}
	if cfg.SigningKeyPath == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_SIGNING_KEY_FILE is required")
	}
	if cfg.GitHub.ClientID == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_GITHUB_CLIENT_ID is required")
	}
	if cfg.GitHub.ClientSecret == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_GITHUB_CLIENT_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_DATABASE_URL is required")
	}
	if len(cfg.RefreshTokenKey) == 0 {
		return Config{}, fmt.Errorf("ROCKETSHIP_BROKER_REFRESH_KEY is required")
	}

	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		trimmed := strings.TrimSpace(val)
		if trimmed != "" {
			return trimmed
		}
	}
	return def
}

func defaultScopes(input string) []string {
	scopes := strings.TrimSpace(input)
	if scopes == "" {
		return []string{"openid", "profile", "email", "offline_access"}
	}
	return splitScopes(scopes)
}

func defaultGitHubScopes(input string) []string {
	scopes := strings.TrimSpace(input)
	if scopes == "" {
		return []string{"read:user", "user:email"}
	}
	return splitScopes(scopes)
}

func splitScopes(scopes string) []string {
	fields := strings.Fields(scopes)
	result := make([]string, 0, len(fields))
	for _, s := range fields {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
