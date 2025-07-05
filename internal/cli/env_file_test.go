package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnvFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "rocketship-env-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	t.Run("valid env file", func(t *testing.T) {
		// Create a test .env file
		envPath := filepath.Join(tmpDir, ".env")
		content := `# This is a comment
API_KEY=test-api-key-123
DATABASE_URL=postgres://user:pass@localhost/db

# Another comment
EMPTY_VALUE=
QUOTED_VALUE="value with spaces"
SINGLE_QUOTED='single quoted value'
NO_QUOTES=simple_value
`
		err := os.WriteFile(envPath, []byte(content), 0644)
		require.NoError(t, err)

		// Load the env file
		env, err := loadEnvFile(envPath)
		require.NoError(t, err)

		// Verify the loaded values
		assert.Equal(t, "test-api-key-123", env["API_KEY"])
		assert.Equal(t, "postgres://user:pass@localhost/db", env["DATABASE_URL"])
		assert.Equal(t, "", env["EMPTY_VALUE"])
		assert.Equal(t, "value with spaces", env["QUOTED_VALUE"])
		assert.Equal(t, "single quoted value", env["SINGLE_QUOTED"])
		assert.Equal(t, "simple_value", env["NO_QUOTES"])
	})

	t.Run("invalid format", func(t *testing.T) {
		// Create a test .env file with invalid format
		envPath := filepath.Join(tmpDir, ".env.invalid")
		content := `VALID_KEY=value
INVALID_LINE_NO_EQUALS
ANOTHER_VALID=test
`
		err := os.WriteFile(envPath, []byte(content), 0644)
		require.NoError(t, err)

		// Try to load the env file
		_, err = loadEnvFile(envPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid line 2")
	})

	t.Run("empty key", func(t *testing.T) {
		// Create a test .env file with empty key
		envPath := filepath.Join(tmpDir, ".env.empty")
		content := `VALID=value
=empty_key_value
`
		err := os.WriteFile(envPath, []byte(content), 0644)
		require.NoError(t, err)

		// Try to load the env file
		_, err = loadEnvFile(envPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty key")
	})

	t.Run("non-existent file", func(t *testing.T) {
		// Try to load a non-existent file
		_, err := loadEnvFile(filepath.Join(tmpDir, "non-existent.env"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open env file")
	})
}

func TestSetEnvironmentVariables(t *testing.T) {
	// Save original env values
	originalValues := make(map[string]string)
	testKeys := []string{"TEST_ENV_VAR_1", "TEST_ENV_VAR_2"}
	for _, key := range testKeys {
		originalValues[key] = os.Getenv(key)
	}

	// Clean up after test
	defer func() {
		for key, value := range originalValues {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
	}()

	// Test setting environment variables
	env := map[string]string{
		"TEST_ENV_VAR_1": "value1",
		"TEST_ENV_VAR_2": "value2",
	}

	err := setEnvironmentVariables(env)
	require.NoError(t, err)

	// Verify the environment variables were set
	assert.Equal(t, "value1", os.Getenv("TEST_ENV_VAR_1"))
	assert.Equal(t, "value2", os.Getenv("TEST_ENV_VAR_2"))
}