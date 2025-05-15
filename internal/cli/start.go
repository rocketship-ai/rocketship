package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// NewStartCmd creates a new start command
func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start rocketship the rocketship server",
		Long:  `Start rocketship components like the server.`,
	}

	cmd.AddCommand(newStartServerCmd())
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
				// Check if server is already running
				if running, components := IsServerRunning(); running {
					componentNames := make([]string, len(components))
					for i, c := range components {
						componentNames[i] = c.String()
					}
					return fmt.Errorf("server components already running: %s", strings.Join(componentNames, ", "))
				}

				if isBackground {
					// Start processes and return immediately
					return setupLocalEnvironmentBackground()
				}
				return setupLocalEnvironment()
			}

			return fmt.Errorf("remote server connection not yet implemented")
		},
	}

	cmd.Flags().Bool("local", false, "Start a local rocketship server")
	cmd.Flags().Bool("background", false, "Start server in background mode")
	return cmd
}

func setupLocalEnvironment() error {
	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server configuration
	config, err := NewServerConfig()
	if err != nil {
		return fmt.Errorf("failed to create server configuration: %w", err)
	}
	defer config.Cleanup()

	// Get the singleton process manager
	pm := GetProcessManager()
	defer pm.Cleanup()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		Logger.Info("shutting down local environment...")
		cancel()
		pm.Cleanup()
		os.Exit(0)
	}()

	// Start all server components
	if err := StartServer(config, pm); err != nil {
		return err
	}

	Logger.Info("local development environment is ready! 🚀")

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
	// Create server configuration
	config, err := NewServerConfig()
	if err != nil {
		return fmt.Errorf("failed to create server configuration: %w", err)
	}

	// Get the singleton process manager
	pm := GetProcessManager()

	// Start all server components
	if err := StartServer(config, pm); err != nil {
		config.Cleanup()
		return err
	}

	Logger.Info("local development environment is ready! 🚀")
	return nil
}
