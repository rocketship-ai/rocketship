package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewRunCmd creates a new run command
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a rocketship test",
		Long:  `Run a rocketship test from a YAML file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load session
			session, err := LoadSession()
			if err != nil {
				return fmt.Errorf("failed to load session: %w", err)
			}

			// Get test file path
			testPath, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}

			if testPath == "" {
				// Look for rocketship.yaml in current directory
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				testPath = filepath.Join(wd, "rocketship.yaml")
			}

			// Read YAML file
			yamlData, err := os.ReadFile(testPath)
			if err != nil {
				return fmt.Errorf("failed to read test file: %w", err)
			}

			// Create engine client
			client, err := NewEngineClient(session.EngineAddress)
			if err != nil {
				return fmt.Errorf("failed to create engine client: %w", err)
			}
			defer client.Close()

			// Create run
			runID, err := client.RunTest(context.Background(), yamlData)
			if err != nil {
				return fmt.Errorf("failed to create run: %w", err)
			}

			fmt.Printf("Created run %s\n", runID)

			// Stream logs
			logStream, err := client.StreamLogs(context.Background(), runID)
			if err != nil {
				return fmt.Errorf("failed to stream logs: %w", err)
			}

			for {
				log, err := logStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("error receiving log: %w", err)
				}
				fmt.Printf("[%s] %s\n", log.Ts, log.Msg)
			}

			return nil
		},
	}

	cmd.Flags().String("file", "", "Path to the test file (default: rocketship.yaml in current directory)")
	return cmd
}
