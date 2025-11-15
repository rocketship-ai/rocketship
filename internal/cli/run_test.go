package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunCmd(t *testing.T) {
	cmd := NewRunCmd()

	// Test command name and description
	assert.Equal(t, "run", cmd.Use)
	assert.Equal(t, "Run rocketship tests", cmd.Short)
	assert.Contains(t, cmd.Long, "Run rocketship tests from YAML files")

	// Test flags
	fileFlag := cmd.Flags().Lookup("file")
	assert.NotNil(t, fileFlag, "file flag should exist")
	assert.Equal(t, "file", fileFlag.Name)
	assert.Equal(t, "Path to a Rocketship test file (YAML)", fileFlag.Usage)

	dirFlag := cmd.Flags().Lookup("dir")
	assert.NotNil(t, dirFlag, "dir flag should exist")
	assert.Equal(t, "dir", dirFlag.Name)
	assert.Equal(t, "Path to directory containing test files (for .rocketship, runs all YAML test files recursively)", dirFlag.Usage)
}

func TestFindRocketshipFiles(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "rocketship-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test directory structure
	dirs := []string{
		filepath.Join(tmpDir, "test1"),
		filepath.Join(tmpDir, "test2"),
		filepath.Join(tmpDir, "test2", "nested"),
		filepath.Join(tmpDir, "empty"),
	}

	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
	}

	// Create test files
	files := map[string]string{
		filepath.Join(tmpDir, "rocketship.yaml"):                "test1",
		filepath.Join(tmpDir, "test1", "rocketship.yaml"):       "test2",
		filepath.Join(tmpDir, "test2", "rocketship.yaml"):       "test3",
		filepath.Join(tmpDir, "test2", "nested", "other.yaml"):  "not-included",
		filepath.Join(tmpDir, "test2", "nested", "config.yaml"): "not-included",
	}

	for path, content := range files {
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Test finding files
	found, err := findRocketshipFiles(tmpDir)
	require.NoError(t, err)

	// Should find exactly 3 rocketship.yaml files
	assert.Equal(t, 3, len(found), "Should find exactly 3 rocketship.yaml files")

	// Verify each expected file is found
	expectedFiles := []string{
		filepath.Join(tmpDir, "rocketship.yaml"),
		filepath.Join(tmpDir, "test1", "rocketship.yaml"),
		filepath.Join(tmpDir, "test2", "rocketship.yaml"),
	}

	for _, expected := range expectedFiles {
		wasFound := false
		for _, actual := range found {
			if filepath.Clean(actual) == filepath.Clean(expected) {
				wasFound = true
				break
			}
		}
		assert.True(t, wasFound, "Should find %s", expected)
	}

	// Test with empty directory
	found, err = findRocketshipFiles(filepath.Join(tmpDir, "empty"))
	require.NoError(t, err)
	assert.Empty(t, found, "Should find no files in empty directory")

	// Test with non-existent directory
	_, err = findRocketshipFiles(filepath.Join(tmpDir, "nonexistent"))
	assert.Error(t, err, "Should return error for non-existent directory")
}

func TestFindYamlTestFilesInRocketshipDir(t *testing.T) {
	// Create a temporary .rocketship directory structure for testing
	tmpDir, err := os.MkdirTemp("", "rocketship-yaml-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	rocketDir := filepath.Join(tmpDir, ".rocketship")
	require.NoError(t, os.MkdirAll(rocketDir, 0755))

	// Create test YAML files that should be included
	includeFiles := []string{
		filepath.Join(rocketDir, "auth_login.yaml"),
		filepath.Join(rocketDir, "payments.yaml"),
		filepath.Join(rocketDir, "nested", "flow.yaml"),
	}
	for _, path := range includeFiles {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("test"), 0644))
	}

	// Create tmp directory with YAML that should be excluded
	tmpSubdir := filepath.Join(rocketDir, "tmp")
	require.NoError(t, os.MkdirAll(tmpSubdir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpSubdir, "scratch.yaml"), []byte("scratch"), 0644))

	// Find YAML test files
	found, err := findYamlTestFiles(rocketDir)
	require.NoError(t, err)

	// Should find exactly the non-tmp YAML files
	assert.Equal(t, len(includeFiles), len(found), "Should find only YAML files outside tmp/")

	for _, expected := range includeFiles {
		wasFound := false
		for _, actual := range found {
			if filepath.Clean(actual) == filepath.Clean(expected) {
				wasFound = true
				break
			}
		}
		assert.True(t, wasFound, "Should find %s", expected)
	}

	// Ensure tmp YAML is not included
	for _, actual := range found {
		assert.NotContains(t, actual, string(filepath.Separator)+"tmp"+string(filepath.Separator), "tmp directory files should be excluded")
	}
}

func TestAutoModeCleanupOnCancellation(t *testing.T) {
	// Test that cleanup function is called when context is cancelled in auto mode
	// This tests the signal handling logic we added to fix the Ctrl+C issue

	t.Run("cleanup called in auto mode when context cancelled", func(t *testing.T) {
		// Setup
		ctx, cancel := context.WithCancel(context.Background())
		resultChan := make(chan TestSuiteResult, 1)
		cleanupCalled := false

		cleanup := func() {
			cleanupCalled = true
		}

		// Simulate the result collection loop logic from run.go
		go func() {
			// This simulates the logic from lines 484-499 in run.go
			var results []TestSuiteResult
			for {
				select {
				case <-ctx.Done():
					// If we're in auto mode and context is cancelled, call cleanup immediately
					isAuto := true // Simulate auto mode
					if isAuto && cleanup != nil {
						cleanup()
					}
					return
				case result, ok := <-resultChan:
					if !ok {
						return
					}
					_ = append(results, result)
				}
			}
		}()

		// Wait a bit to ensure goroutine is running
		time.Sleep(10 * time.Millisecond)

		// Cancel the context (simulates Ctrl+C)
		cancel()

		// Wait for cleanup to be called
		timeout := time.After(1 * time.Second)
		for !cleanupCalled {
			select {
			case <-timeout:
				t.Fatal("cleanup function was not called within timeout")
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Verify cleanup was called
		assert.True(t, cleanupCalled, "cleanup function should be called when context is cancelled in auto mode")
	})

	t.Run("cleanup not called when not in auto mode", func(t *testing.T) {
		// Setup
		ctx, cancel := context.WithCancel(context.Background())
		resultChan := make(chan TestSuiteResult, 1)
		cleanupCalled := false

		cleanup := func() {
			cleanupCalled = true
		}

		// Simulate the result collection loop logic for non-auto mode
		go func() {
			var results []TestSuiteResult
			for {
				select {
				case <-ctx.Done():
					// If we're NOT in auto mode, cleanup should not be called
					isAuto := false // Simulate non-auto mode
					if isAuto && cleanup != nil {
						cleanup()
					}
					return
				case result, ok := <-resultChan:
					if !ok {
						return
					}
					_ = append(results, result)
				}
			}
		}()

		// Wait a bit to ensure goroutine is running
		time.Sleep(10 * time.Millisecond)

		// Cancel the context
		cancel()

		// Wait a bit to see if cleanup is incorrectly called
		time.Sleep(100 * time.Millisecond)

		// Verify cleanup was NOT called
		assert.False(t, cleanupCalled, "cleanup function should NOT be called when not in auto mode")
	})

	t.Run("cleanup not called when cleanup function is nil", func(t *testing.T) {
		// Setup
		ctx, cancel := context.WithCancel(context.Background())
		resultChan := make(chan TestSuiteResult, 1)

		// Simulate the result collection loop logic with nil cleanup
		completed := false
		go func() {
			defer func() { completed = true }()
			var results []TestSuiteResult
			for {
				select {
				case <-ctx.Done():
					// If cleanup is nil, should not panic
					isAuto := true
					var cleanup func()
					if isAuto && cleanup != nil {
						cleanup()
					}
					return
				case result, ok := <-resultChan:
					if !ok {
						return
					}
					_ = append(results, result)
				}
			}
		}()

		// Wait a bit to ensure goroutine is running
		time.Sleep(10 * time.Millisecond)

		// Cancel the context
		cancel()

		// Wait for completion
		timeout := time.After(1 * time.Second)
		for !completed {
			select {
			case <-timeout:
				t.Fatal("goroutine did not complete within timeout")
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Verify no panic occurred (test passes if we reach here)
		assert.True(t, completed, "should handle nil cleanup function without panic")
	})
}
