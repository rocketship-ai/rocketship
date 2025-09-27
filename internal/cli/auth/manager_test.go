package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerFallbackToFileStore(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("ROCKETSHIP_DISABLE_KEYRING", "1")

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	token := TokenData{
		AccessToken:  "abc",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
		RefreshToken: "refresh",
	}

	// Force keyring failure by pointing service to empty path and removing permissions.
	mgr.keyring = &failingStore{err: errors.New("keyring unsupported")}

	if err := mgr.Save("test", token); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := mgr.Load("test")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.AccessToken != "abc" {
		t.Fatalf("expected access token abc, got %s", loaded.AccessToken)
	}

	if err := mgr.Delete("test"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestTokenDataMarshalRoundTrip(t *testing.T) {
	t.Setenv("ROCKETSHIP_DISABLE_KEYRING", "1")
	td := TokenData{
		AccessToken:  "token",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Minute),
		Scopes:       []string{"openid", "email"},
	}

	payload, err := td.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	round, err := UnmarshalTokenData(payload)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if round.AccessToken != td.AccessToken || len(round.Scopes) != len(td.Scopes) {
		t.Fatalf("round trip mismatch")
	}
}

// failingStore implements Store and always returns the provided error.
type failingStore struct {
	err error
}

func (f *failingStore) Save(string, TokenData) error   { return f.err }
func (f *failingStore) Load(string) (TokenData, error) { return TokenData{}, f.err }
func (f *failingStore) Delete(string) error            { return f.err }

func TestFileStorePathSanitisation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	td := TokenData{AccessToken: "abc", TokenType: "Bearer"}
	if err := store.Save("foo/bar", td); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "foo_bar.json")); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}
