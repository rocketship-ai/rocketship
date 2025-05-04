package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewRunCmd creates a new run command
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [test-file]",
		Short: "Run a test file",
		Long: `Run a test file specified by the path.
The test file should be a YAML file containing the test definition.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testFile := args[0]
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				return fmt.Errorf("test file %s does not exist", testFile)
			}
			// TODO: Implement test execution
			fmt.Printf("Running test file: %s\n", testFile)
			return nil
		},
	}

	return cmd
} 