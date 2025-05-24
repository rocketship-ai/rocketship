package cli

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rocketship-ai/rocketship/internal/embedded"
)

func TestNewVersionCmd(t *testing.T) {
	t.Parallel()

	cmd := NewVersionCmd()
	if cmd == nil {
		t.Fatal("NewVersionCmd returned nil")
	}

	if cmd.Use != "version" {
		t.Errorf("Expected Use to be 'version', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be non-empty")
	}

	if cmd.Long == "" {
		t.Error("Expected Long description to be non-empty")
	}

	// Check that Run is set
	if cmd.Run == nil {
		t.Error("Expected Run to be set")
	}
}

func TestVersionCmd_DefaultVersion(t *testing.T) {
	t.Parallel()

	// Save original environment
	originalEnv := os.Getenv("ROCKETSHIP_VERSION")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("ROCKETSHIP_VERSION", originalEnv)
		} else {
			_ = os.Unsetenv("ROCKETSHIP_VERSION")
		}
	}()

	// Unset environment variable to test default behavior
	_ = os.Unsetenv("ROCKETSHIP_VERSION")

	cmd := NewVersionCmd()
	
	// Execute command - just check it doesn't error
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}
	
	// Output goes directly to os.Stdout, so we can't easily capture it
	// Just verify the command runs successfully
}

func TestVersionCmd_EnvironmentVersion(t *testing.T) {
	t.Parallel()

	// Save original environment
	originalEnv := os.Getenv("ROCKETSHIP_VERSION")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("ROCKETSHIP_VERSION", originalEnv)
		} else {
			_ = os.Unsetenv("ROCKETSHIP_VERSION")
		}
	}()

	// Set custom version
	customVersion := "v2.5.0-test"
	_ = os.Setenv("ROCKETSHIP_VERSION", customVersion)

	cmd := NewVersionCmd()
	
	// Execute command - just check it doesn't error
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}
	
	// Output goes directly to os.Stdout, so we can't easily capture it
	// Just verify the command runs successfully
}

func TestVersionCmd_EmptyEnvironmentVersion(t *testing.T) {
	t.Parallel()

	// Save original environment
	originalEnv := os.Getenv("ROCKETSHIP_VERSION")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("ROCKETSHIP_VERSION", originalEnv)
		} else {
			_ = os.Unsetenv("ROCKETSHIP_VERSION")
		}
	}()

	// Set empty version (should fall back to default)
	_ = os.Setenv("ROCKETSHIP_VERSION", "")

	cmd := NewVersionCmd()
	
	// Execute command - just check it doesn't error
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}
}

func TestVersionCmd_OutputFormat(t *testing.T) {
	t.Parallel()

	// Simple test - just verify command runs without error
	cmd := NewVersionCmd()
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}
}

func TestVersionCmd_ConcurrentExecution(t *testing.T) {
	t.Parallel()

	// Simple concurrent test
	var wg sync.WaitGroup
	errors := make(chan error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := NewVersionCmd()
			err := cmd.Execute()
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent execution failed: %v", err)
	}
}

func TestVersionCmd_ArgumentHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "no arguments",
			args: []string{},
		},
		{
			name: "with arguments (should be ignored)",
			args: []string{"extra", "args"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := NewVersionCmd()
			cmd.SetArgs(tt.args)
			
			err := cmd.Execute()
			if err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}
			
			// Output goes to stdout, just verify command runs successfully
		})
	}
}

func TestVersionCmd_Help(t *testing.T) {
	t.Parallel()

	cmd := NewVersionCmd()
	
	help := cmd.UsageString()
	if help == "" {
		t.Error("Expected non-empty help string")
	}

	// Check that help contains version command description
	if !strings.Contains(help, "version") {
		t.Error("Help should contain version command description")
	}
}

func TestVersionCmd_Structure(t *testing.T) {
	t.Parallel()

	cmd := NewVersionCmd()

	// Test command metadata
	if cmd.Use != "version" {
		t.Errorf("Expected Use to be 'version', got %s", cmd.Use)
	}

	// Test that the command has appropriate descriptions
	if !strings.Contains(cmd.Short, "version") {
		t.Error("Short description should mention version")
	}

	if !strings.Contains(cmd.Long, "version") {
		t.Error("Long description should mention version")
	}

	// Test that command is runnable
	if !cmd.Runnable() {
		t.Error("Version command should be runnable")
	}

	// Test that command has no subcommands
	if cmd.HasSubCommands() {
		t.Error("Version command should not have subcommands")
	}

	// Test that command has no flags
	if cmd.Flags().NFlag() > 0 {
		t.Error("Version command should not have flags")
	}
}

func TestVersionCmd_DefaultVersionValidation(t *testing.T) {
	t.Parallel()

	// Test that embedded.DefaultVersion is properly formatted
	if embedded.DefaultVersion == "" {
		t.Error("embedded.DefaultVersion should not be empty")
	}

	// Test that it follows semantic versioning pattern (starts with 'v')
	if len(embedded.DefaultVersion) < 2 || embedded.DefaultVersion[0] != 'v' {
		t.Errorf("embedded.DefaultVersion should start with 'v', got: %s", embedded.DefaultVersion)
	}
}

