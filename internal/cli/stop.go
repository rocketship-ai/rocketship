package cli

import (
	"context"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/spf13/cobra"
)

// NewStopCmd creates a new stop command
func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop rocketship the rocketship server",
		Long:  `Stop rocketship components like the server.`,
	}

	cmd.AddCommand(newStopServerCmd())

	return cmd
}

func newStopServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Stop the rocketship server",
		Long:  `Stop the rocketship server and all its components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if any components are running
			if running, components := IsServerRunning(); !running {
				Logger.Info("no server components are running")
				return nil
			} else {
				componentNames := make([]string, len(components))
				for i, c := range components {
					componentNames[i] = c.String()
				}
				Logger.Debug("stopping components", "components", componentNames)
			}

			// Wait for suite cleanup workflows to complete before shutting down
			waitForCleanupBeforeShutdown()

			// Get the process manager and cleanup
			pm := GetProcessManager()
			pm.Cleanup()

			Logger.Info("server components stopped successfully! 🛑")
			return nil
		},
	}

	return cmd
}

// waitForCleanupBeforeShutdown waits for suite cleanup workflows to complete before server shutdown
func waitForCleanupBeforeShutdown() {
	// Cleanup workflows have a 45-minute timeout, so we need to wait at least that long.
	// We set 50 minutes to allow for some buffer time.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 50*time.Minute)
	defer waitCancel()

	tempClient, err := NewEngineClient("localhost:7700")
	if err != nil {
		Logger.Debug("Could not connect to engine for cleanup wait", "error", err)
		return
	}
	defer func() { _ = tempClient.Close() }()

	Logger.Debug("Waiting for suite cleanup workflows to complete (this may take up to 45 minutes)...")
	_, err = tempClient.client.WaitForCleanup(waitCtx, &generated.WaitForCleanupRequest{TimeoutSeconds: 3000})
	if err != nil {
		Logger.Warn("Failed to wait for cleanup", "error", err)
	} else {
		Logger.Debug("Suite cleanup workflows completed")
	}
}
