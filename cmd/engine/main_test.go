package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEngineTokenFromEnv(t *testing.T) {
	t.Setenv("ROCKETSHIP_ENGINE_TOKEN", " secret-token ")
	token, err := loadEngineToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("expected trimmed token, got %q", token)
	}
}

func TestLoadEngineTokenFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(path, []byte(" file-token\n"), 0600); err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}
	t.Setenv("ROCKETSHIP_ENGINE_TOKEN_FILE", path)
	token, err := loadEngineToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "file-token" {
		t.Fatalf("expected trimmed token from file, got %q", token)
	}
}

func TestLoadEngineTokenFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte("\n"), 0600); err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}
	t.Setenv("ROCKETSHIP_ENGINE_TOKEN_FILE", path)
	if _, err := loadEngineToken(); err == nil {
		t.Fatal("expected error for empty token file")
	}
}
