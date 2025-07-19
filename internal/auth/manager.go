package auth

import (
	"context"
	"fmt"
	"time"
)

// Manager handles authentication operations
type Manager struct {
	client  Client
	storage TokenStorage
}

// NewManager creates a new authentication manager
func NewManager(client Client, storage TokenStorage) *Manager {
	return &Manager{
		client:  client,
		storage: storage,
	}
}

// GetAuthURL generates an authorization URL with PKCE
func (m *Manager) GetAuthURL(state string) (string, *PKCEChallenge, error) {
	return m.client.GetAuthURL(state)
}

// HandleCallback processes the OAuth callback
func (m *Manager) HandleCallback(ctx context.Context, code string, codeVerifier string) (*UserInfo, error) {
	// Exchange code for tokens
	token, err := m.client.ExchangeCode(ctx, code, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Save tokens
	if err := m.storage.SaveToken(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	// Get user info
	userInfo, err := m.client.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return userInfo, nil
}

// GetCurrentUser returns the current authenticated user
func (m *Manager) GetCurrentUser(ctx context.Context) (*UserInfo, error) {
	token, err := m.ensureValidToken(ctx)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil // Not authenticated
	}

	return m.client.GetUserInfo(ctx, token.AccessToken)
}

// GetValidToken returns a valid access token, refreshing if necessary
func (m *Manager) GetValidToken(ctx context.Context) (string, error) {
	token, err := m.ensureValidToken(ctx)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", fmt.Errorf("not authenticated")
	}
	return token.AccessToken, nil
}

// Logout logs out the current user
func (m *Manager) Logout(ctx context.Context) error {
	token, err := m.storage.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	// Perform logout if token exists
	if token != nil {
		if err := m.client.Logout(ctx, token); err != nil {
			// Log error but continue with local cleanup
			fmt.Printf("Warning: provider logout failed: %v\n", err)
		}
	}

	// Always delete local token
	return m.storage.DeleteToken(ctx)
}

// IsAuthenticated checks if the user is authenticated
func (m *Manager) IsAuthenticated(ctx context.Context) bool {
	token, err := m.storage.GetToken(ctx)
	if err != nil || token == nil {
		return false
	}

	// Check if token is expired
	if !token.ExpiresAt.IsZero() && token.ExpiresAt.Before(time.Now()) {
		// Try to refresh
		if token.RefreshToken != "" {
			if _, err := m.refreshToken(ctx, token); err == nil {
				return true
			}
		}
		return false
	}

	return true
}

// ensureValidToken ensures we have a valid token, refreshing if necessary
func (m *Manager) ensureValidToken(ctx context.Context) (*Token, error) {
	token, err := m.storage.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	if token == nil {
		return nil, nil // Not authenticated
	}

	// Check if token needs refresh
	if !token.ExpiresAt.IsZero() && token.ExpiresAt.Before(time.Now().Add(30*time.Second)) {
		if token.RefreshToken == "" {
			return nil, fmt.Errorf("token expired and no refresh token available")
		}
		return m.refreshToken(ctx, token)
	}

	return token, nil
}

// refreshToken refreshes the access token
func (m *Manager) refreshToken(ctx context.Context, oldToken *Token) (*Token, error) {
	newToken, err := m.client.RefreshToken(ctx, oldToken.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Preserve refresh token if not returned
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = oldToken.RefreshToken
	}

	// Save new token
	if err := m.storage.SaveToken(ctx, newToken); err != nil {
		return nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	return newToken, nil
}