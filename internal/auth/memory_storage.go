package auth

import (
	"context"
	"sync"
)

// MemoryStorage implements TokenStorage in memory (for testing/web)
type MemoryStorage struct {
	mu    sync.RWMutex
	token *Token
}

// NewMemoryStorage creates a new memory-based token storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

// SaveToken saves a token in memory
func (m *MemoryStorage) SaveToken(ctx context.Context, token *Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
	return nil
}

// GetToken retrieves a token from memory
func (m *MemoryStorage) GetToken(ctx context.Context) (*Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token, nil
}

// DeleteToken removes a token from memory
func (m *MemoryStorage) DeleteToken(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = nil
	return nil
}