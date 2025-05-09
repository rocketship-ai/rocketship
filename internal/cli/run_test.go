package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRunCmd(t *testing.T) {
	cmd := NewRunCmd()

	// Test command name and description
	assert.Equal(t, "run", cmd.Use)
	assert.Equal(t, "Run a rocketship test", cmd.Short)
	assert.Contains(t, cmd.Long, "Run a rocketship test from a YAML file.")

	// Test flag
	fileFlag := cmd.Flags().Lookup("file")
	assert.NotNil(t, fileFlag, "file flag should exist")
	assert.Equal(t, "file", fileFlag.Name)
	assert.Equal(t, "Path to the test file (default: rocketship.yaml in current directory)", fileFlag.Usage)
}
