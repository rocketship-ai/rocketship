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

type testSummary struct {
	totalSuites  int
	passedSuites int
	failedSuites int
	totalTests   int
	passedTests  int
	failedTests  int
}

func summarizeResults(results []TestSuiteResult) testSummary {
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

	return testSummary{
		totalSuites:  totalSuites,
		passedSuites: passedSuites,
		failedSuites: totalSuites - passedSuites,
		totalTests:   totalTests,
		passedTests:  passedTests,
		failedTests:  failedTests,
	}
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

// isReservedTestDir returns true if a directory name is reserved for internal use
// within the .rocketship tree (e.g. tmp scratch space).
func isReservedTestDir(name string) bool {
	switch name {
	case "tmp":
		return true
	default:
		return false
	}
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

// findYamlTestFiles recursively finds all YAML test files in the given directory.
// Used for the .rocketship directory, where any *.yaml file is considered a test suite,
// except for files under a tmp/ directory (e.g. .rocketship/tmp/).
func findYamlTestFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && isReservedTestDir(info.Name()) {
			// Skip reserved directories like .rocketship/tmp/
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
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

	Logger.Debug("Starting log streaming", "run_id", runID)
	// Stream logs and track results
	logStream, err := client.StreamLogs(ctx, runID)
	if err != nil {
		Logger.Error("failed to stream logs", "path", yamlPath, "error", err)
		resultChan <- TestSuiteResult{Name: config.Name}
		return
	}
	Logger.Debug("Log stream established, entering monitoring loop", "run_id", runID)

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
func printFinalSummary(summary testSummary) {
	fmt.Println("\n=== Final Summary ===")
	fmt.Printf("Total Test Suites: %d\n", summary.totalSuites)
	fmt.Printf("%s Passed Suites: %d\n", color.GreenString("✓"), summary.passedSuites)
	fmt.Printf("%s Failed Suites: %d\n", color.RedString("✗"), summary.failedSuites)
	fmt.Printf("\nTotal Tests: %d\n", summary.totalTests)
	fmt.Printf("%s Passed Tests: %d\n", color.GreenString("✓"), summary.passedTests)
	fmt.Printf("%s Failed Tests: %d\n", color.RedString("✗"), summary.failedTests)
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
		if wrapped := translateAuthError("failed to list recent runs", err); wrapped != nil {
			return wrapped
		}
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
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run rocketship tests",
		Long:  `Run rocketship tests from YAML files. Can run a single file or all tests in a directory.`,
		SilenceUsage: true, // Don't print usage on test failures
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a context that we can cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigChan
				cancel()
				Logger.Debug("Context cancelled due to signal", "signal", sig.String())
			}()

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
			var clientAddress string

			// Handle server management and address resolution based on flags
			if isAuto {
				// Auto mode - start local server and force localhost address
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
				clientAddress = "localhost:7700" // Force localhost for auto mode
				cleanup = func() {
					// Wait for suite cleanup workflows to complete before shutting down
					// Cleanup workflows have a 45-minute timeout, so we need to wait at least that long.
					// We set 50 minutes to allow for some buffer time.
					waitCtx, waitCancel := context.WithTimeout(context.Background(), 50*time.Minute)
					defer waitCancel()

					tempClient, err := NewEngineClient("localhost:7700")
					if err == nil {
						Logger.Debug("Waiting for suite cleanup workflows to complete (this may take up to 45 minutes)...")
						_, err := tempClient.client.WaitForCleanup(waitCtx, &generated.WaitForCleanupRequest{TimeoutSeconds: 3000})
						if err != nil {
							Logger.Warn("Failed to wait for cleanup", "error", err)
						} else {
							Logger.Debug("Suite cleanup workflows completed")
						}
						_ = tempClient.Close()
					}

					pm := GetProcessManager()
					pm.Cleanup()
				}
				defer cleanup()
			} else if engineFlagSet {
				// Explicit engine address provided - use it directly
				clientAddress = engineAddr
			} else {
				// No flags set - let the client middleware handle profile resolution
				clientAddress = "" // Empty string triggers profile resolution
			}

			// Create engine client - middleware will handle address resolution
			client, err := NewEngineClient(clientAddress)
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

			envFile, err := cmd.Flags().GetString("env-file")
			if err != nil {
				return err
			}

			// Load environment variables from file if specified
			if envFile != "" {
				envVars, err := loadEnvFile(envFile)
				if err != nil {
					return fmt.Errorf("failed to load env file: %w", err)
				}
				if err := setEnvironmentVariables(envVars); err != nil {
					return fmt.Errorf("failed to set environment variables: %w", err)
				}
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
				// When pointing at a .rocketship directory, treat any *.yaml file as a test suite
				// (excluding scratch files under .rocketship/tmp). For other directories, preserve
				// the existing behavior of only running rocketship.yaml files.
				if filepath.Base(dirPath) == ".rocketship" {
					testFiles, err = findYamlTestFiles(dirPath)
					if err != nil {
						return fmt.Errorf("failed to find test files: %w", err)
					}
					if len(testFiles) == 0 {
						return fmt.Errorf("no YAML test files found in directory: %s", dirPath)
					}
				} else {
					testFiles, err = findRocketshipFiles(dirPath)
					if err != nil {
						return fmt.Errorf("failed to find test files: %w", err)
					}
					if len(testFiles) == 0 {
						return fmt.Errorf("no rocketship.yaml files found in directory: %s", dirPath)
					}
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
					Logger.Debug("Context cancelled during result collection")

					// FIRST: Wait for all test goroutines to finish their CancelRun calls
					Logger.Debug("Waiting for test goroutines to complete their cancellation requests...")
					wg.Wait()
					Logger.Debug("All test goroutines have completed cancellation")

					// THEN: Wait for cleanup workflows to complete BEFORE shutting down server
					Logger.Debug("Waiting for cleanup workflows to complete...")
					cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cleanupCancel()

					if err := client.WaitForCleanup(cleanupCtx); err != nil {
						Logger.Warn("Failed to wait for cleanup", "error", err)
					} else {
						Logger.Debug("Cleanup workflows completed successfully")
					}

					// FINALLY: Now it's safe to shut down the server
					if isAuto && cleanup != nil {
						Logger.Debug("Calling cleanup function for auto mode server")
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
			summary := summarizeResults(results)
			printFinalSummary(summary)

			// If this was an auto run, also display recent test runs
			if isAuto {
				if err := displayRecentRuns(client); err != nil {
					Logger.Debug("failed to display recent runs", "error", err)
				}
			}

			if summary.totalSuites == 0 {
				return fmt.Errorf("no test suites executed")
			}
			if summary.failedSuites > 0 {
				return fmt.Errorf("%d test suite(s) failed", summary.failedSuites)
			}

			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "Path to a Rocketship test file (YAML)")
	cmd.Flags().StringP("dir", "d", "", "Path to directory containing test files (for .rocketship, runs all YAML test files recursively)")
	cmd.Flags().StringP("engine", "e", "", "Address of the rocketship engine (defaults to active profile)")
	cmd.Flags().BoolP("auto", "a", false, "Automatically start and stop the local server for test execution")
	cmd.Flags().StringToStringP("var", "v", nil, "Set variables (can be used multiple times: --var key=value --var nested.key=value)")
	cmd.Flags().StringP("var-file", "", "", "Load variables from YAML file")
	cmd.Flags().StringP("env-file", "", "", "Load environment variables from .env file")
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
