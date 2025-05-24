package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rocketship-ai/rocketship/internal/embedded"
)

// ServerConfig holds configuration for server components
type ServerConfig struct {
	LogsDir     string
	TemporalLog *os.File
	WorkerLog   *os.File
	EngineLog   *os.File
}

// NewServerConfig creates a new server configuration
func NewServerConfig() (*ServerConfig, error) {
	// Check if DEBUG logging is enabled
	logLevel := os.Getenv("ROCKETSHIP_LOG")
	useStdout := logLevel == "DEBUG"

	var temporalLog, workerLog, engineLog *os.File
	var logsDir string

	if useStdout {
		// Use stdout for all logs when DEBUG is enabled
		temporalLog = os.Stdout
		workerLog = os.Stdout
		engineLog = os.Stdout
		logsDir = "" // No logs directory needed
	} else {
		// Use log files for non-DEBUG mode
		logsDir = filepath.Join(os.TempDir(), "rocketship-logs")
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create logs directory: %w", err)
		}

		var err error
		temporalLog, err = os.OpenFile(filepath.Join(logsDir, "temporal.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create temporal log file: %w", err)
		}

		workerLog, err = os.OpenFile(filepath.Join(logsDir, "worker.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			_ = temporalLog.Close()
			return nil, fmt.Errorf("failed to create worker log file: %w", err)
		}

		engineLog, err = os.OpenFile(filepath.Join(logsDir, "engine.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			_ = temporalLog.Close()
			_ = workerLog.Close()
			return nil, fmt.Errorf("failed to create engine log file: %w", err)
		}
	}

	return &ServerConfig{
		LogsDir:     logsDir,
		TemporalLog: temporalLog,
		WorkerLog:   workerLog,
		EngineLog:   engineLog,
	}, nil
}

// Cleanup cleans up server resources
func (c *ServerConfig) Cleanup() {
	// Don't close stdout, only close actual log files
	if c.TemporalLog != nil && c.TemporalLog != os.Stdout {
		if err := c.TemporalLog.Close(); err != nil {
			Logger.Debug("failed to close temporal log", "error", err)
		}
	}
	if c.WorkerLog != nil && c.WorkerLog != os.Stdout {
		if err := c.WorkerLog.Close(); err != nil {
			Logger.Debug("failed to close worker log", "error", err)
		}
	}
	if c.EngineLog != nil && c.EngineLog != os.Stdout {
		if err := c.EngineLog.Close(); err != nil {
			Logger.Debug("failed to close engine log", "error", err)
		}
	}

	// Only remove logs directory if it exists (not empty for DEBUG mode)
	if c.LogsDir != "" {
		if err := os.RemoveAll(c.LogsDir); err != nil {
			Logger.Debug("failed to remove logs directory", "error", err)
		}
	}
}

// StartServer starts all server components
func StartServer(config *ServerConfig, pm *processManager) error {
	Logger.Info("starting Rocketship server...")
	// Start Temporal server if not already running
	if !pm.IsComponentRunning(Temporal) {
		Logger.Debug("starting Temporal server")
		temporalCmd := exec.Command("temporal", "server", "start-dev")
		temporalCmd.Stdout = config.TemporalLog
		temporalCmd.Stderr = config.TemporalLog
		if err := pm.Add(temporalCmd, Temporal); err != nil {
			return fmt.Errorf("failed to start temporal server: %w", err)
		}

		// Give Temporal a moment to start
		time.Sleep(5 * time.Second)
	} else {
		Logger.Debug("Temporal server is already running")
	}

	// Set environment variables for worker and engine
	if err := os.Setenv("TEMPORAL_HOST", "localhost:7233"); err != nil {
		return fmt.Errorf("failed to set TEMPORAL_HOST environment variable: %w", err)
	}

	// Start the worker from embedded binary if not already running
	if !pm.IsComponentRunning(Worker) {
		Logger.Debug("starting Rocketship worker")
		env := []string{"TEMPORAL_HOST=localhost:7233"}
		if logLevel := os.Getenv("ROCKETSHIP_LOG"); logLevel != "" {
			env = append(env, "ROCKETSHIP_LOG="+logLevel)
		}
		workerCmd, err := embedded.ExtractAndRun("worker", nil, env)
		if err != nil {
			return fmt.Errorf("failed to start worker: %w", err)
		}
		workerCmd.Stdout = config.WorkerLog
		workerCmd.Stderr = config.WorkerLog
		if err := pm.Add(workerCmd, Worker); err != nil {
			return fmt.Errorf("failed to start worker: %w", err)
		}
	} else {
		Logger.Debug("Rocketship worker is already running")
	}

	// Start the engine from embedded binary if not already running
	if !pm.IsComponentRunning(Engine) {
		Logger.Debug("starting Rocketship engine")
		env := []string{"TEMPORAL_HOST=localhost:7233"}
		if logLevel := os.Getenv("ROCKETSHIP_LOG"); logLevel != "" {
			env = append(env, "ROCKETSHIP_LOG="+logLevel)
		}
		engineCmd, err := embedded.ExtractAndRun("engine", nil, env)
		if err != nil {
			return fmt.Errorf("failed to start engine: %w", err)
		}
		engineCmd.Stdout = config.EngineLog
		engineCmd.Stderr = config.EngineLog
		if err := pm.Add(engineCmd, Engine); err != nil {
			return fmt.Errorf("failed to start engine: %w", err)
		}

		// Wait for engine to be ready
		ctx := context.Background()
		if err := waitForEngine(ctx); err != nil {
			return fmt.Errorf("engine failed to start: %w", err)
		}
	} else {
		Logger.Debug("Rocketship engine is already running")
	}

	return nil
}

// StopServer stops all server components and cleans up resources
func StopServer() error {
	pm := GetProcessManager()
	pm.Cleanup()
	return nil
}

// IsServerRunning checks if any server components are already running
func IsServerRunning() (bool, []ComponentType) {
	pm := GetProcessManager()
	running := []ComponentType{}

	for _, component := range []ComponentType{Temporal, Worker, Engine} {
		if pm.IsComponentRunning(component) {
			running = append(running, component)
		}
	}

	return len(running) > 0, running
}
