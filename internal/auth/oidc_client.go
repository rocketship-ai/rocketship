package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCClient implements the Client interface using OIDC
type OIDCClient struct {
	config       *AuthConfig
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// NewOIDCClient creates a new OIDC client
func NewOIDCClient(ctx context.Context, config *AuthConfig) (*OIDCClient, error) {
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       config.Scopes,
	}

	// Add default scopes if not present
	hasOpenID := false
	hasProfile := false
	hasEmail := false
	for _, scope := range oauth2Config.Scopes {
		switch scope {
		case "openid":
			hasOpenID = true
		case "profile":
			hasProfile = true
		case "email":
			hasEmail = true
		}
	}
	if !hasOpenID {
		oauth2Config.Scopes = append(oauth2Config.Scopes, "openid")
	}
	if !hasProfile {
		oauth2Config.Scopes = append(oauth2Config.Scopes, "profile")
	}
	if !hasEmail {
		oauth2Config.Scopes = append(oauth2Config.Scopes, "email")
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &OIDCClient{
		config:       config,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
	}, nil
}

// GetAuthURL returns the authorization URL with PKCE
func (c *OIDCClient) GetAuthURL(state string) (string, *PKCEChallenge, error) {
	pkce, err := GeneratePKCEChallenge()
	if err != nil {
		return "", nil, err
	}

	authURL := c.oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", pkce.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", pkce.Method),
	)

	return authURL, pkce, nil
}

// ExchangeCode exchanges an authorization code for tokens
func (c *OIDCClient) ExchangeCode(ctx context.Context, code string, codeVerifier string) (*Token, error) {
	oauth2Token, err := c.oauth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	return c.convertToken(oauth2Token), nil
}

// RefreshToken refreshes an access token
func (c *OIDCClient) RefreshToken(ctx context.Context, refreshToken string) (*Token, error) {
	tokenSource := c.oauth2Config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})

	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return c.convertToken(newToken), nil
}

// GetUserInfo retrieves user information
func (c *OIDCClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	userInfoURL := c.provider.Endpoint().AuthURL
	// Replace /auth with /userinfo for Keycloak-style endpoints
	userInfoURL = strings.Replace(userInfoURL, "/auth", "/userinfo", 1)
	// For other providers, try standard userinfo endpoint
	if !strings.Contains(userInfoURL, "/userinfo") {
		// Try to get userinfo endpoint from provider metadata
		var claims struct {
			UserInfoEndpoint string `json:"userinfo_endpoint"`
		}
		if err := c.provider.Claims(&claims); err == nil && claims.UserInfoEndpoint != "" {
			userInfoURL = claims.UserInfoEndpoint
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}

	// Check if user is admin based on group membership
	if c.config.AdminGroup != "" {
		for _, group := range userInfo.Groups {
			if group == c.config.AdminGroup {
				userInfo.IsAdmin = true
				break
			}
		}
	}

	return &userInfo, nil
}

// ValidateToken validates an access token
func (c *OIDCClient) ValidateToken(ctx context.Context, accessToken string) (*UserInfo, error) {
	// For access token validation, we'll use the userinfo endpoint
	// as OIDC doesn't provide a standard introspection endpoint
	return c.GetUserInfo(ctx, accessToken)
}

// Logout performs logout operation
func (c *OIDCClient) Logout(ctx context.Context, token *Token) error {
	// Get end session endpoint from provider metadata
	var claims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := c.provider.Claims(&claims); err != nil {
		// If no end session endpoint, just return success
		// The client will handle token deletion
		return nil
	}

	if claims.EndSessionEndpoint == "" {
		return nil
	}

	// Build logout URL
	logoutURL, err := url.Parse(claims.EndSessionEndpoint)
	if err != nil {
		return fmt.Errorf("failed to parse logout URL: %w", err)
	}

	q := logoutURL.Query()
	if token.IDToken != "" {
		q.Set("id_token_hint", token.IDToken)
	}
	q.Set("post_logout_redirect_uri", c.config.RedirectURL)
	logoutURL.RawQuery = q.Encode()

	// Perform logout request
	req, err := http.NewRequestWithContext(ctx, "GET", logoutURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create logout request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform logout: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

// convertToken converts an oauth2.Token to our Token type
func (c *OIDCClient) convertToken(oauth2Token *oauth2.Token) *Token {
	token := &Token{
		AccessToken:  oauth2Token.AccessToken,
		RefreshToken: oauth2Token.RefreshToken,
		TokenType:    oauth2Token.TokenType,
		ExpiresAt:    oauth2Token.Expiry,
	}

	// Extract ID token if present
	if idToken, ok := oauth2Token.Extra("id_token").(string); ok {
		token.IDToken = idToken
	}

	// Calculate expires_in
	if !token.ExpiresAt.IsZero() {
		token.ExpiresIn = int(time.Until(token.ExpiresAt).Seconds())
	}

	return token
}