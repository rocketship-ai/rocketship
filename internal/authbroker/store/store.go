package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RefreshRecord struct {
	Subject   string    `json:"subject"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Roles     []string  `json:"roles"`
	Scopes    []string  `json:"scopes"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Store interface {
	Save(token string, record RefreshRecord) error
	Get(token string) (RefreshRecord, error)
	Delete(token string) error
}

var ErrNotFound = errors.New("refresh token not found")

// FileStore persists refresh tokens using AES-GCM encryption.
type FileStore struct {
	path string
	key  []byte

	mu      sync.Mutex
	records map[string]RefreshRecord
}

type payload struct {
	Records map[string]RefreshRecord `json:"records"`
	SavedAt time.Time                `json:"saved_at"`
	Version int                      `json:"version"`
}

const currentVersion = 1

func NewFileStore(path string, key []byte) (*FileStore, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("file store expects 32-byte key")
	}

	fs := &FileStore{
		path:    path,
		key:     append([]byte(nil), key...),
		records: make(map[string]RefreshRecord),
	}

	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (f *FileStore) Save(token string, record RefreshRecord) error {
	hash := hashToken(token)
	f.mu.Lock()
	defer f.mu.Unlock()

	f.records[hash] = record
	return f.persistLocked()
}

func (f *FileStore) Get(token string) (RefreshRecord, error) {
	hash := hashToken(token)
	f.mu.Lock()
	defer f.mu.Unlock()

	rec, ok := f.records[hash]
	if !ok {
		return RefreshRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *FileStore) Delete(token string) error {
	hash := hashToken(token)
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.records[hash]; !ok {
		return ErrNotFound
	}
	delete(f.records, hash)
	return f.persistLocked()
}

func (f *FileStore) load() error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return f.ensureParent()
		}
		return fmt.Errorf("failed to read refresh store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	plaintext, err := f.decrypt(data)
	if err != nil {
		return fmt.Errorf("failed to decrypt refresh store: %w", err)
	}

	var pl payload
	if err := json.Unmarshal(plaintext, &pl); err != nil {
		return fmt.Errorf("failed to decode refresh store: %w", err)
	}

	if pl.Records != nil {
		f.records = pl.Records
	}
	return nil
}

func (f *FileStore) persistLocked() error {
	if err := f.ensureParent(); err != nil {
		return err
	}

	pl := payload{
		Records: f.records,
		SavedAt: time.Now().UTC(),
		Version: currentVersion,
	}
	buf, err := json.Marshal(pl)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh store: %w", err)
	}

	ciphertext, err := f.encrypt(buf)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh store: %w", err)
	}

	if err := os.WriteFile(f.path, ciphertext, 0o600); err != nil {
		return fmt.Errorf("failed to write refresh store: %w", err)
	}
	return nil
}

func (f *FileStore) ensureParent() error {
	dir := filepath.Dir(f.path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create refresh store directory: %w", err)
	}
	return nil
}

func (f *FileStore) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, sealed...), nil
}

func (f *FileStore) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, data, nil)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}
