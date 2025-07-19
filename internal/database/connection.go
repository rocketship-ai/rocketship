package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config represents database configuration
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
	MaxConns int
	MinConns int
}

// DefaultConfig returns a default database configuration
func DefaultConfig() *Config {
	return &Config{
		Host:     getEnvOrDefault("ROCKETSHIP_DB_HOST", "localhost"),
		Port:     getEnvIntOrDefault("ROCKETSHIP_DB_PORT", 5432),
		Database: getEnvOrDefault("ROCKETSHIP_DB_NAME", "rocketship"),
		Username: getEnvOrDefault("ROCKETSHIP_DB_USER", "rocketship"),
		Password: getEnvOrDefault("ROCKETSHIP_DB_PASSWORD", "rocketship"),
		SSLMode:  getEnvOrDefault("ROCKETSHIP_DB_SSLMODE", "disable"),
		MaxConns: getEnvIntOrDefault("ROCKETSHIP_DB_MAX_CONNS", 25),
		MinConns: getEnvIntOrDefault("ROCKETSHIP_DB_MIN_CONNS", 5),
	}
}

// Connect creates a new database connection pool
func Connect(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set pool configuration
	poolConfig.MaxConns = int32(config.MaxConns)
	poolConfig.MinConns = int32(config.MinConns)
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = time.Minute * 30

	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// IsAvailable checks if database is available
func IsAvailable(ctx context.Context, config *Config) bool {
	pool, err := Connect(ctx, config)
	if err != nil {
		return false
	}
	defer pool.Close()
	return true
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns environment variable value as int or default
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// parseInt parses string to int
func parseInt(s string) (int, error) {
	var result int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}