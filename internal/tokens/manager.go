package tokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// Manager handles API token operations
type Manager struct {
	repo *rbac.Repository
}

// NewManager creates a new token manager
func NewManager(repo *rbac.Repository) *Manager {
	return &Manager{repo: repo}
}

// CreateToken creates a new API token
func (m *Manager) CreateToken(ctx context.Context, req *CreateTokenRequest) (*CreateTokenResponse, error) {
	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	
	// Create token hash
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])
	
	// Create API token record
	apiToken := &rbac.APIToken{
		ID:          uuid.New().String(),
		TokenHash:   tokenHash,
		TeamID:      req.TeamID,
		Name:        req.Name,
		Permissions: req.Permissions,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   time.Now(),
		CreatedBy:   req.CreatedBy,
	}
	
	// Save to database
	if err := m.repo.CreateAPIToken(ctx, apiToken); err != nil {
		return nil, fmt.Errorf("failed to create API token: %w", err)
	}
	
	return &CreateTokenResponse{
		Token:     token,
		TokenID:   apiToken.ID,
		ExpiresAt: apiToken.ExpiresAt,
	}, nil
}

// ValidateToken validates an API token and returns its context
func (m *Manager) ValidateToken(ctx context.Context, token string) (*rbac.AuthContext, error) {
	// Create token hash
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])
	
	// Get token from database
	apiToken, err := m.repo.GetAPIToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}
	if apiToken == nil {
		return nil, fmt.Errorf("invalid token")
	}
	
	// Check if token is expired
	if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}
	
	// Update last used timestamp
	if err := m.repo.UpdateAPITokenLastUsed(ctx, apiToken.ID); err != nil {
		// Log error but don't fail validation
		fmt.Printf("Warning: failed to update token last used: %v\n", err)
	}
	
	// Get team information
	team, err := m.repo.GetTeam(ctx, apiToken.TeamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("token team not found")
	}
	
	// Create auth context
	authCtx := &rbac.AuthContext{
		UserID:          "", // API tokens don't have users
		Email:           "",
		Name:            fmt.Sprintf("API Token: %s", apiToken.Name),
		IsAdmin:         false, // API tokens are never admins
		TokenID:         &apiToken.ID,
		TokenTeamID:     &apiToken.TeamID,
		TokenPerms:      apiToken.Permissions,
		TeamMemberships: []rbac.TeamMember{}, // API tokens don't have team memberships
	}
	
	return authCtx, nil
}

// RevokeToken revokes an API token
func (m *Manager) RevokeToken(ctx context.Context, tokenID string) error {
	// For now, we'll just set expires_at to now
	// In a real implementation, you might want a separate revoked status
	query := `UPDATE api_tokens SET expires_at = NOW() WHERE id = $1`
	
	// This is a direct query - in a real implementation, you'd add this to the repository
	return fmt.Errorf("not implemented - would execute: %s", query)
}

// CreateTokenRequest represents a request to create an API token
type CreateTokenRequest struct {
	TeamID      string             `json:"team_id"`
	Name        string             `json:"name"`
	Permissions []rbac.Permission  `json:"permissions"`
	ExpiresAt   *time.Time         `json:"expires_at"`
	CreatedBy   string             `json:"created_by"`
}

// CreateTokenResponse represents the response from creating an API token
type CreateTokenResponse struct {
	Token     string     `json:"token"`
	TokenID   string     `json:"token_id"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// ValidateTokenRequest represents a request to validate an API token
type ValidateTokenRequest struct {
	Token string `json:"token"`
}

// ValidateTokenResponse represents the response from validating an API token
type ValidateTokenResponse struct {
	Valid   bool           `json:"valid"`
	Context *rbac.AuthContext `json:"context,omitempty"`
	Error   string         `json:"error,omitempty"`
}