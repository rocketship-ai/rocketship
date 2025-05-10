package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

			// Set up signal handling
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Println("\nReceived interrupt signal, cleaning up...")
				cancel()
			}()

			// Load session
			session, err := LoadSession()
			if err != nil {
				return fmt.Errorf("failed to load session: %w", err)
			}

			fmt.Printf("Connecting to engine at %s...\n", session.EngineAddress)

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

			fmt.Printf("Reading test file: %s\n", testPath)

			// Read YAML file
			yamlData, err := os.ReadFile(testPath)
			if err != nil {
				return fmt.Errorf("failed to read test file: %w", err)
			}

			fmt.Printf("YAML file size: %d bytes\n", len(yamlData))

			// Create engine client
			fmt.Println("Creating engine client...")
			client, err := NewEngineClient(session.EngineAddress)
			if err != nil {
				return fmt.Errorf("failed to create engine client: %w", err)
			}

			defer func() {
				fmt.Println("Closing engine client...")
				if err := client.Close(); err != nil {
					fmt.Printf("Warning: error closing client: %v\n", err)
				}
			}()

			fmt.Println("Creating run...")

			// Create run with timeout
			runCtx, runCancel := context.WithTimeout(ctx, 30*time.Second)
			defer runCancel()

			runID, err := client.RunTest(runCtx, yamlData)
			if err != nil {
				return fmt.Errorf("failed to create run: %w", err)
			}

			fmt.Printf("Created run %s\n", runID)

			// Stream logs
			fmt.Println("Starting log stream...")
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
					fmt.Printf("[%s] %s\n", log.Ts, log.Msg)
				}
			}
		},
	}

	cmd.Flags().String("file", "", "Path to the test file (default: rocketship.yaml in current directory)")
	return cmd
}
