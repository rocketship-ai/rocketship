package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewSuggestCmd creates a new suggest command
func NewSuggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest [api-endpoint]",
		Short: "Suggest tests for an API endpoint",
		Long: `Use AI to suggest test cases for a given API endpoint.
The endpoint should be specified as a URL.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			// TODO: Implement test suggestion
			fmt.Printf("Suggesting tests for endpoint: %s\n", endpoint)
			return nil
		},
	}

	return cmd
} 