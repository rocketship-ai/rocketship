package cli

import (
	"fmt"
	"os"

	"github.com/rocketship-ai/rocketship/internal/embedded"
	"github.com/spf13/cobra"
)

// NewVersionCmd creates a new version command
func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Rocketship",
		Long:  `Print the version number of Rocketship CLI.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Get version from environment variable or use default
			version := os.Getenv("ROCKETSHIP_VERSION")
			if version == "" {
				version = embedded.DefaultVersion
			}
			fmt.Printf("Rocketship CLI %s\n", version)
		},
	}

	return cmd
}
