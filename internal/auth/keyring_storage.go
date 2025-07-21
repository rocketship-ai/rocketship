package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "rocketship"
	keyringUser    = "token"
)

// KeyringStorage implements TokenStorage using system keyring
type KeyringStorage struct{
	service string
	user    string
}

// NewKeyringStorage creates a new keyring-based token storage
func NewKeyringStorage() *KeyringStorage {
	return &KeyringStorage{
		service: keyringService,
		user:    keyringUser,
	}
}

// NewKeyringStorageWithKey creates a new keyring-based token storage with custom key
func NewKeyringStorageWithKey(service, user string) *KeyringStorage {
	return &KeyringStorage{
		service: service,
		user:    user,
	}
}

// SaveToken saves a token to the keyring
func (k *KeyringStorage) SaveToken(ctx context.Context, token *Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := keyring.Set(k.service, k.user, string(data)); err != nil {
		return fmt.Errorf("failed to save token to keyring: %w", err)
	}

	return nil
}

// GetToken retrieves a token from the keyring
func (k *KeyringStorage) GetToken(ctx context.Context) (*Token, error) {
	data, err := keyring.Get(k.service, k.user)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, nil // No token stored
		}
		return nil, fmt.Errorf("failed to get token from keyring: %w", err)
	}

	var token Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// DeleteToken removes a token from the keyring
func (k *KeyringStorage) DeleteToken(ctx context.Context) error {
	err := keyring.Delete(k.service, k.user)
	if err != nil && err != keyring.ErrNotFound {
		return fmt.Errorf("failed to delete token from keyring: %w", err)
	}
	return nil
}