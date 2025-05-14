package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewStopCmd creates a new stop command
func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop rocketship components",
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
				return fmt.Errorf("no running server found (PID file not found)")
			}

			// Load process manager state
			pm, err := LoadFromFile(pidFile)
			if err != nil {
				return fmt.Errorf("failed to load process state: %w", err)
			}

			// Cleanup will send SIGTERM to all processes and wait for them to exit
			pm.Cleanup()

			// Remove the PID file
			if err := os.Remove(pidFile); err != nil {
				log.Printf("Warning: Failed to remove PID file: %v", err)
			}

			// Clean up log files
			logsDir := filepath.Join(os.TempDir(), "rocketship-logs")
			if err := os.RemoveAll(logsDir); err != nil {
				log.Printf("Warning: Failed to remove logs directory: %v", err)
			}

			fmt.Println("Server components stopped successfully! 🛑")
			return nil
		},
	}

	return cmd
}
