package embedded

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed bin/worker bin/engine
var binaries embed.FS

// ExtractAndRun extracts a binary from the embedded filesystem and runs it
func ExtractAndRun(name string, args []string, env []string) (*exec.Cmd, error) {
	// Get the temporary directory
	tempDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache directory: %w", err)
	}
	rocketshipDir := filepath.Join(tempDir, "rocketship")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(rocketshipDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Path to the extracted binary
	binaryPath := filepath.Join(rocketshipDir, name)

	// Check if we need to extract the binary
	needsExtract := true
	if stat, err := os.Stat(binaryPath); err == nil {
		if stat.Mode()&0111 != 0 { // Check if executable
			needsExtract = false
		}
	}

	// Extract the binary if needed
	if needsExtract {
		// Read the embedded binary
		data, err := binaries.ReadFile(fmt.Sprintf("bin/%s", name))
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded binary %s: %w", name, err)
		}

		// Create the binary file
		f, err := os.OpenFile(binaryPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create binary file: %w", err)
		}

		// Write the binary data and handle errors, including close
		_, copyErr := io.Copy(f, bytes.NewReader(data))
		closeErr := f.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("failed to write binary data: %w", copyErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close binary file: %w", closeErr)
		}
	}

	// Create the command
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}
