package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedPythonScript(t *testing.T) {
	// Test that the embedded Python script is not empty
	if len(embeddedPythonScript) == 0 {
		t.Fatal("embedded Python script is empty")
	}

	// Test that we can write the embedded script to a file
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test_browser_automation.py")

	pe := &PythonExecutor{}
	err := pe.copyPythonScript(scriptPath)
	if err != nil {
		t.Fatalf("failed to copy Python script: %v", err)
	}

	// Verify the file was created and has content
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("failed to stat copied script: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("copied script is empty")
	}

	// Verify the content matches
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read copied script: %v", err)
	}

	if len(content) != len(embeddedPythonScript) {
		t.Fatalf("copied script size mismatch: got %d, want %d", len(content), len(embeddedPythonScript))
	}
}
