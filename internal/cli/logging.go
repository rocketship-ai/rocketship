package cli

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is the global logger instance
var Logger *slog.Logger

// InitLogging initializes the logger with the appropriate level based on environment
func InitLogging() {
	level := new(slog.LevelVar)

	// Check for log level in environment
	logLevel := strings.ToUpper(os.Getenv("ROCKETSHIP_LOG"))
	switch logLevel {
	case "DEBUG":
		level.Set(slog.LevelDebug)
	case "WARN":
		level.Set(slog.LevelWarn)
	case "ERROR":
		level.Set(slog.LevelError)
	default:
		// Default to INFO if not set or unrecognized
		level.Set(slog.LevelInfo)
	}

	Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	// Replace the default logger
	slog.SetDefault(Logger)
}
