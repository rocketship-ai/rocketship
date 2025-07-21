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
		Long:  `Start a local rocketship server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			isBackground, err := cmd.Flags().GetBool("background")
			if err != nil {
				return err
			}
			
			https, err := cmd.Flags().GetBool("https")
			if err != nil {
				return err
			}
			
			domain, err := cmd.Flags().GetString("domain")
			if err != nil {
				return err
			}

			// Configure TLS if HTTPS is enabled
			if https {
				if domain == "" {
					return fmt.Errorf("--domain is required when --https is enabled")
				}
				
				// Set TLS environment variables
				_ = os.Setenv("ROCKETSHIP_TLS_ENABLED", "true")
				_ = os.Setenv("ROCKETSHIP_TLS_DOMAIN", domain)
				Logger.Info("HTTPS enabled", "domain", domain)
			} else {
				// Ensure TLS is disabled
				_ = os.Setenv("ROCKETSHIP_TLS_ENABLED", "false")
			}

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
				return setupLocalServerBackground()
			}
			return setupLocalServer()
		},
	}

	cmd.Flags().BoolP("background", "b", false, "Start server in background mode")
	cmd.Flags().Bool("https", false, "Enable HTTPS using generated certificates")
	cmd.Flags().String("domain", "", "Domain name for HTTPS certificate (required with --https)")
	return cmd
}

func setupLocalServer() error {
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

	Logger.Info("local server is ready! 🚀")

	// Keep the parent process running until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// waitForEngine attempts to connect to the engine with exponential backoff
func waitForEngine(ctx context.Context) error {
	// Use appropriate address based on TLS configuration
	address := "localhost:7700"
	if os.Getenv("ROCKETSHIP_TLS_ENABLED") == "true" {
		// TLS typically uses port 443, but we'll keep 7700 for consistency
		address = "localhost:7700" 
	}
	
	client, err := NewEngineClient(address)
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

func setupLocalServerBackground() error {
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

	Logger.Info("local server is ready! 🚀")
	return nil
}
