package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates a new root command
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rocketship",
		Short: "Rocketship CLI - AI-powered API testing platform",
		Long: `Rocketship is an open-source, AI-driven platform for end-to-end API testing.
It uses AI to generate, execute, and analyze API tests, making testing more efficient and comprehensive.`,
	}

	// Add subcommands
	cmd.AddCommand(NewStartCmd())
	cmd.AddCommand(NewRunCmd())
	cmd.AddCommand(NewStatusCmd())
	cmd.AddCommand(NewLogsCmd())
	cmd.AddCommand(NewSuggestCmd())

	return cmd
}
