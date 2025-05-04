package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewStartCmd creates a new start command
func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Rocketship engine",
		Long: `Start the Rocketship engine which will process test requests
and manage the test execution workflow.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement engine start
			fmt.Println("Starting Rocketship engine...")
			return nil
		},
	}

	return cmd
}
