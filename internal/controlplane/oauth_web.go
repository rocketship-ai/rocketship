package controlplane

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleToken is the main OAuth token endpoint that dispatches to grant-specific handlers
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form payload")
		return
	}

	grantType := strings.TrimSpace(r.Form.Get("grant_type"))
	switch grantType {
	case "urn:ietf:params:oauth:grant-type:device_code":
		s.handleDeviceGrant(w, r)
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshGrant(w, r)
	default:
		writeOAuthError(w, "unsupported_grant_type", "grant_type not supported")
	}
}

// handleRefreshEndpoint is a convenience endpoint that wraps handleRefreshGrant
func (s *Server) handleRefreshEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form payload")
		return
	}
	r.Form.Set("grant_type", "refresh_token")
	s.handleRefreshGrant(w, r)
}

// validateAndRotateRefreshToken validates a refresh token and issues new tokens (DRY helper)
func (s *Server) validateAndRotateRefreshToken(ctx context.Context, refreshToken string) (oauthTokenResponse, error) {
	record, err := s.store.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, persistence.ErrRefreshTokenNotFound) {
			return oauthTokenResponse{}, fmt.Errorf("refresh token invalid or expired")
		}
		log.Printf("failed to load refresh token: %v", err)
		return oauthTokenResponse{}, fmt.Errorf("failed to validate refresh token: %w", err)
	}

	now := time.Now().UTC()
	if now.After(record.ExpiresAt) {
		_ = s.store.DeleteRefreshToken(ctx, refreshToken)
		return oauthTokenResponse{}, fmt.Errorf("refresh token expired")
	}

	summary, err := s.store.RoleSummary(ctx, record.User.ID)
	if err != nil {
		log.Printf("failed to load roles during refresh: %v", err)
		return oauthTokenResponse{}, fmt.Errorf("failed to resolve roles: %w", err)
	}
	roles := summary.AggregatedRoles()
	if len(roles) == 1 && containsRole(roles, "pending") {
		_ = s.store.DeleteRefreshToken(ctx, refreshToken)
		return oauthTokenResponse{}, fmt.Errorf("user has no active organization")
	}
	primaryOrg := selectPrimaryOrg(summary)

	if err := s.store.DeleteRefreshToken(ctx, refreshToken); err != nil {
		log.Printf("failed to remove old refresh token: %v", err)
		return oauthTokenResponse{}, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	tokens, err := s.mintTokens(ctx, record.User, roles, primaryOrg, record.Scopes)
	if err != nil {
		log.Printf("failed to rotate tokens: %v", err)
		return oauthTokenResponse{}, fmt.Errorf("failed to issue refreshed tokens: %w", err)
	}

	return tokens, nil
}

// handleRefreshGrant exchanges a refresh token for new access and refresh tokens
func (s *Server) handleRefreshGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := strings.TrimSpace(r.Form.Get("refresh_token"))
	clientID := strings.TrimSpace(r.Form.Get("client_id"))
	if refreshToken == "" {
		writeOAuthError(w, "invalid_request", "refresh_token missing")
		return
	}
	if clientID != "" && !strings.EqualFold(clientID, s.cfg.ClientID) {
		writeOAuthError(w, "unauthorized_client", "client_id not recognised")
		return
	}

	tokens, err := s.validateAndRotateRefreshToken(r.Context(), refreshToken)
	if err != nil {
		writeOAuthError(w, "invalid_grant", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// handleAuthorize initiates the OAuth Web Application Flow with PKCE.
// It redirects the user to GitHub for authorization.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters
	redirectURI := strings.TrimSpace(r.URL.Query().Get("redirect_uri"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	codeChallenge := strings.TrimSpace(r.URL.Query().Get("code_challenge"))
	codeChallengeMethod := strings.TrimSpace(r.URL.Query().Get("code_challenge_method"))

	if redirectURI == "" {
		writeError(w, http.StatusBadRequest, "redirect_uri required")
		return
	}

	// Validate redirect_uri - must match ${ISSUER}/login
	expectedRedirectURI := s.cfg.Issuer + "/login"
	if redirectURI != expectedRedirectURI {
		writeError(w, http.StatusBadRequest, "redirect_uri not allowed")
		return
	}

	if state == "" {
		writeError(w, http.StatusBadRequest, "state required for CSRF protection")
		return
	}
	if codeChallenge == "" {
		writeError(w, http.StatusBadRequest, "code_challenge required for PKCE")
		return
	}
	if codeChallengeMethod != "S256" {
		writeError(w, http.StatusBadRequest, "code_challenge_method must be S256")
		return
	}

	// Store the auth session
	s.mu.Lock()
	s.authSessions[state] = authSession{
		state:         state,
		codeChallenge: codeChallenge,
		redirectURI:   redirectURI,
		scopes:        s.cfg.GitHub.Scopes,
		expiresAt:     time.Now().Add(10 * time.Minute),
	}
	s.mu.Unlock()

	// Build GitHub authorization URL
	githubAuthURL := "https://github.com/login/oauth/authorize"
	params := url.Values{}
	params.Set("client_id", s.cfg.GitHub.ClientID)
	params.Set("redirect_uri", fmt.Sprintf("%s/callback", s.cfg.Issuer))
	params.Set("state", state)
	params.Set("scope", strings.Join(s.cfg.GitHub.Scopes, " "))
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", codeChallengeMethod)

	authURL := fmt.Sprintf("%s?%s", githubAuthURL, params.Encode())
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback handles the OAuth callback from GitHub.
// It exchanges the authorization code for an access token and creates a user session.
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse callback parameters
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	errorCode := strings.TrimSpace(r.URL.Query().Get("error"))

	if errorCode != "" {
		errorDesc := r.URL.Query().Get("error_description")
		log.Printf("github authorization error: %s (%s)", errorCode, errorDesc)
		http.Error(w, fmt.Sprintf("Authorization failed: %s", errorDesc), http.StatusBadRequest)
		return
	}

	if code == "" {
		writeError(w, http.StatusBadRequest, "code parameter missing")
		return
	}
	if state == "" {
		writeError(w, http.StatusBadRequest, "state parameter missing")
		return
	}

	// Lookup and validate the auth session
	s.mu.Lock()
	session, ok := s.authSessions[state]
	if ok {
		delete(s.authSessions, state)
	}
	s.mu.Unlock()

	if !ok {
		writeError(w, http.StatusBadRequest, "invalid or expired state parameter")
		return
	}

	if time.Now().After(session.expiresAt) {
		writeError(w, http.StatusBadRequest, "authorization session expired")
		return
	}

	// Redirect back to the app with the authorization code.
	// The client will then call /token with the code and code_verifier for PKCE validation.
	redirectURL, err := url.Parse(session.redirectURI)
	if err != nil {
		log.Printf("invalid redirect_uri: %v", err)
		writeError(w, http.StatusInternalServerError, "invalid redirect_uri in session")
		return
	}

	q := redirectURL.Query()
	q.Set("code", code)
	q.Set("state", state)
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// handleAuthorizationCodeGrant exchanges an authorization code for tokens (Web Application Flow).
func (s *Server) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.Form.Get("code"))
	redirectURI := strings.TrimSpace(r.Form.Get("redirect_uri"))
	codeVerifier := strings.TrimSpace(r.Form.Get("code_verifier"))

	if code == "" {
		writeOAuthError(w, "invalid_request", "code missing")
		return
	}
	if redirectURI == "" {
		writeOAuthError(w, "invalid_request", "redirect_uri missing")
		return
	}
	if codeVerifier == "" {
		writeOAuthError(w, "invalid_request", "code_verifier missing for PKCE")
		return
	}

	// Exchange the authorization code for an access token with GitHub
	ctx := r.Context()
	githubRedirectURI := fmt.Sprintf("%s/callback", s.cfg.Issuer)
	token, err := s.github.ExchangeAuthorizationCode(ctx, code, githubRedirectURI, codeVerifier)
	if err != nil {
		log.Printf("github authorization code exchange failed: %v", err)
		writeOAuthError(w, "invalid_grant", fmt.Sprintf("code exchange failed: %v", err))
		return
	}

	// Fetch user information from GitHub
	user, err := s.github.FetchUser(ctx, token.AccessToken)
	if err != nil {
		log.Printf("github user lookup failed: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github user lookup failed: %v", err))
		return
	}

	if user.Email == "" {
		writeOAuthError(w, "access_denied", "github account is missing an email address")
		return
	}

	// Upsert user in our database
	userRecord, err := s.store.UpsertGitHubUser(ctx, persistence.GitHubUserInput{
		GitHubUserID: user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Username:     user.Login,
	})
	if err != nil {
		log.Printf("failed to upsert user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to persist user")
		return
	}

	// Load user roles
	summary, err := s.store.RoleSummary(ctx, userRecord.ID)
	if err != nil {
		log.Printf("failed to load user roles: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve roles")
		return
	}

	roles := summary.AggregatedRoles()
	primaryOrg := selectPrimaryOrg(summary)

	// Mint tokens
	tokens, err := s.mintTokens(ctx, userRecord, roles, primaryOrg, s.cfg.Scopes)
	if err != nil {
		log.Printf("failed to issue tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to mint tokens")
		return
	}

	if containsRole(roles, "pending") {
		log.Printf("user %s authenticated but has no organization membership", user.Email)
	}

	// Set tokens as httpOnly cookies for browser security
	s.setAuthCookies(w, r, tokens)

	// Still return tokens in response for compatibility (CLI/API clients)
	writeJSON(w, http.StatusOK, tokens)
}

// handleLogout clears authentication cookies and invalidates the refresh token
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Determine secure flag based on host
	secure := !strings.Contains(r.Host, "localhost") && !strings.Contains(r.Host, "127.0.0.1") && !strings.Contains(r.Host, ".local")

	// Try to get refresh token from cookie to delete it from the database
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		// Delete refresh token from database (best effort, don't fail logout if this fails)
		_ = s.store.DeleteRefreshToken(r.Context(), cookie.Value)
	}

	// Clear access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete cookie
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Clear refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete cookie
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// handleGetToken returns a fresh access token, auto-refreshing from the refresh_token cookie if needed.
// This endpoint allows the web UI to get JWTs for gRPC Authorization headers without storing tokens in localStorage.
func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// CSRF hardening: allow only same-origin requests
	origin := r.Header.Get("Origin")
	if origin != "" && !strings.EqualFold(origin, s.cfg.Issuer) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	referer := r.Header.Get("Referer")
	if referer != "" && !strings.HasPrefix(referer, s.cfg.Issuer) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Never cache this endpoint
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	// Support force=true query param to always rotate tokens
	if r.URL.Query().Get("force") == "true" {
		s.respondTokenFromRefresh(w, r)
		return
	}

	// Try to read access token cookie
	accessCookie, err := r.Cookie("access_token")
	if err != nil || accessCookie.Value == "" {
		s.respondTokenFromRefresh(w, r)
		return
	}

	// Parse and validate the access token
	claims, err := s.parseToken(accessCookie.Value)
	if err != nil {
		s.respondTokenFromRefresh(w, r)
		return
	}

	// Check expiry with 2-minute skew (refresh if expiring soon)
	const skew = 120 // seconds
	now := time.Now().Unix()
	exp, ok := claims["exp"].(float64)
	if !ok {
		s.respondTokenFromRefresh(w, r)
		return
	}

	if int64(exp) <= now+skew {
		// Token expired or expiring soon - refresh
		s.respondTokenFromRefresh(w, r)
		return
	}

	// Token still valid - return it with expiry
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessCookie.Value,
		"expires_at":   int64(exp),
	})
}

// respondTokenFromRefresh attempts to refresh tokens using the refresh_token cookie
func (s *Server) respondTokenFromRefresh(w http.ResponseWriter, r *http.Request) {
	refreshCookie, err := r.Cookie("refresh_token")
	if err != nil || refreshCookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	tokens, err := s.validateAndRotateRefreshToken(r.Context(), refreshCookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Set new cookies
	s.setAuthCookies(w, r, tokens)

	// Return fresh access token with expiry
	expiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).Unix()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": tokens.AccessToken,
		"expires_at":   expiresAt,
	})
}
