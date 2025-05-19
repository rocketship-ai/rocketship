package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates a new root command
func NewRootCmd() *cobra.Command {
	// Initialize logging at the root level
	InitLogging()

	cmd := &cobra.Command{
		Use:   "rocketship",
		Short: "Rocketship CLI",
		Long:  `Rocketship is a CLI tool for running automated tests.`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewStartCmd(),
		NewRunCmd(),
		NewStopCmd(),
		NewVersionCmd(),
	)

	return cmd
}
