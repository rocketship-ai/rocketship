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

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// NewRunCmd creates a new run command
func NewRunCmd() *cobra.Command {
	// Initialize color functions with bold
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	purple := color.New(color.FgMagenta, color.Bold).SprintFunc()

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
				fmt.Println("\nCancelling test...")
				cancel()
			}()

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

			run, err := dsl.ParseYAML(yamlData)
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

			fmt.Printf("%s\n", purple(fmt.Sprintf("Starting test run \"%s\"...", run.Name)))

			// Stream logs
			logStream, err := client.StreamLogs(ctx, runID)
			if err != nil {
				return fmt.Errorf("failed to stream logs: %w", err)
			}

			for {
				select {
				case <-ctx.Done():
					fmt.Printf("\n%s\n", purple(fmt.Sprintf("Test run \"%s\" has finished", run.Name)))
					return nil
				default:
					log, err := logStream.Recv()
					if err == io.EOF {
						fmt.Printf("\n%s\n", purple(fmt.Sprintf("Test run \"%s\" has finished", run.Name)))
						return nil
					}
					if err != nil {
						// Check if the error is due to context cancellation
						if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
							fmt.Printf("\n%s\n", purple(fmt.Sprintf("Test run \"%s\" has finished", run.Name)))
							return nil
						}
						return fmt.Errorf("error receiving log: %w", err)
					}
					// Check if the log message contains test result information
					if msg := log.Msg; len(msg) > 7 {
						if msg[:7] == "Test: \"" {
							if msg[len(msg)-6:] == "passed" {
								fmt.Printf("%s\n", green(fmt.Sprintf("[%s] %s", log.Ts, msg)))
							} else if msg[len(msg)-6:] == "failed" {
								fmt.Printf("%s\n", red(fmt.Sprintf("[%s] %s", log.Ts, msg)))
							} else {
								fmt.Printf("[%s] %s\n", log.Ts, msg)
							}
						} else {
							fmt.Printf("[%s] %s\n", log.Ts, msg)
						}
					} else {
						fmt.Printf("[%s] %s\n", log.Ts, log.Msg)
					}
				}
			}
		},
	}

	cmd.Flags().String("file", "", "Path to the test file (default: rocketship.yaml in current directory)")
	return cmd
}
