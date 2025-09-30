package store

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.enc")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	fs, err := NewFileStore(path, key)
	if err != nil {
		t.Fatalf("failed to create file store: %v", err)
	}

	record := RefreshRecord{
		Subject:   "user-123",
		Email:     "user@example.com",
		Name:      "User",
		Username:  "user",
		Roles:     []string{"owner"},
		Scopes:    []string{"openid"},
		IssuedAt:  time.Now().UTC().Truncate(time.Second),
		ExpiresAt: time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}

	if err := fs.Save("token-1", record); err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	loaded, err := fs.Get("token-1")
	if err != nil {
		t.Fatalf("failed to load record: %v", err)
	}
	if loaded.Subject != record.Subject || loaded.Email != record.Email {
		t.Fatalf("loaded record mismatch: %+v", loaded)
	}

	// Re-open to ensure data persisted and encrypted
	fs2, err := NewFileStore(path, key)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	if _, err := fs2.Get("token-1"); err != nil {
		t.Fatalf("expected record after reopen: %v", err)
	}

	if err := fs2.Delete("token-1"); err != nil {
		t.Fatalf("failed to delete token: %v", err)
	}
	if _, err := fs2.Get("token-1"); err == nil {
		t.Fatalf("expected not found after delete")
	}
}

func TestFileStoreRejectsInvalidKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.enc")
	if _, err := NewFileStore(path, []byte("short")); err == nil {
		t.Fatalf("expected error for short key")
	}
}

func TestEncryptedContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.enc")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	fs, err := NewFileStore(path, key)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := fs.Save("token-1", RefreshRecord{Subject: "s"}); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read ciphertext: %v", err)
	}
	if len(contents) == 0 {
		t.Fatalf("expected encrypted data")
	}

	// Ensure plaintext token is not present when base64 encoded
	encoded := base64.StdEncoding.EncodeToString(contents)
	if strings.Contains(encoded, "token-1") {
		t.Fatalf("ciphertext should not contain plaintext token")
	}
}
