package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/embedded"
	"github.com/spf13/cobra"
)

// NewStartCmd creates a new start command
func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start rocketship the rocketship server",
		Long:  `Start rocketship components like the server or create a new session.`,
	}

	cmd.AddCommand(newStartServerCmd())
	cmd.AddCommand(newStartSessionCmd())

	return cmd
}

func newStartServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the rocketship server",
		Long:  `Start the rocketship server either locally or connect to a remote instance.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			isLocal, err := cmd.Flags().GetBool("local")
			if err != nil {
				return err
			}

			isBackground, err := cmd.Flags().GetBool("background")
			if err != nil {
				return err
			}

			if isLocal {
				if isBackground {
					// Start processes and return immediately
					return setupLocalEnvironmentBackground()
				}
				return setupLocalEnvironment()
			}

			return fmt.Errorf("remote server connection not yet implemented")
		},
	}

	cmd.Flags().Bool("local", false, "Start local development environment")
	cmd.Flags().Bool("background", false, "Start server in background mode")
	return cmd
}

func newStartSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Start a new rocketship session",
		Long:  `Start a new rocketship session by connecting to a rocketship engine.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			engineAddr, err := cmd.Flags().GetString("engine")
			if err != nil {
				return err
			}

			if engineAddr == "" {
				return fmt.Errorf("engine address is required")
			}

			session := &Session{
				EngineAddress: engineAddr,
				SessionID:     uuid.New().String(),
				CreatedAt:     time.Now(),
			}

			if err := SaveSession(session); err != nil {
				return fmt.Errorf("failed to save session: %w", err)
			}

			fmt.Printf("Session saved successfully. Engine address: %s\n", engineAddr)
			return nil
		},
	}

	cmd.Flags().String("engine", "", "Address of the rocketship engine (e.g., localhost:8080)")
	return cmd
}

func setupLocalEnvironment() error {
	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up process manager
	pm := newProcessManager()
	defer pm.Cleanup()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nShutting down local environment...")
		cancel()
		pm.Cleanup()
		os.Exit(0)
	}()

	// Start Temporal server
	log.Println("Starting Temporal server...")
	temporalCmd := exec.CommandContext(ctx, "temporal", "server", "start-dev")
	temporalCmd.Stderr = os.Stderr
	temporalCmd.Stdout = os.Stdout
	if err := temporalCmd.Start(); err != nil {
		return fmt.Errorf("failed to start temporal server: %w", err)
	}
	pm.Add(temporalCmd)

	// Give Temporal a moment to start
	time.Sleep(5 * time.Second)

	// Set environment variables for worker and engine
	if err := os.Setenv("TEMPORAL_HOST", "localhost:7233"); err != nil {
		return fmt.Errorf("failed to set TEMPORAL_HOST environment variable: %w", err)
	}

	// Start the worker from embedded binary
	log.Println("Starting Rocketship worker...")
	workerCmd, err := embedded.ExtractAndRun("worker", nil, []string{"TEMPORAL_HOST=localhost:7233"})
	if err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}
	if err := workerCmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}
	pm.Add(workerCmd)

	// Start the engine from embedded binary
	log.Println("Starting Rocketship engine...")
	engineCmd, err := embedded.ExtractAndRun("engine", nil, []string{"TEMPORAL_HOST=localhost:7233"})
	if err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	if err := engineCmd.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	pm.Add(engineCmd)

	log.Println("Local development environment is ready! 🚀")

	// Keep the parent process running until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// waitForEngine attempts to connect to the engine with exponential backoff
func waitForEngine(ctx context.Context) error {
	client, err := NewEngineClient("localhost:7700")
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	backoff := 100 * time.Millisecond
	maxBackoff := 2 * time.Second
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		checkCtx, cancel := context.WithTimeout(ctx, time.Second)
		err := client.HealthCheck(checkCtx)
		cancel()

		if err == nil {
			return nil
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return fmt.Errorf("timeout waiting for engine to become ready")
}

func setupLocalEnvironmentBackground() error {
	// Create a context that we can cancel
	ctx := context.Background()

	// Create logs directory
	logsDir := filepath.Join(os.TempDir(), "rocketship-logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Set up process manager
	pm := newProcessManager()

	// Open log files
	temporalLog, err := os.OpenFile(filepath.Join(logsDir, "temporal.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporal log file: %w", err)
	}

	workerLog, err := os.OpenFile(filepath.Join(logsDir, "worker.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create worker log file: %w", err)
	}

	engineLog, err := os.OpenFile(filepath.Join(logsDir, "engine.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create engine log file: %w", err)
	}

	// Start Temporal server
	log.Println("Starting Temporal server...")
	temporalCmd := exec.CommandContext(ctx, "temporal", "server", "start-dev")
	temporalCmd.Stdout = temporalLog
	temporalCmd.Stderr = temporalLog
	if err := temporalCmd.Start(); err != nil {
		return fmt.Errorf("failed to start temporal server: %w", err)
	}
	pm.Add(temporalCmd)

	// Give Temporal a moment to start
	time.Sleep(5 * time.Second)

	// Set environment variables for worker and engine
	if err := os.Setenv("TEMPORAL_HOST", "localhost:7233"); err != nil {
		return fmt.Errorf("failed to set TEMPORAL_HOST environment variable: %w", err)
	}

	// Start the worker from embedded binary
	log.Println("Starting Rocketship worker...")
	workerCmd, err := embedded.ExtractAndRun("worker", nil, []string{"TEMPORAL_HOST=localhost:7233"})
	if err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}
	workerCmd.Stdout = workerLog
	workerCmd.Stderr = workerLog
	if err := workerCmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}
	pm.Add(workerCmd)

	// Start the engine from embedded binary
	log.Println("Starting Rocketship engine...")
	engineCmd, err := embedded.ExtractAndRun("engine", nil, []string{"TEMPORAL_HOST=localhost:7233"})
	if err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	engineCmd.Stdout = engineLog
	engineCmd.Stderr = engineLog
	if err := engineCmd.Start(); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}
	pm.Add(engineCmd)

	// Wait for engine to be ready
	log.Println("Waiting for engine to become ready...")
	if err := waitForEngine(context.Background()); err != nil {
		return fmt.Errorf("engine failed to start: %w", err)
	}

	log.Println("Local development environment is ready! 🚀")
	return nil
}
