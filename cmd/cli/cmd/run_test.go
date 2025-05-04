package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

var testStartTime = time.Now()

func TestRunCommand(t *testing.T) {
	origCmd := runCmd
	defer func() { runCmd = origCmd }()

	testCmd := &cobra.Command{
		Use:   "run",
		Short: runCmd.Short,
		Long:  runCmd.Long,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	runCmd = testCmd

	runCmd.Flags().StringVar(&testFile, "file", "", "Path to YAML file")
	runCmd.Flags().StringVar(&testName, "test", "", "Name of test to run")
	runCmd.Flags().StringVar(&outputFormat, "format", "", "Output format (junit)")
	runCmd.Flags().StringVar(&outputFile, "output", "", "Output file")
	if err := runCmd.MarkFlagRequired("file"); err != nil {
		t.Fatal(err)
	}

	runCmd.SetArgs([]string{"--file", "test.yaml"})
	if err := runCmd.Execute(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// runCmd.SetArgs([]string{})
	// if err := runCmd.Execute(); err == nil {
	// 	t.Error("Expected error for missing required flag, got nil")
	// }

	tempFile, err := os.CreateTemp("", "junit-*.xml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()

	err = generateJUnitOutput(tempFile.Name(), "PASSED", []string{"Test log 1", "Test log 2"}, testStartTime)
	if err != nil {
		t.Errorf("Failed to generate JUnit output: %v", err)
	}

	if _, err := os.Stat(tempFile.Name()); os.IsNotExist(err) {
		t.Error("JUnit output file was not created")
	}

	if err := tempFile.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFormatLogs(t *testing.T) {
	logs := []string{"Log 1", "Log 2", "Log 3"}
	formatted := formatLogs(logs)
	expected := "Log 1\nLog 2\nLog 3\n"

	if formatted != expected {
		t.Errorf("Expected formatted logs to be '%s', got '%s'", expected, formatted)
	}
}
