package authbroker

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// handleDeviceCode initiates the OAuth device flow by requesting a device code from GitHub
func (s *Server) handleDeviceCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form payload")
		return
	}

	clientID := strings.TrimSpace(r.Form.Get("client_id"))
	if clientID == "" {
		writeOAuthError(w, "invalid_request", "client_id missing")
		return
	}
	if !strings.EqualFold(clientID, s.cfg.ClientID) {
		writeOAuthError(w, "unauthorized_client", "client_id not recognised")
		return
	}

	ctx := r.Context()
	dc, err := s.github.RequestDeviceCode(ctx, s.cfg.GitHub.Scopes)
	if err != nil {
		log.Printf("github device flow error: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github device flow error: %v", err))
		return
	}

	s.mu.Lock()
	s.pending[dc.DeviceCode] = deviceSession{
		clientID:  clientID,
		scopes:    copyScopes(s.cfg.Scopes),
		expiresAt: time.Now().Add(dc.ExpiresIn),
	}
	s.mu.Unlock()

	resp := map[string]interface{}{
		"device_code":               dc.DeviceCode,
		"user_code":                 dc.UserCode,
		"verification_uri":          dc.VerificationURI,
		"verification_uri_complete": dc.VerificationURIComplete,
		"expires_in":                dc.RawExpiresIn,
		"interval":                  dc.RawInterval,
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDeviceGrant exchanges a device code for access and refresh tokens
func (s *Server) handleDeviceGrant(w http.ResponseWriter, r *http.Request) {
	deviceCode := strings.TrimSpace(r.Form.Get("device_code"))
	clientID := strings.TrimSpace(r.Form.Get("client_id"))
	if deviceCode == "" {
		writeOAuthError(w, "invalid_request", "device_code missing")
		return
	}
	if !strings.EqualFold(clientID, s.cfg.ClientID) {
		writeOAuthError(w, "unauthorized_client", "client_id not recognised")
		return
	}

	session, ok := s.lookupDeviceSession(deviceCode)
	if !ok {
		writeOAuthError(w, "authorization_pending", "device authorization pending")
		return
	}

	ctx := r.Context()
	token, terr, err := s.github.ExchangeDeviceCode(ctx, deviceCode)
	if err != nil {
		log.Printf("github token exchange failed: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github token exchange failed: %v", err))
		return
	}
	if terr.Error != "" {
		log.Printf("github token exchange error: %s (%s)", terr.Error, terr.ErrorDescription)
		writeOAuthError(w, terr.Error, terr.ErrorDescription)
		return
	}
	log.Printf("github token exchange success: type=%s scope=%q len=%d", token.TokenType, token.Scope, len(token.AccessToken))

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

	summary, err := s.store.RoleSummary(ctx, userRecord.ID)
	if err != nil {
		log.Printf("failed to load user roles: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve roles")
		return
	}

	roles := summary.AggregatedRoles()
	primaryOrg := selectPrimaryOrg(summary)

	tokens, err := s.mintTokens(ctx, userRecord, roles, primaryOrg, session.scopes)
	if err != nil {
		log.Printf("failed to issue tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to mint tokens")
		return
	}

	s.removeDeviceSession(deviceCode)

	if containsRole(roles, "pending") {
		log.Printf("user %s authenticated but has no organization membership", user.Email)
	}

	writeJSON(w, http.StatusOK, tokens)
}

// lookupDeviceSession retrieves a pending device session
func (s *Server) lookupDeviceSession(deviceCode string) (deviceSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.pending[deviceCode]
	if !ok {
		return deviceSession{}, false
	}
	if time.Now().After(session.expiresAt) {
		delete(s.pending, deviceCode)
		return deviceSession{}, false
	}
	return session, true
}

// removeDeviceSession removes a device session from the pending map
func (s *Server) removeDeviceSession(deviceCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, deviceCode)
}
