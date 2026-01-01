package executors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rocketship-ai/rocketship/internal/plugins/script/runtime"
)

func TestShellExecutor_Language(t *testing.T) {
	executor := NewShellExecutor()
	if executor.Language() != "shell" {
		t.Errorf("Expected language 'shell', got '%s'", executor.Language())
	}
}

func TestShellExecutor_ValidateScript(t *testing.T) {
	executor := NewShellExecutor()

	tests := []struct {
		name    string
		script  string
		wantErr bool
	}{
		{
			name:    "valid script",
			script:  "echo 'hello world'",
			wantErr: false,
		},
		{
			name:    "empty script",
			script:  "",
			wantErr: true,
		},
		{
			name:    "whitespace only script",
			script:  "   \n\t  ",
			wantErr: true,
		},
		{
			name:    "complex script",
			script:  "#!/bin/bash\necho 'test'\nls -la",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ValidateScript(tt.script)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShellExecutor_Execute_BasicCommands(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()

	tests := []struct {
		name             string
		script           string
		expectedExitCode string
		checkStdout      func(string) bool
		checkStderr      func(string) bool
		shouldFail       bool
	}{
		{
			name:             "simple echo",
			script:           "echo 'hello world'",
			expectedExitCode: "0",
			checkStdout:      func(s string) bool { return strings.Contains(s, "hello world") },
			checkStderr:      func(s string) bool { return s == "" },
			shouldFail:       false,
		},
		{
			name:             "command that fails",
			script:           "exit 42",
			expectedExitCode: "42",
			checkStdout:      func(s string) bool { return s == "" },
			shouldFail:       true,
		},
		{
			name:             "command with stderr",
			script:           "echo 'error message' >&2",
			expectedExitCode: "0",
			checkStdout:      func(s string) bool { return s == "" },
			checkStderr:      func(s string) bool { return strings.Contains(s, "error message") },
			shouldFail:       false,
		},
		{
			name:             "multiline script",
			script:           "echo 'line 1'\necho 'line 2'",
			expectedExitCode: "0",
			checkStdout: func(s string) bool {
				return strings.Contains(s, "line 1") && strings.Contains(s, "line 2")
			},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rtCtx := runtime.NewContext(make(map[string]string), make(map[string]interface{}), make(map[string]string))

			err := executor.Execute(ctx, tt.script, rtCtx)

			if tt.shouldFail && err == nil {
				t.Error("Expected execution to fail, but it succeeded")
				return
			}

			if !tt.shouldFail && err != nil {
				t.Errorf("Expected execution to succeed, but it failed: %v", err)
				return
			}

			// Check saved values
			if exitCode, exists := rtCtx.Saved["exit_code"]; exists {
				if exitCode != tt.expectedExitCode {
					t.Errorf("Expected exit code %s, got %s", tt.expectedExitCode, exitCode)
				}
			} else {
				t.Error("Exit code not saved")
			}

			if stdout, exists := rtCtx.Saved["stdout"]; exists && tt.checkStdout != nil {
				if !tt.checkStdout(stdout) {
					t.Errorf("Stdout check failed. Stdout: %s", stdout)
				}
			}

			if stderr, exists := rtCtx.Saved["stderr"]; exists && tt.checkStderr != nil {
				if !tt.checkStderr(stderr) {
					t.Errorf("Stderr check failed. Stderr: %s", stderr)
				}
			}

			// Check that duration is saved
			if _, exists := rtCtx.Saved["duration"]; !exists {
				t.Error("Duration not saved")
			}
		})
	}
}

func TestShellExecutor_Execute_VariableSubstitution(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()

	// Set up runtime context with state and variables
	state := map[string]string{
		"test_var": "hello",
		"number":   "42",
	}
	vars := map[string]interface{}{
		"config_var": "world",
		"env_name":   "testing",
	}

	rtCtx := runtime.NewContext(state, vars, make(map[string]string))

	script := `
		echo "State: {{ test_var }}"
		echo "Config: {{ .vars.config_var }}"
		echo "Number: {{ number }}"
		echo "Env: {{ .vars.env_name }}"
	`

	err := executor.Execute(ctx, script, rtCtx)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	stdout := rtCtx.Saved["stdout"]

	expectedStrings := []string{
		"State: hello",
		"Config: world",
		"Number: 42",
		"Env: testing",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(stdout, expected) {
			t.Errorf("Expected stdout to contain '%s', but got: %s", expected, stdout)
		}
	}
}

func TestShellExecutor_Execute_EnvironmentVariables(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()

	// Set up runtime context
	state := map[string]string{
		"test_value": "state_value",
	}
	vars := map[string]interface{}{
		"config_value": "var_value",
	}

	rtCtx := runtime.NewContext(state, vars, make(map[string]string))

	script := `
		echo "State env: $ROCKETSHIP_TEST_VALUE"
		echo "Config env: $ROCKETSHIP_VAR_CONFIG_VALUE"
		echo "Environment check complete"
	`

	err := executor.Execute(ctx, script, rtCtx)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	stdout := rtCtx.Saved["stdout"]

	expectedStrings := []string{
		"State env: state_value",
		"Config env: var_value",
		"Environment check complete",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(stdout, expected) {
			t.Errorf("Expected stdout to contain '%s', but got: %s", expected, stdout)
		}
	}
}

func TestShellExecutor_Execute_WorkingDirectory(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()
	rtCtx := runtime.NewContext(make(map[string]string), make(map[string]interface{}), make(map[string]string))

	// Get current working directory
	expectedWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	script := "pwd"

	err = executor.Execute(ctx, script, rtCtx)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	stdout := strings.TrimSpace(rtCtx.Saved["stdout"])
	if stdout != expectedWd {
		t.Errorf("Expected working directory %s, got %s", expectedWd, stdout)
	}
}

func TestShellExecutor_Execute_FileOperations(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()
	rtCtx := runtime.NewContext(make(map[string]string), make(map[string]interface{}), make(map[string]string))

	// Use os.TempDir() for better cross-platform compatibility
	tempDir := os.TempDir()
	tempFile := fmt.Sprintf("%s/rocketship_test.txt", tempDir)
	testContent := "test content from rocketship"

	script := fmt.Sprintf(`
		echo '%s' > '%s'
		cat '%s'
		rm '%s'
	`, testContent, tempFile, tempFile, tempFile)

	err := executor.Execute(ctx, script, rtCtx)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	stdout := rtCtx.Saved["stdout"]
	if !strings.Contains(stdout, testContent) {
		t.Errorf("Expected stdout to contain '%s', but got: %s", testContent, stdout)
	}

	// Verify file was cleaned up
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Temporary file was not cleaned up")
	}
}

func TestShellExecutor_commandExists(t *testing.T) {
	executor := NewShellExecutor()

	// Test with a command that should exist on most systems
	if !executor.commandExists("echo") {
		t.Error("Expected 'echo' command to exist")
	}

	// Test with a command that should not exist
	if executor.commandExists("rocketship_nonexistent_command_12345") {
		t.Error("Expected non-existent command to return false")
	}
}

func TestShellExecutor_processVariables(t *testing.T) {
	executor := NewShellExecutor()

	state := map[string]string{
		"user_id": "123",
		"token":   "abc123",
	}
	vars := map[string]interface{}{
		"api_url": "https://api.example.com",
		"timeout": 30,
	}

	rtCtx := runtime.NewContext(state, vars, make(map[string]string))

	script := `
		curl -H "Authorization: Bearer {{ token }}" \
		     -d "user_id={{ user_id }}" \
		     "{{ .vars.api_url }}/users" \
		     --timeout {{ .vars.timeout }}
	`

	processed, err := executor.processVariables(script, rtCtx)
	if err != nil {
		t.Fatalf("Variable processing failed: %v", err)
	}

	expectedSubstitutions := map[string]string{
		"{{ token }}":         "abc123",
		"{{ user_id }}":       "123",
		"{{ .vars.api_url }}": "https://api.example.com",
		"{{ .vars.timeout }}": "30",
	}

	for placeholder, expected := range expectedSubstitutions {
		if strings.Contains(processed, placeholder) {
			t.Errorf("Placeholder '%s' was not replaced", placeholder)
		}
		if !strings.Contains(processed, expected) {
			t.Errorf("Expected processed script to contain '%s'", expected)
		}
	}
}
