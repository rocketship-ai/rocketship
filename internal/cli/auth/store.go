package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

var (
	ErrTokenNotFound = errors.New("token not found")
)

// Store persists per-profile token data.
type Store interface {
	Save(profile string, data TokenData) error
	Load(profile string) (TokenData, error)
	Delete(profile string) error
}

// KeyringStore stores tokens in the OS keyring.
type KeyringStore struct {
	service string
}

func NewKeyringStore(service string) *KeyringStore {
	return &KeyringStore{service: service}
}

func (s *KeyringStore) key(profile string) string {
	// Keep key deterministic yet human readable.
	return fmt.Sprintf("profile:%s", profile)
}

func (s *KeyringStore) Save(profile string, data TokenData) error {
	payload, err := data.Marshal()
	if err != nil {
		return err
	}
	if err := keyring.Set(s.service, s.key(profile), string(payload)); err != nil {
		return err
	}
	return nil
}

func (s *KeyringStore) Load(profile string) (TokenData, error) {
	value, err := keyring.Get(s.service, s.key(profile))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return TokenData{}, ErrTokenNotFound
		}
		return TokenData{}, err
	}
	return UnmarshalTokenData([]byte(value))
}

func (s *KeyringStore) Delete(profile string) error {
	if err := keyring.Delete(s.service, s.key(profile)); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// FileStore stores tokens within ~/.rocketship/tokens/
type FileStore struct {
	dir string
}

func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(profile string) string {
	sanitized := strings.ReplaceAll(profile, string(os.PathSeparator), "_")
	return filepath.Join(s.dir, sanitized+".json")
}

func (s *FileStore) Save(profile string, data TokenData) error {
	payload, err := data.Marshal()
	if err != nil {
		return err
	}
	path := s.path(profile)
	return os.WriteFile(path, payload, 0600)
}

func (s *FileStore) Load(profile string) (TokenData, error) {
	path := s.path(profile)
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TokenData{}, ErrTokenNotFound
		}
		return TokenData{}, err
	}
	return UnmarshalTokenData(payload)
}

func (s *FileStore) Delete(profile string) error {
	path := s.path(profile)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}
