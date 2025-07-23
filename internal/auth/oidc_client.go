package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"

	"github.com/rocketship-ai/rocketship/internal/rbac"
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
	var userInfoURL string
	
	// Try to get userinfo endpoint from provider metadata first
	var claims struct {
		UserInfoEndpoint string `json:"userinfo_endpoint"`
	}
	if err := c.provider.Claims(&claims); err == nil && claims.UserInfoEndpoint != "" {
		userInfoURL = claims.UserInfoEndpoint
	} else {
		// Fallback: construct userinfo URL from issuer
		issuer := strings.TrimSuffix(c.config.IssuerURL, "/")
		userInfoURL = issuer + "/userinfo"
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

	// SECURITY: Role determination moved to server-side for security
	// Client no longer determines user roles - server will determine based on server config
	userInfo.OrgRole = rbac.OrgRoleMember // Placeholder, server will override with authoritative role

	return &userInfo, nil
}

// ValidateToken validates an access token using JWT validation instead of calling Auth0
func (c *OIDCClient) ValidateToken(ctx context.Context, accessToken string) (*UserInfo, error) {
	// Try JWT validation first (fast, no network calls)
	userInfo, err := c.ValidateJWT(ctx, accessToken)
	if err == nil {
		return userInfo, nil
	}
	
	// If JWT validation fails, fall back to userinfo endpoint (for debugging/development)
	// This provides a graceful fallback while we transition to full JWT validation
	userInfo, err = c.GetUserInfo(ctx, accessToken)
	if err != nil {
		// In development, tokens might be issued by external URL but validated by internal URL
		// If validation fails with 401, log a helpful message
		if strings.Contains(err.Error(), "401") {
			return nil, fmt.Errorf("token validation failed (possible issuer mismatch between external/internal URLs): %w", err)
		}
		return nil, err
	}
	return userInfo, nil
}

// ValidateJWT validates a JWT token locally without calling Auth0
func (c *OIDCClient) ValidateJWT(ctx context.Context, tokenString string) (*UserInfo, error) {
	// Parse the JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		// Get the public key for verification
		return c.getPublicKey(ctx, token)
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}
	
	if !token.Valid {
		return nil, fmt.Errorf("invalid JWT token")
	}
	
	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to parse JWT claims")
	}
	
	// Validate required claims
	if err := c.validateClaims(claims); err != nil {
		return nil, fmt.Errorf("invalid JWT claims: %w", err)
	}
	
	// Extract user information from claims
	userInfo := &UserInfo{
		Subject: c.getStringClaim(claims, "sub"),
		Email:   c.getStringClaim(claims, "email"),
		Name:    c.getStringClaim(claims, "name"),
		// Role determination is done server-side, set placeholder
		OrgRole: rbac.OrgRoleMember,
	}
	
	// Use nickname as fallback for name if name is empty
	if userInfo.Name == "" {
		userInfo.Name = c.getStringClaim(claims, "nickname")
	}
	
	return userInfo, nil
}

// getPublicKey retrieves the public key for JWT verification
func (c *OIDCClient) getPublicKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
	// Get the key ID from the JWT header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("no kid found in JWT header")
	}
	
	// Get the JWKS (JSON Web Key Set) from the provider
	jwksURL := strings.TrimSuffix(c.config.IssuerURL, "/") + "/.well-known/jwks.json"
	
	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}
	
	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.Kid == kid && key.Kty == "RSA" && key.Use == "sig" {
			return c.rsaPublicKeyFromJWK(key.N, key.E)
		}
	}
	
	return nil, fmt.Errorf("no matching public key found for kid: %s", kid)
}

// rsaPublicKeyFromJWK constructs an RSA public key from JWK parameters
func (c *OIDCClient) rsaPublicKeyFromJWK(nStr, eStr string) (*rsa.PublicKey, error) {
	// Decode the modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	
	// Decode the exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}
	
	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	
	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// validateClaims validates JWT claims
func (c *OIDCClient) validateClaims(claims jwt.MapClaims) error {
	// Validate issuer
	iss, ok := claims["iss"].(string)
	if !ok || iss != c.config.IssuerURL {
		return fmt.Errorf("invalid issuer: expected %s, got %s", c.config.IssuerURL, iss)
	}
	
	// Validate audience (if present)
	if aud, ok := claims["aud"]; ok {
		switch v := aud.(type) {
		case string:
			if v != c.config.ClientID {
				return fmt.Errorf("invalid audience: expected %s, got %s", c.config.ClientID, v)
			}
		case []interface{}:
			found := false
			for _, a := range v {
				if audStr, ok := a.(string); ok && audStr == c.config.ClientID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("client ID not found in audience list")
			}
		}
	}
	
	// Validate expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return fmt.Errorf("token has expired")
		}
	}
	
	// Validate not before (if present)
	if nbf, ok := claims["nbf"].(float64); ok {
		if time.Now().Unix() < int64(nbf) {
			return fmt.Errorf("token not yet valid")
		}
	}
	
	return nil
}

// getStringClaim safely extracts a string claim
func (c *OIDCClient) getStringClaim(claims jwt.MapClaims, key string) string {
	if value, ok := claims[key].(string); ok {
		return value
	}
	return ""
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