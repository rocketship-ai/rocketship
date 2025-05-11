package embedded

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	githubReleaseURL = "https://github.com/rocketship-ai/rocketship/releases/download/v%s/%s"
)

// ExtractAndRun extracts a binary from GitHub releases and runs it
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
		// Determine platform-specific binary name
		binaryName := fmt.Sprintf("%s-%s-%s", name, runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}

		// Download the binary from GitHub releases
		url := fmt.Sprintf(githubReleaseURL, "0.1.2", binaryName)
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to download binary %s: %w", name, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download binary %s: HTTP %d", name, resp.StatusCode)
		}

		// Create the binary file
		f, err := os.OpenFile(binaryPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create binary file: %w", err)
		}

		// Write the binary data and handle errors, including close
		_, copyErr := io.Copy(f, resp.Body)
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
