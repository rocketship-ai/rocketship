package auth

import (
	"context"
	"time"
)

// TokenType represents the type of token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeID      TokenType = "id"
)

// Token represents an OAuth2/OIDC token
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// UserInfo represents user information from OIDC
type UserInfo struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Groups        []string `json:"groups,omitempty"`
	IsAdmin       bool     `json:"-"` // Determined by group membership
}

// PKCEChallenge represents a PKCE code challenge
type PKCEChallenge struct {
	CodeVerifier  string
	CodeChallenge string
	Method        string
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	IssuerURL     string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	AdminEmails   string // Comma-separated list of admin emails
}

// TokenStorage interface for storing tokens
type TokenStorage interface {
	// SaveToken saves a token
	SaveToken(ctx context.Context, token *Token) error
	// GetToken retrieves a token
	GetToken(ctx context.Context) (*Token, error)
	// DeleteToken removes a token
	DeleteToken(ctx context.Context) error
}

// Client interface for authentication operations
type Client interface {
	// GetAuthURL returns the authorization URL with PKCE
	GetAuthURL(state string) (string, *PKCEChallenge, error)
	// ExchangeCode exchanges an authorization code for tokens
	ExchangeCode(ctx context.Context, code string, codeVerifier string) (*Token, error)
	// RefreshToken refreshes an access token
	RefreshToken(ctx context.Context, refreshToken string) (*Token, error)
	// GetUserInfo retrieves user information
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
	// ValidateToken validates an access token
	ValidateToken(ctx context.Context, accessToken string) (*UserInfo, error)
	// Logout performs logout operation
	Logout(ctx context.Context, token *Token) error
}