package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultServiceName = "rocketship"
	tokenExpirySlack   = 2 * time.Minute
)

// Manager coordinates storage and refresh lifecycle.
type Manager struct {
	keyring  Store
	fallback Store
}

var errKeyringDisabled = errors.New("rocketship keyring disabled")

// NewManager constructs a Manager that prefers the system keyring.
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}
	fallback, err := NewFileStore(fmt.Sprintf("%s/.rocketship/tokens", home))
	if err != nil {
		return nil, fmt.Errorf("failed to initialise token store: %w", err)
	}

	var keyringStore Store = NewKeyringStore(defaultServiceName)
	if disabled := strings.ToLower(os.Getenv("ROCKETSHIP_DISABLE_KEYRING")); disabled == "1" || disabled == "true" {
		keyringStore = &disabledStore{}
	}

	mgr := &Manager{
		keyring:  keyringStore,
		fallback: fallback,
	}
	return mgr, nil
}

// Save persists token data, preferring the OS keyring.
func (m *Manager) Save(profile string, data TokenData) error {
	if err := data.Validate(); err != nil {
		return err
	}
	if err := m.keyring.Save(profile, data); err == nil {
		return nil
	}
	if err := m.fallback.Save(profile, data); err != nil {
		return fmt.Errorf("failed to store tokens on disk: %w", err)
	}
	return nil
}

// Load returns stored tokens for a profile.
func (m *Manager) Load(profile string) (TokenData, error) {
	data, err := m.keyring.Load(profile)
	if err == nil {
		return data, nil
	}
	ls, loadErr := m.fallback.Load(profile)
	if loadErr != nil {
		if errors.Is(loadErr, ErrTokenNotFound) {
			return TokenData{}, ErrTokenNotFound
		}
		return TokenData{}, loadErr
	}
	return ls, nil
}

// Delete removes tokens for a profile.
func (m *Manager) Delete(profile string) error {
	_ = m.keyring.Delete(profile)
	if err := m.fallback.Delete(profile); err != nil && !errors.Is(err, ErrTokenNotFound) {
		return fmt.Errorf("failed to delete fallback token: %w", err)
	}
	return nil
}

// AccessToken returns the usable access token, refreshing when necessary.
func (m *Manager) AccessToken(profile string, refreshFn func(TokenData) (TokenData, error)) (string, error) {
	data, err := m.Load(profile)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", err
		}
		return "", fmt.Errorf("failed to load tokens: %w", err)
	}

	if data.IsExpired(tokenExpirySlack) {
		if !data.HasRefresh() {
			return "", errors.New("access token expired and no refresh token available")
		}
		if refreshFn == nil {
			return "", errors.New("refresh function not provided")
		}
		refreshed, err := refreshFn(data)
		if err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		if err := m.Save(profile, refreshed); err != nil {
			return "", err
		}
		data = refreshed
	}

	return data.AccessToken, nil
}

type disabledStore struct{}

func (d *disabledStore) Save(string, TokenData) error   { return errKeyringDisabled }
func (d *disabledStore) Load(string) (TokenData, error) { return TokenData{}, errKeyringDisabled }
func (d *disabledStore) Delete(string) error            { return errKeyringDisabled }
