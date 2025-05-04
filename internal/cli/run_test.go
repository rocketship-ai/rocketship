package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRunCmd(t *testing.T) {
	cmd := NewRunCmd()

	// Test command name and description
	assert.Equal(t, "run [test-file]", cmd.Use)
	assert.Equal(t, "Run a test file", cmd.Short)
	assert.Contains(t, cmd.Long, "The test file should be a YAML file containing the test definition")

	// Test argument validation
	err := cmd.Args(cmd, []string{})
	assert.Error(t, err, "should require exactly one argument")

	err = cmd.Args(cmd, []string{"test1", "test2"})
	assert.Error(t, err, "should not accept more than one argument")

	err = cmd.Args(cmd, []string{"test.yaml"})
	assert.NoError(t, err, "should accept exactly one argument")
} 