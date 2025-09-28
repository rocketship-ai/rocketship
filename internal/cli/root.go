package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// NewRootCmd creates a new root command
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rocketship",
		Short: "Rocketship CLI",
		Long:  `Rocketship is a CLI tool for running automated tests.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Check if debug flag is set
			debug, _ := cmd.Flags().GetBool("debug")
			if debug {
				// Set the environment variable for debug logging
				_ = os.Setenv("ROCKETSHIP_LOG", "DEBUG")
			}
			
			// Initialize logging after potentially setting the debug env var
			InitLogging()
		},
	}

	// Add persistent flags
	cmd.PersistentFlags().Bool("debug", false, "Enable debug logging")

	// Add subcommands
	cmd.AddCommand(
		NewStartCmd(),
		NewRunCmd(),
		NewStopCmd(),
		NewVersionCmd(),
		NewValidateCmd(),
		NewListCmd(),
		NewGetCmd(),
		NewProfileCmd(),
		NewLoginCmd(),
		NewLogoutCmd(),
		NewAuthStatusCmd(),
	)

	return cmd
}
