package cli

import (
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

			// Get the process manager and cleanup
			pm := GetProcessManager()
			pm.Cleanup()

			Logger.Info("server components stopped successfully! 🛑")
			return nil
		},
	}

	return cmd
}
