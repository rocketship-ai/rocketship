package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewStartCmd(t *testing.T) {
	t.Parallel()

	cmd := NewStartCmd()
	if cmd == nil {
		t.Fatal("NewStartCmd returned nil")
	}

	if cmd.Use != "start" {
		t.Errorf("Expected Use to be 'start', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be non-empty")
	}

	// Check that it has subcommands
	if !cmd.HasSubCommands() {
		t.Error("Expected start command to have subcommands")
	}

	// Check for server subcommand
	serverCmd, _, err := cmd.Find([]string{"server"})
	if err != nil {
		t.Errorf("Failed to find server subcommand: %v", err)
	}
	if serverCmd == nil {
		t.Error("Server subcommand should not be nil")
	}
}

func TestNewStartServerCmd(t *testing.T) {
	t.Parallel()

	cmd := newStartServerCmd()
	if cmd == nil {
		t.Fatal("newStartServerCmd returned nil")
	}

	if cmd.Use != "server" {
		t.Errorf("Expected Use to be 'server', got %s", cmd.Use)
	}

	// Check flags - local flag should not exist anymore
	localFlag := cmd.Flag("local")
	if localFlag != nil {
		t.Error("Local flag should not exist anymore")
	}

	backgroundFlag := cmd.Flag("background")
	if backgroundFlag == nil {
		t.Error("Expected background flag to exist")
	} else if backgroundFlag.Shorthand != "b" {
		t.Errorf("Expected background flag shorthand to be 'b', got %s", backgroundFlag.Shorthand)
	}
}

func TestStartServerCmd_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "local server startup fails",
			args:        []string{},
			wantErr:     true,
			errContains: "failed to", // Now it should try to start local server and fail due to missing dependencies
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newStartServerCmd()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWaitForEngine(t *testing.T) {
	t.Parallel()

	// Skip this test as it tries to connect to actual engine
	t.Skip("Skipping test that requires actual engine connection")
}

func TestWaitForEngine_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Skip this test as it tries to connect to actual engine
	t.Skip("Skipping test that requires actual engine connection")
}

func TestWaitForEngine_Concurrent(t *testing.T) {
	t.Parallel()

	// Skip this test as it tries to connect to actual engine
	t.Skip("Skipping test that requires actual engine connection")
}

func TestSetupLocalServer_MockComponents(t *testing.T) {
	t.Parallel()

	// This test is limited because setupLocalServer has many dependencies
	// In a real implementation, we'd need to mock the process manager and server config
	
	// Test that the function exists (can't actually test it without mocking dependencies)
	// setupLocalServer exists and is callable
	_ = setupLocalServer
}

func TestSetupLocalServerBackground_MockComponents(t *testing.T) {
	t.Parallel()

	// This test is limited because setupLocalServerBackground has many dependencies
	// In a real implementation, we'd need to mock the process manager and server config
	
	// Test that the function exists (can't actually test it without mocking dependencies)
	// setupLocalServerBackground exists and is callable
	_ = setupLocalServerBackground
}

// Test the command parsing and flag handling
func TestStartServerCmd_FlagParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           []string
		expectedBg     bool
		expectParseErr bool
	}{
		{
			name:       "no flags",
			args:       []string{},
			expectedBg: false,
		},
		{
			name:       "background flag short",
			args:       []string{"-b"},
			expectedBg: true,
		},
		{
			name:       "background flag long",
			args:       []string{"--background"},
			expectedBg: true,
		},
		{
			name:           "invalid local flag short",
			args:           []string{"-l"},
			expectParseErr: true,
		},
		{
			name:           "invalid local flag long",
			args:           []string{"--local"},
			expectParseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newStartServerCmd()
			cmd.SetArgs(tt.args)

			// Parse flags without executing
			err := cmd.ParseFlags(tt.args)
			if tt.expectParseErr {
				if err == nil {
					t.Error("Expected parse error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected parse error: %v", err)
			}

			// Check background flag value
			bgFlag, err := cmd.Flags().GetBool("background")
			if err != nil {
				t.Fatalf("Failed to get background flag: %v", err)
			}

			if bgFlag != tt.expectedBg {
				t.Errorf("Expected background flag to be %v, got %v", tt.expectedBg, bgFlag)
			}
		})
	}
}

// Test help and usage output
func TestStartServerCmd_Help(t *testing.T) {
	t.Parallel()

	cmd := newStartServerCmd()
	
	help := cmd.UsageString()
	if help == "" {
		t.Error("Expected non-empty help string")
	}

	// Check that help does not contain local flag (it's been removed)
	if contains(help, "local") {
		t.Error("Help should not contain local flag description")
	}

	if !contains(help, "background") {
		t.Error("Help should contain background flag description")
	}
}

// Test command structure and relationships
func TestStartCmd_Structure(t *testing.T) {
	t.Parallel()

	startCmd := NewStartCmd()
	
	// Test command hierarchy
	if !startCmd.HasSubCommands() {
		t.Error("Start command should have subcommands")
	}

	commands := startCmd.Commands()
	if len(commands) == 0 {
		t.Error("Start command should have at least one subcommand")
	}

	// Find server subcommand
	var serverCmd *cobra.Command
	for _, cmd := range commands {
		if cmd.Use == "server" {
			serverCmd = cmd
			break
		}
	}

	if serverCmd == nil {
		t.Error("Start command should have a server subcommand")
	}

	// Test that server command does not have local flag (it's been removed)
	if serverCmd.Flag("local") != nil {
		t.Error("Server command should not have local flag")
	}

	if serverCmd.Flag("background") == nil {
		t.Error("Server command should have background flag")
	}
}

// Benchmark tests


