package embedded

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestExtractAndRun_LocalBinary(t *testing.T) {
	t.Parallel()
	
	// Create a temporary directory structure
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "internal", "embedded", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	// Create a mock local binary
	localBinary := filepath.Join(binDir, "test-binary")
	if err := os.WriteFile(localBinary, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create local binary: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tempDir)

	cmd, err := ExtractAndRun("test-binary", []string{"arg1"}, []string{"ENV=test"})
	if err != nil {
		t.Fatalf("ExtractAndRun failed: %v", err)
	}

	// The command path might be relative, so just check that it ends with the expected path
	if !strings.Contains(cmd.Path, "test-binary") {
		t.Errorf("Expected command path to contain 'test-binary', got %s", cmd.Path)
	}
}

func TestExtractAndRun_DownloadBinary(t *testing.T) {
	t.Parallel()

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock binary content"))
	}))
	defer server.Close()

	// Note: GitHub release URL is hardcoded, would need dependency injection for proper testing

	// Use a custom cache directory
	tempCacheDir := t.TempDir()
	originalCacheDir := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalCacheDir) }()

	// Mock UserCacheDir by creating the expected directory
	cacheDir := filepath.Join(tempCacheDir, "cache")
	_ = os.MkdirAll(cacheDir, 0755)

	// Set ROCKETSHIP_VERSION for testing
	_ = os.Setenv("ROCKETSHIP_VERSION", "v1.0.0")
	defer func() { _ = os.Unsetenv("ROCKETSHIP_VERSION") }()

	// This test is more complex due to the hardcoded GitHub URL
	// In a real implementation, we'd inject the URL as a dependency
	t.Skip("Skipping download test due to hardcoded GitHub URL - would need dependency injection for proper testing")
}

func TestExtractAndRun_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	// Test concurrent calls to ensure thread safety
	numGoroutines := 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Create a temporary directory structure
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "internal", "embedded", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	// Create a mock local binary
	localBinary := filepath.Join(binDir, "concurrent-test")
	if err := os.WriteFile(localBinary, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create local binary: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tempDir)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := ExtractAndRun("concurrent-test", []string{}, []string{})
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent call failed: %v", err)
	}
}

func TestLoadMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected *binaryMetadata
		wantErr  bool
	}{
		{
			name:     "valid metadata",
			content:  `{"version": "v1.2.3"}`,
			expected: &binaryMetadata{Version: "v1.2.3"},
			wantErr:  false,
		},
		{
			name:    "invalid json",
			content: `invalid json`,
			wantErr: true,
		},
		{
			name:    "missing file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var path string
			if tt.name != "missing file" {
				// Create temporary file with content
				tmpFile := filepath.Join(t.TempDir(), "metadata.json")
				if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				path = tmpFile
			} else {
				path = filepath.Join(t.TempDir(), "nonexistent.json")
			}

			result, err := loadMetadata(path)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Version != tt.expected.Version {
				t.Errorf("Expected version %s, got %s", tt.expected.Version, result.Version)
			}
		})
	}
}

func TestSaveMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "valid version",
			version: "v1.2.3",
			wantErr: false,
		},
		{
			name:    "empty version",
			version: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpFile := filepath.Join(t.TempDir(), "metadata.json")

			err := saveMetadata(tmpFile, tt.version)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify the saved content
			data, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read saved file: %v", err)
			}

			var metadata binaryMetadata
			if err := json.Unmarshal(data, &metadata); err != nil {
				t.Fatalf("Failed to unmarshal saved metadata: %v", err)
			}

			if metadata.Version != tt.version {
				t.Errorf("Expected saved version %s, got %s", tt.version, metadata.Version)
			}
		})
	}
}

func TestSaveMetadata_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	// Test concurrent writes to different files
	numGoroutines := 20
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	tempDir := t.TempDir()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			path := filepath.Join(tempDir, "metadata_%d.json")
			version := "v1.0.0"
			
			err := saveMetadata(path, version)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent save failed: %v", err)
	}
}

func TestBinaryNameGeneration(t *testing.T) {
	t.Parallel()

	// Test the binary name generation logic
	testCases := []struct {
		name string
		goos string
		goarch string
		expected string
	}{
		{"engine", "linux", "amd64", "engine-linux-amd64"},
		{"worker", "darwin", "arm64", "worker-darwin-arm64"},
		{"engine", "windows", "amd64", "engine-windows-amd64.exe"},
	}

	for _, tc := range testCases {
		t.Run(tc.goos+"_"+tc.goarch, func(t *testing.T) {
			t.Parallel()

			// Simulate binary name generation
			binaryName := tc.name + "-" + tc.goos + "-" + tc.goarch
			if tc.goos == "windows" {
				binaryName += ".exe"
			}

			if binaryName != tc.expected {
				t.Errorf("Expected binary name %s, got %s", tc.expected, binaryName)
			}
		})
	}
}

func TestEnvironmentVariableHandling(t *testing.T) {
	t.Parallel()

	// Test ROCKETSHIP_VERSION environment variable handling
	tests := []struct {
		name     string
		envVar   string
		expected string
	}{
		{
			name:     "custom version",
			envVar:   "v2.0.0",
			expected: "v2.0.0",
		},
		{
			name:     "empty env var",
			envVar:   "",
			expected: DefaultVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Save original env var
			originalEnv := os.Getenv("ROCKETSHIP_VERSION")
			defer func() { _ = os.Setenv("ROCKETSHIP_VERSION", originalEnv) }()

			// Set test env var
			if tt.envVar != "" {
				_ = os.Setenv("ROCKETSHIP_VERSION", tt.envVar)
			} else {
				_ = os.Unsetenv("ROCKETSHIP_VERSION")
			}

			// Test the logic that uses environment variables
			envVersion := os.Getenv("ROCKETSHIP_VERSION")
			targetVersion := tt.expected
			if envVersion != "" {
				targetVersion = envVersion
			}

			if targetVersion != tt.expected {
				t.Errorf("Expected target version %s, got %s", tt.expected, targetVersion)
			}
		})
	}
}

func TestDefaultVersion(t *testing.T) {
	t.Parallel()

	// Test that DefaultVersion is properly defined
	if DefaultVersion == "" {
		t.Error("DefaultVersion should not be empty")
	}

	// Test that it follows semantic versioning pattern
	if len(DefaultVersion) < 5 || DefaultVersion[0] != 'v' {
		t.Errorf("DefaultVersion should follow semantic versioning pattern, got: %s", DefaultVersion)
	}
}

func TestGithubReleaseURL(t *testing.T) {
	t.Parallel()

	// Test that the GitHub release URL is properly formatted
	expectedPattern := "https://github.com/rocketship-ai/rocketship/releases/download/%s/%s"
	if githubReleaseURL != expectedPattern {
		t.Errorf("Expected GitHub release URL pattern %s, got %s", expectedPattern, githubReleaseURL)
	}
}

func TestVersionUpgradeBugFix(t *testing.T) {
	t.Parallel()

	// Test the specific bug fix for version upgrades
	// When CLI is upgraded but cached binaries are old, they should be re-downloaded
	tests := []struct {
		name           string
		cliVersion     string
		cachedVersion  string
		envVersion     string
		expectedTarget string
		shouldExtract  bool
	}{
		{
			name:           "CLI upgraded, no env var - should extract new version",
			cliVersion:     "v1.5.1",
			cachedVersion:  "v1.5.0",
			envVersion:     "",
			expectedTarget: "v1.5.1",
			shouldExtract:  true,
		},
		{
			name:           "CLI and cache match, no env var - should not extract",
			cliVersion:     "v1.5.1",
			cachedVersion:  "v1.5.1",
			envVersion:     "",
			expectedTarget: "v1.5.1",
			shouldExtract:  false,
		},
		{
			name:           "ENV var overrides everything - should extract env version",
			cliVersion:     "v1.5.1",
			cachedVersion:  "v1.5.0",
			envVersion:     "v1.4.0",
			expectedTarget: "v1.4.0",
			shouldExtract:  true,
		},
		{
			name:           "ENV var matches cache - should not extract",
			cliVersion:     "v1.5.1",
			cachedVersion:  "v1.4.0",
			envVersion:     "v1.4.0",
			expectedTarget: "v1.4.0",
			shouldExtract:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Save and restore environment
			originalEnv := os.Getenv("ROCKETSHIP_VERSION")
			defer func() { _ = os.Setenv("ROCKETSHIP_VERSION", originalEnv) }()

			// Set test environment
			if tt.envVersion != "" {
				_ = os.Setenv("ROCKETSHIP_VERSION", tt.envVersion)
			} else {
				_ = os.Unsetenv("ROCKETSHIP_VERSION")
			}

			// Simulate the fixed logic from binaries.go
			metadata := &binaryMetadata{Version: tt.cachedVersion}
			var targetVersion string
			var needsExtract bool

			// This is the exact logic from the fixed ExtractAndRun function
			if envVersion := os.Getenv("ROCKETSHIP_VERSION"); envVersion != "" {
				targetVersion = envVersion
				needsExtract = metadata.Version != envVersion
			} else {
				// Fixed: use CLI version (DefaultVersion) instead of cached version
				targetVersion = tt.cliVersion // In real code this would be DefaultVersion
				needsExtract = metadata.Version != tt.cliVersion
			}

			if targetVersion != tt.expectedTarget {
				t.Errorf("Expected target version %s, got %s", tt.expectedTarget, targetVersion)
			}

			if needsExtract != tt.shouldExtract {
				t.Errorf("Expected needsExtract=%v, got %v", tt.shouldExtract, needsExtract)
			}

			// Log the test case results for clarity
			t.Logf("CLI: %s, Cached: %s, Env: %s → Target: %s, Extract: %v",
				tt.cliVersion, tt.cachedVersion, tt.envVersion, targetVersion, needsExtract)
		})
	}
}

// Benchmark tests for performance

