package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// findRocketshipFiles recursively finds all rocketship.yaml files in the given directory
func findRocketshipFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (info.Name() == "rocketship.yaml") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// runSingleTest runs a single test file and streams its logs
func runSingleTest(ctx context.Context, client *EngineClient, yamlPath string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Read and validate YAML file
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to read test file %s: %v\n", yamlPath, err)
		return
	}

	// client-side validation of YAML
	_, err = dsl.ParseYAML(yamlData)
	if err != nil {
		fmt.Printf("[ERROR] Failed to parse YAML %s: %v\n", yamlPath, err)
		return
	}

	// Create run with timeout
	runCtx, runCancel := context.WithTimeout(ctx, 30*time.Second)
	defer runCancel()

	runID, err := client.RunTest(runCtx, yamlData)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create run for %s: %v\n", yamlPath, err)
		return
	}

	// Stream logs
	logStream, err := client.StreamLogs(ctx, runID)
	if err != nil {
		fmt.Printf("[ERROR] Failed to stream logs for %s: %v\n", yamlPath, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			log, err := logStream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
					return
				}
				fmt.Printf("[ERROR] Error receiving log for %s: %v\n", yamlPath, err)
				return
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

			// Include the test file path in the log output for clarity
			fmt.Printf("%s [%s] %s\n", printer.Sprint("["+filepath.Base(filepath.Dir(yamlPath))+"]"), log.Ts, log.Msg)
		}
	}
}

// NewRunCmd creates a new run command
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run rocketship tests",
		Long:  `Run rocketship tests from YAML files. Can run a single file or all tests in a directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a context that we can cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Get engine address from flag
			engineAddr, err := cmd.Flags().GetString("engine")
			if err != nil {
				return err
			}

			// If engine address not provided, try to load from session
			if engineAddr == "" {
				session, err := LoadSession()
				if err != nil {
					return fmt.Errorf("no engine address provided and no active session found - use --engine flag or 'rocketship start' first: %w", err)
				}
				engineAddr = session.EngineAddress
			}

			// Create engine client using the engine address
			client, err := NewEngineClient(engineAddr)
			if err != nil {
				return fmt.Errorf("failed to create engine client: %w", err)
			}
			defer func() { _ = client.Close() }()

			// Get test file or directory path
			testFile, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}

			dirPath, err := cmd.Flags().GetString("dir")
			if err != nil {
				return err
			}

			var testFiles []string

			if dirPath != "" {
				// Find all rocketship.yaml files in the directory
				testFiles, err = findRocketshipFiles(dirPath)
				if err != nil {
					return fmt.Errorf("failed to find test files: %w", err)
				}
				if len(testFiles) == 0 {
					return fmt.Errorf("no rocketship.yaml files found in directory: %s", dirPath)
				}
			} else if testFile != "" {
				testFiles = []string{testFile}
			} else {
				// Look for rocketship.yaml in current directory
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				testFiles = []string{filepath.Join(wd, "rocketship.yaml")}
			}

			// Run all tests in parallel
			var wg sync.WaitGroup
			for _, tf := range testFiles {
				wg.Add(1)
				go runSingleTest(ctx, client, tf, &wg)
			}

			// Wait for all tests to complete
			wg.Wait()
			return nil
		},
	}

	cmd.Flags().String("file", "", "Path to a single test file (default: rocketship.yaml in current directory)")
	cmd.Flags().String("dir", "", "Path to directory containing test files (will run all rocketship.yaml files recursively)")
	cmd.Flags().String("engine", "", "Address of the rocketship engine (e.g., localhost:7700)")
	return cmd
}
