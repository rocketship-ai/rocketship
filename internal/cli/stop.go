package cli

import (
	"os"
	"path/filepath"

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
			pidFile := filepath.Join(os.TempDir(), "rocketship-server.pid")

			// Check if PID file exists
			if _, err := os.Stat(pidFile); os.IsNotExist(err) {
				Logger.Debug("no running server found (PID file not found)")
				return nil
			}

			// Load process manager state
			pm, err := LoadFromFile(pidFile)
			if err != nil {
				Logger.Debug("failed to load process state", "error", err)
				return nil
			}

			// Cleanup will send SIGTERM to all processes and wait for them to exit
			pm.Cleanup()

			// Remove the PID file
			if err := os.Remove(pidFile); err != nil {
				Logger.Debug("failed to remove PID file", "error", err)
			}

			// Clean up log files
			logsDir := filepath.Join(os.TempDir(), "rocketship-logs")
			if err := os.RemoveAll(logsDir); err != nil {
				Logger.Debug("failed to remove logs directory", "error", err)
			}

			Logger.Debug("server components stopped successfully! 🛑")
			return nil
		},
	}

	return cmd
}
