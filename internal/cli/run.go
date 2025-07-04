package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	yaml "gopkg.in/yaml.v3"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/dsl"
)

type TestSuiteResult struct {
	Name        string
	TotalTests  int
	PassedTests int
	FailedTests int
}

// processConfigTemplates processes only config variable templates ({{ .vars.* }}) in the YAML
// Leaves runtime variables ({{ variable }}) untouched for later processing
func processConfigTemplates(yamlData []byte, vars map[string]interface{}) ([]byte, error) {
	// Parse YAML to interface{} for template processing
	var yamlDoc interface{}
	if err := yaml.Unmarshal(yamlData, &yamlDoc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML for template processing: %w", err)
	}

	// Process only config variables, leaving runtime variables for later
	processedDoc, err := dsl.ProcessConfigVariablesRecursive(yamlDoc, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to process config variables: %w", err)
	}

	// Convert back to YAML
	processedYaml, err := yaml.Marshal(processedDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal processed YAML: %w", err)
	}

	return processedYaml, nil
}

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
func runSingleTest(ctx context.Context, client *EngineClient, yamlPath string, cliVars map[string]string, varFile string, showTimestamp bool, runContext *generated.RunContext, resultChan chan<- TestSuiteResult) {
	defer func() {
		// Ensure we always send a result, even on panic
		if r := recover(); r != nil {
			resultChan <- TestSuiteResult{
				Name:        "unknown",
				TotalTests:  0,
				PassedTests: 0,
				FailedTests: 0,
			}
		}
	}()

	// Read and validate YAML file
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		Logger.Error("failed to read test file", "path", yamlPath, "error", err)
		resultChan <- TestSuiteResult{Name: "unknown"}
		return
	}

	// Parse YAML to get config
	config, err := dsl.ParseYAML(yamlData)
	if err != nil {
		Logger.Error("failed to parse YAML", "path", yamlPath, "error", err)
		resultChan <- TestSuiteResult{Name: "unknown"}
		return
	}

	// Load variables from file if specified
	var varFileVars map[string]interface{}
	if varFile != "" {
		varFileData, err := os.ReadFile(varFile)
		if err != nil {
			Logger.Error("failed to read variable file", "path", varFile, "error", err)
			resultChan <- TestSuiteResult{Name: config.Name}
			return
		}
		var varFileConfig map[string]interface{}
		if err := yaml.Unmarshal(varFileData, &varFileConfig); err != nil {
			Logger.Error("failed to parse variable file", "path", varFile, "error", err)
			resultChan <- TestSuiteResult{Name: config.Name}
			return
		}
		varFileVars = varFileConfig
	}

	// Merge variables: YAML vars < var-file < CLI vars (CLI takes highest precedence)
	mergedVars := config.Vars
	if varFileVars != nil {
		if mergedVars == nil {
			mergedVars = make(map[string]interface{})
		}
		for k, v := range varFileVars {
			mergedVars[k] = v
		}
	}
	finalVars := dsl.MergeVariables(mergedVars, cliVars)

	// Process templates in the config before sending to engine
	processedYamlData, err := processConfigTemplates(yamlData, finalVars)
	if err != nil {
		Logger.Error("failed to process templates", "path", yamlPath, "error", err)
		resultChan <- TestSuiteResult{Name: config.Name}
		return
	}

	// Create run with timeout
	runCtx, runCancel := context.WithTimeout(ctx, 30*time.Second)
	defer runCancel()

	var runID string
	if runContext != nil {
		runID, err = client.RunTestWithContext(runCtx, processedYamlData, runContext)
	} else {
		runID, err = client.RunTest(runCtx, processedYamlData)
	}
	if err != nil {
		Logger.Error("failed to create run", "path", yamlPath, "error", err)
		resultChan <- TestSuiteResult{Name: config.Name}
		return
	}

	Logger.Info("Starting log streaming", "run_id", runID)
	// Stream logs and track results
	logStream, err := client.StreamLogs(ctx, runID)
	if err != nil {
		Logger.Error("failed to stream logs", "path", yamlPath, "error", err)
		resultChan <- TestSuiteResult{Name: config.Name}
		return
	}
	Logger.Info("Log stream established, entering monitoring loop", "run_id", runID)

	var result TestSuiteResult
	result.Name = config.Name

	// Create a channel to receive logs
	logChan := make(chan *generated.LogLine)
	errChan := make(chan error)
	
	// Start goroutine to receive logs
	go func() {
		defer close(logChan)
		defer close(errChan)
		for {
			log, err := logStream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			logChan <- log
		}
	}()

	for {
		select {
		case <-ctx.Done():
			Logger.Info("Context cancelled, attempting to cancel run", "run_id", runID)
			// Try to cancel the run on the server side
			if err := client.CancelRun(context.Background(), runID); err != nil {
				Logger.Error("Failed to cancel run", "run_id", runID, "error", err)
			} else {
				Logger.Info("Successfully requested run cancellation", "run_id", runID)
			}
			resultChan <- result
			return
		case log := <-logChan:
			if log == nil {
				// Channel closed, stream ended
				resultChan <- result
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

			// Build multi-level bracket prefix
			brackets := "[" + config.Name + "]"
			if log.TestName != "" {
				brackets += " [" + log.TestName + "]"
			}
			if log.StepName != "" {
				brackets += " [" + log.StepName + "]"
			}
			
			// Print the log with multi-level bracket prefix and optional timestamp
			if showTimestamp {
				fmt.Printf("%s [%s] %s\n", printer.Sprint(brackets), log.Ts, log.Msg)
			} else {
				fmt.Printf("%s %s\n", printer.Sprint(brackets), log.Msg)
			}

			// Parse final summary message to extract results
			if strings.Contains(log.Msg, "Test run:") && strings.Contains(log.Msg, "finished") {
				if strings.Contains(log.Msg, "All") {
					// Format: "Test run: "XXX" finished. All N tests passed."
					var count int
					_, err := fmt.Sscanf(log.Msg, "Test run: %q finished. All %d tests passed.", &result.Name, &count)
					if err == nil {
						result.TotalTests = count
						result.PassedTests = count
					}
				} else {
					// Format: "Test run: "XXX" finished. N/M tests passed, P/M tests failed."
					_, err := fmt.Sscanf(log.Msg, "Test run: %q finished. %d/%d tests passed, %d/%d tests failed.",
						&result.Name, &result.PassedTests, &result.TotalTests, &result.FailedTests, &result.TotalTests)
					if err != nil {
						Logger.Error("failed to parse test results", "message", log.Msg, "error", err)
					}
				}
			}
		case err := <-errChan:
			if err == io.EOF {
				resultChan <- result
				return
			}
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
					Logger.Info("Log stream cancelled", "run_id", runID)
					resultChan <- result
					return
				}
				Logger.Error("error receiving log", "path", yamlPath, "error", err)
				resultChan <- result
				return
			}
		}
	}
}

// printFinalSummary prints the aggregated results of all test suites
func printFinalSummary(results []TestSuiteResult) {
	totalSuites := len(results)
	passedSuites := 0
	totalTests := 0
	passedTests := 0
	failedTests := 0

	for _, r := range results {
		if r.FailedTests == 0 && r.TotalTests > 0 {
			passedSuites++
		}
		totalTests += r.TotalTests
		passedTests += r.PassedTests
		failedTests += r.FailedTests
	}

	fmt.Println("\n=== Final Summary ===")
	fmt.Printf("Total Test Suites: %d\n", totalSuites)
	fmt.Printf("%s Passed Suites: %d\n", color.GreenString("✓"), passedSuites)
	fmt.Printf("%s Failed Suites: %d\n", color.RedString("✗"), totalSuites-passedSuites)
	fmt.Printf("\nTotal Tests: %d\n", totalTests)
	fmt.Printf("%s Passed Tests: %d\n", color.GreenString("✓"), passedTests)
	fmt.Printf("%s Failed Tests: %d\n", color.RedString("✗"), failedTests)
}

// displayRecentRuns shows recent test runs after an auto run completes
func displayRecentRuns(client *EngineClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Request all recent runs
	req := &generated.ListRunsRequest{
		OrderBy:    "started_at",
		Descending: true,
	}

	resp, err := client.client.ListRuns(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list recent runs: %w", err)
	}

	if len(resp.Runs) == 0 {
		return nil
	}

	fmt.Println("\n=== Recent Test Runs ===")
	return displayRunsTable(resp.Runs)
}

// NewRunCmd creates a new run command
func NewRunCmd() *cobra.Command {
	fmt.Println("DEBUG: NewRunCmd called")
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run rocketship tests",
		Long:  `Run rocketship tests from YAML files. Can run a single file or all tests in a directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("DEBUG: RunE function called")
			Logger.Info("=== RUN COMMAND STARTING ===")
			
			// Create a context that we can cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			Logger.Info("=== SETTING UP SIGNAL HANDLER ===")
			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			Logger.Info("Signal handler setup complete, waiting for signals...")
			go func() {
				Logger.Info("Signal handler goroutine started")
				sig := <-sigChan
				Logger.Info("Received interrupt signal", "signal", sig.String(), "action", "cancelling operations")
				cancel()
				Logger.Info("Context cancelled due to signal", "signal", sig.String())
			}()
			Logger.Info("=== SIGNAL HANDLER SETUP COMPLETE ===")

			// Check if we're in auto mode
			isAuto, err := cmd.Flags().GetBool("auto")
			if err != nil {
				return err
			}

			// Get engine address from flag
			engineAddr, err := cmd.Flags().GetString("engine")
			if err != nil {
				return err
			}

			// Validate flags - cannot use both --auto and --engine (when explicitly set)
			engineFlagSet := cmd.Flags().Changed("engine")
			if isAuto && engineFlagSet {
				return fmt.Errorf("cannot use both --auto and --engine flags together. Use --auto to automatically manage a local server, or --engine to connect to an existing server")
			}

			var cleanup func()
			// Handle server management based on flags
			if isAuto {
				// Check if server is already running
				if running, components := IsServerRunning(); running {
					componentNames := make([]string, len(components))
					for i, c := range components {
						componentNames[i] = c.String()
					}
					return fmt.Errorf("cannot start in auto mode - server components already running: %s", strings.Join(componentNames, ", "))
				}

				if err := setupLocalServerBackground(); err != nil {
					return fmt.Errorf("failed to start local server: %w", err)
				}
				engineAddr = "localhost:7700"
				cleanup = func() {
					pm := GetProcessManager()
					pm.Cleanup()
				}
				defer cleanup()
			}

			// Create engine client using the engine address
			client, err := NewEngineClient(engineAddr)
			if err != nil {
				return fmt.Errorf("failed to create engine client: %w", err)
			}
			defer func() { _ = client.Close() }()

			// Get variable flags
			cliVars, err := cmd.Flags().GetStringToString("var")
			if err != nil {
				return err
			}

			varFile, err := cmd.Flags().GetString("var-file")
			if err != nil {
				return err
			}

			// Get timestamp flag
			showTimestamp, err := cmd.Flags().GetBool("timestamp")
			if err != nil {
				return err
			}

			// Get context flags
			projectID, _ := cmd.Flags().GetString("project-id")
			source, _ := cmd.Flags().GetString("source")
			branch, _ := cmd.Flags().GetString("branch")
			commit, _ := cmd.Flags().GetString("commit")
			trigger, _ := cmd.Flags().GetString("trigger")
			scheduleName, _ := cmd.Flags().GetString("schedule-name")
			metadata, _ := cmd.Flags().GetStringToString("metadata")

			// Build RunContext if any context flags are set
			var runContext *generated.RunContext
			if projectID != "" || source != "" || branch != "" || commit != "" || 
			   trigger != "" || scheduleName != "" || len(metadata) > 0 {
				runContext = &generated.RunContext{
					ProjectId:    projectID,
					Source:       source,
					Branch:       branch,
					CommitSha:    commit,
					Trigger:      trigger,
					ScheduleName: scheduleName,
					Metadata:     metadata,
				}
			}

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

			// Channel to collect results from all test suites
			resultChan := make(chan TestSuiteResult, len(testFiles))

			// Run all tests in parallel
			var wg sync.WaitGroup
			for _, tf := range testFiles {
				wg.Add(1)
				go func(testFile string) {
					defer wg.Done()
					runSingleTest(ctx, client, testFile, cliVars, varFile, showTimestamp, runContext, resultChan)
				}(tf)
			}

			// Wait for all tests in a separate goroutine
			go func() {
				wg.Wait()
				close(resultChan)
			}()

			// Collect results with context cancellation handling
			var results []TestSuiteResult
			for {
				select {
				case <-ctx.Done():
					Logger.Info("Context cancelled during result collection, cleaning up auto mode server if needed")
					// If we're in auto mode and context is cancelled, call cleanup immediately
					if isAuto && cleanup != nil {
						Logger.Info("Calling cleanup function for auto mode server")
						cleanup()
					}
					return fmt.Errorf("operation cancelled")
				case result, ok := <-resultChan:
					if !ok {
						// Channel closed, all results collected
						goto collectComplete
					}
					results = append(results, result)
				}
			}
			collectComplete:

			// Print final summary
			printFinalSummary(results)
			
			// If this was an auto run, also display recent test runs
			if isAuto {
				if err := displayRecentRuns(client); err != nil {
					Logger.Debug("failed to display recent runs", "error", err)
				}
			}
			
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "Path to a single test file (default: rocketship.yaml in current directory)")
	cmd.Flags().StringP("dir", "d", "", "Path to directory containing test files (will run all rocketship.yaml files recursively)")
	cmd.Flags().StringP("engine", "e", "localhost:7700", "Address of the rocketship engine")
	cmd.Flags().BoolP("auto", "a", false, "Automatically start and stop the local server for test execution")
	cmd.Flags().StringToStringP("var", "v", nil, "Set variables (can be used multiple times: --var key=value --var nested.key=value)")
	cmd.Flags().StringP("var-file", "", "", "Load variables from YAML file")
	cmd.Flags().BoolP("timestamp", "t", false, "Show timestamps in log output")
	
	// Context flags for enhanced metadata tracking
	cmd.Flags().String("project-id", "", "Project identifier for test run tracking")
	cmd.Flags().String("source", "", "Run source: cli-local, ci-branch, ci-main, scheduled")
	cmd.Flags().String("branch", "", "Git branch name (auto-detected if not specified)")
	cmd.Flags().String("commit", "", "Git commit SHA (auto-detected if not specified)")
	cmd.Flags().String("trigger", "", "Trigger type: manual, webhook, schedule")
	cmd.Flags().String("schedule-name", "", "Schedule name for scheduled runs")
	cmd.Flags().StringToString("metadata", nil, "Additional metadata key=value pairs")
	
	return cmd
}
