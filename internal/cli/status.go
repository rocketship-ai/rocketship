package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewStatusCmd creates a new status command
func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check the status of the Rocketship engine",
		Long: `Check the current status of the Rocketship engine,
including whether it's running and its current state.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status check
			fmt.Println("Checking Rocketship engine status...")
			return nil
		},
	}

	return cmd
} 