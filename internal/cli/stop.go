package cli

import (
	"fmt"
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

			// Load process manager state
			pm, err := LoadFromFile(pidFile)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no running server found")
				}
				return fmt.Errorf("failed to load process state: %w", err)
			}

			// Clean up processes
			pm.Cleanup()

			// Remove PID file
			if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove PID file: %w", err)
			}

			fmt.Println("Server components stopped successfully! 🛑")
			return nil
		},
	}

	return cmd
}
