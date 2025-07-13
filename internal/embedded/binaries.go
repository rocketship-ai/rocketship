package embedded

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	githubReleaseURL = "https://github.com/rocketship-ai/rocketship/releases/download/%s/%s"
	DefaultVersion   = "v0.5.8" // This should be updated with each release
)

type binaryMetadata struct {
	Version string `json:"version"`
}

// ExtractAndRun extracts a binary and runs it
func ExtractAndRun(name string, args []string, env []string) (*exec.Cmd, error) {
	// Always check for local development binary first
	localBinaryPath := filepath.Join("internal", "embedded", "bin", name)
	if stat, err := os.Stat(localBinaryPath); err == nil && stat.Mode()&0111 != 0 {
		// Use local binary in development mode
		cmd := exec.Command(localBinaryPath, args...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd, nil
	}

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

	// Path to the extracted binary and its metadata
	binaryPath := filepath.Join(rocketshipDir, name)
	metadataPath := binaryPath + ".json"

	// Check if we need to extract the binary
	needsExtract := true
	var targetVersion string

	// If binary exists, check its version
	if stat, err := os.Stat(binaryPath); err == nil && stat.Mode()&0111 != 0 {
		if metadata, err := loadMetadata(metadataPath); err == nil {
			// If ROCKETSHIP_VERSION is set, use that
			if envVersion := os.Getenv("ROCKETSHIP_VERSION"); envVersion != "" {
				targetVersion = envVersion
				needsExtract = metadata.Version != envVersion
			} else {
				// Otherwise use the current CLI's version and re-extract if cached version differs
				targetVersion = DefaultVersion
				needsExtract = metadata.Version != DefaultVersion
			}
		}
	}

	// If no existing version (first install), use defaultVersion
	if targetVersion == "" {
		targetVersion = DefaultVersion
	}

	// Extract the binary if needed
	if needsExtract {
		// Determine platform-specific binary name
		binaryName := fmt.Sprintf("%s-%s-%s", name, runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}

		// Download the binary from GitHub releases
		url := fmt.Sprintf(githubReleaseURL, targetVersion, binaryName)
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

		// Save metadata with the version we just downloaded
		if err := saveMetadata(metadataPath, targetVersion); err != nil {
			return nil, fmt.Errorf("failed to save binary metadata: %w", err)
		}
	}

	// Create the command
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func loadMetadata(path string) (*binaryMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var metadata binaryMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func saveMetadata(path string, version string) error {
	metadata := binaryMetadata{
		Version: version,
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
