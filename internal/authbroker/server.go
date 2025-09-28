package authbroker

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/rocketship-ai/rocketship/internal/authbroker/store"
)

// Server exposes OAuth-compatible endpoints backed by GitHub device flow.
type Server struct {
	cfg     Config
	signer  *Signer
	github  githubProvider
	store   store.Store
	mux     *http.ServeMux
	pending map[string]deviceSession
	mu      sync.Mutex
}

type githubProvider interface {
	RequestDeviceCode(ctx context.Context, scopes []string) (DeviceCodeResponse, error)
	ExchangeDeviceCode(ctx context.Context, deviceCode string) (TokenResponse, tokenError, error)
	FetchUser(ctx context.Context, accessToken string) (GitHubUser, error)
}

type deviceSession struct {
	clientID  string
	scopes    []string
	expiresAt time.Time
}

// NewServer constructs a broker server using the provided configuration.
func NewServer(cfg Config) (*Server, error) {
	signer, err := NewSignerFromPEM(cfg.SigningKeyPath, cfg.SigningKeyID)
	if err != nil {
		return nil, err
	}

	tokenStore, err := store.NewFileStore(cfg.Store.Path, cfg.Store.EncryptionKey)
	if err != nil {
		return nil, err
	}

	return newServerWithComponents(cfg, signer, NewGitHubClient(cfg.GitHub, nil), tokenStore)
}

func newServerWithComponents(cfg Config, signer *Signer, github githubProvider, tokenStore store.Store) (*Server, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is required")
	}
	if github == nil {
		return nil, fmt.Errorf("github client is required")
	}
	if tokenStore == nil {
		return nil, fmt.Errorf("token store is required")
	}

	srv := &Server{
		cfg:     cfg,
		signer:  signer,
		github:  github,
		store:   tokenStore,
		mux:     http.NewServeMux(),
		pending: make(map[string]deviceSession),
	}
	srv.routes()
	return srv, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/device/code", s.handleDeviceCode)
	s.mux.HandleFunc("/token", s.handleToken)
	s.mux.HandleFunc("/refresh", s.handleRefreshEndpoint)
	s.mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// ServeHTTP satisfies http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

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
	case "refresh_token":
		s.handleRefreshGrant(w, r)
	default:
		writeOAuthError(w, "unsupported_grant_type", "grant_type not supported")
	}
}

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
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github token exchange failed: %v", err))
		return
	}
	if terr.Error != "" {
		writeOAuthError(w, terr.Error, terr.ErrorDescription)
		return
	}

	user, err := s.github.FetchUser(ctx, token.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github user lookup failed: %v", err))
		return
	}

	if user.Email == "" {
		writeOAuthError(w, "access_denied", "github account is missing an email address")
		return
	}

	now := time.Now()
	accessExpires := now.Add(s.cfg.AccessTokenTTL)
	refreshExpires := now.Add(s.cfg.RefreshTokenTTL)

	claims := jwt.MapClaims{
		"iss":                s.cfg.Issuer,
		"aud":                s.cfg.Audience,
		"sub":                fmt.Sprintf("github:%d", user.ID),
		"exp":                accessExpires.Unix(),
		"iat":                now.Unix(),
		"email":              user.Email,
		"email_verified":     true,
		"name":               user.Name,
		"preferred_username": user.Login,
		"scope":              strings.Join(session.scopes, " "),
		"roles":              []string{"owner"},
	}
	jti, err := generateRandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint token identifier")
		return
	}
	claims["jti"] = jti

	signed, err := s.signer.Sign(claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	refreshToken, err := generateRandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint refresh token")
		return
	}

	record := store.RefreshRecord{
		Subject:   claims["sub"].(string),
		Email:     user.Email,
		Name:      user.Name,
		Username:  user.Login,
		Roles:     []string{"owner"},
		Scopes:    session.scopes,
		IssuedAt:  now,
		ExpiresAt: refreshExpires,
	}

	if err := s.store.Save(refreshToken, record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist refresh token")
		return
	}

	s.removeDeviceSession(deviceCode)

	payload := oauthTokenResponse{
		AccessToken:  signed,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		Scope:        strings.Join(session.scopes, " "),
		IDToken:      signed,
	}
	writeJSON(w, http.StatusOK, payload)
}

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

	record, err := s.store.Get(refreshToken)
	if err != nil {
		writeOAuthError(w, "invalid_grant", "refresh token invalid or expired")
		return
	}
	if time.Now().After(record.ExpiresAt) {
		_ = s.store.Delete(refreshToken)
		writeOAuthError(w, "invalid_grant", "refresh token expired")
		return
	}

	now := time.Now()
	accessExpires := now.Add(s.cfg.AccessTokenTTL)

	claims := jwt.MapClaims{
		"iss":                s.cfg.Issuer,
		"aud":                s.cfg.Audience,
		"sub":                record.Subject,
		"exp":                accessExpires.Unix(),
		"iat":                now.Unix(),
		"email":              record.Email,
		"email_verified":     true,
		"name":               record.Name,
		"preferred_username": record.Username,
		"scope":              strings.Join(record.Scopes, " "),
		"roles":              record.Roles,
	}
	jti, err := generateRandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint token identifier")
		return
	}
	claims["jti"] = jti

	signed, err := s.signer.Sign(claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	newRefresh, err := generateRandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mint refresh token")
		return
	}

	record.IssuedAt = now
	record.ExpiresAt = now.Add(s.cfg.RefreshTokenTTL)
	if err := s.store.Delete(refreshToken); err != nil {
		writeOAuthError(w, "invalid_grant", "refresh token invalid")
		return
	}
	if err := s.store.Save(newRefresh, record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist refresh token")
		return
	}

	payload := oauthTokenResponse{
		AccessToken:  signed,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: newRefresh,
		Scope:        strings.Join(record.Scopes, " "),
		IDToken:      signed,
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	jwks, err := s.signer.JWKS()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to produce JWKS")
		return
	}
	writeJSON(w, http.StatusOK, jwks)
}

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

func (s *Server) removeDeviceSession(deviceCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, deviceCode)
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeOAuthError(w http.ResponseWriter, code, description string) {
	payload := map[string]string{"error": code}
	if description != "" {
		payload["error_description"] = description
	}
	writeJSON(w, http.StatusBadRequest, payload)
}

func copyScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	c := make([]string, len(scopes))
	copy(c, scopes)
	return c
}

func generateRandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
