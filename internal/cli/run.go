package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// NewRunCmd creates a new run command
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a rocketship test",
		Long:  `Run a rocketship test from a YAML file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a context that we can cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Load session
			session, err := LoadSession()
			if err != nil {
				return fmt.Errorf("no active session found - use 'rocketship start' first: %w", err)
			}

			// Get test file path
			testFile, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}

			if testFile == "" {
				// Look for rocketship.yaml in current directory
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				testFile = filepath.Join(wd, "rocketship.yaml")
			}

			// Read YAML file
			yamlData, err := os.ReadFile(testFile)
			if err != nil {
				return fmt.Errorf("failed to read test file: %w", err)
			}

			// client-side validation of YAML
			_, err = dsl.ParseYAML(yamlData)
			if err != nil {
				return fmt.Errorf("failed to parse YAML: %w", err)
			}

			// Create engine client
			client, err := NewEngineClient(session.EngineAddress)
			if err != nil {
				return fmt.Errorf("failed to create engine client: %w", err)
			}
			defer func() { _ = client.Close() }()

			// Create run with timeout
			runCtx, runCancel := context.WithTimeout(ctx, 30*time.Second)
			defer runCancel()

			runID, err := client.RunTest(runCtx, yamlData)
			if err != nil {
				return fmt.Errorf("failed to create run: %w", err)
			}

			// Stream logs
			logStream, err := client.StreamLogs(ctx, runID)
			if err != nil {
				return fmt.Errorf("failed to stream logs: %w", err)
			}

			for {
				select {
				case <-ctx.Done():
					return nil
				default:
					log, err := logStream.Recv()
					if err == io.EOF {
						return nil
					}
					if err != nil {
						// Check if the error is due to context cancellation
						if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
							return nil
						}
						return fmt.Errorf("error receiving log: %w", err)
					}

					// Apply styling based on metadata
					printer := color.New()
					switch log.Color {
					case "green":
						printer.Add(color.FgGreen)
					case "red":
						printer.Add(color.FgRed)
					case "purple":
						printer.Add(color.FgMagenta)
					}
					if log.Bold {
						printer.Add(color.Bold)
					}

					fmt.Printf("%s\n", printer.Sprintf("[%s] %s", log.Ts, log.Msg))
				}
			}
		},
	}

	cmd.Flags().String("file", "", "Path to the test file (default: rocketship.yaml in current directory)")
	return cmd
}
