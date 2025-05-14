package cli

import (
	"os"
	"path/filepath"
	"testing"

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
	assert.Equal(t, "Path to a single test file (default: rocketship.yaml in current directory)", fileFlag.Usage)

	dirFlag := cmd.Flags().Lookup("dir")
	assert.NotNil(t, dirFlag, "dir flag should exist")
	assert.Equal(t, "dir", dirFlag.Name)
	assert.Equal(t, "Path to directory containing test files (will run all rocketship.yaml files recursively)", dirFlag.Usage)
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
