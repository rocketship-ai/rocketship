package authbroker

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// Server exposes OAuth-compatible endpoints backed by GitHub device flow.
type Server struct {
	cfg     Config
	signer  *Signer
	github  githubProvider
	store   dataStore
	mux     *http.ServeMux
	pending map[string]deviceSession
	mu      sync.Mutex
}

func (s *Server) Close() error {
	if closer, ok := s.store.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

type dataStore interface {
	UpsertGitHubUser(ctx context.Context, input persistence.GitHubUserInput) (persistence.User, error)
	RoleSummary(ctx context.Context, userID uuid.UUID) (persistence.RoleSummary, error)
	SaveRefreshToken(ctx context.Context, token string, rec persistence.RefreshTokenRecord) error
	GetRefreshToken(ctx context.Context, token string) (persistence.RefreshTokenRecord, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	CreateOrganizationWithProject(ctx context.Context, userID uuid.UUID, input persistence.CreateOrgInput) (persistence.Organization, persistence.Project, error)
	ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]persistence.ProjectMember, error)
	SetProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role string) error
	RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
}

type brokerPrincipal struct {
	UserID   uuid.UUID
	Roles    []string
	Email    string
	Name     string
	Username string
}

func (p brokerPrincipal) HasRole(role string) bool {
	return containsRole(p.Roles, role)
}

func (p brokerPrincipal) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if p.HasRole(role) {
			return true
		}
	}
	return false
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

	ctx := context.Background()
	store, err := persistence.NewStore(ctx, cfg.DatabaseURL, cfg.RefreshTokenKey)
	if err != nil {
		return nil, err
	}

	return newServerWithComponents(cfg, signer, NewGitHubClient(cfg.GitHub, nil), store)
}

func newServerWithComponents(cfg Config, signer *Signer, github githubProvider, dataStore dataStore) (*Server, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is required")
	}
	if github == nil {
		return nil, fmt.Errorf("github client is required")
	}
	if dataStore == nil {
		return nil, fmt.Errorf("data store is required")
	}

	srv := &Server{
		cfg:     cfg,
		signer:  signer,
		github:  github,
		store:   dataStore,
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
	s.mux.HandleFunc("/api/orgs", s.requireAuth(s.handleCreateOrganization))
	s.mux.HandleFunc("/api/projects/", s.requireAuth(s.handleProjectRoutes))
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

	record, err := s.store.GetRefreshToken(r.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, persistence.ErrRefreshTokenNotFound) {
			writeOAuthError(w, "invalid_grant", "refresh token invalid or expired")
			return
		}
		log.Printf("failed to load refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to validate refresh token")
		return
	}

	now := time.Now().UTC()
	if now.After(record.ExpiresAt) {
		_ = s.store.DeleteRefreshToken(r.Context(), refreshToken)
		writeOAuthError(w, "invalid_grant", "refresh token expired")
		return
	}

	summary, err := s.store.RoleSummary(r.Context(), record.User.ID)
	if err != nil {
		log.Printf("failed to load roles during refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve roles")
		return
	}
	roles := summary.AggregatedRoles()
	if len(roles) == 1 && containsRole(roles, "pending") {
		_ = s.store.DeleteRefreshToken(r.Context(), refreshToken)
		writeOAuthError(w, "invalid_grant", "user has no active organization")
		return
	}
	primaryOrg := selectPrimaryOrg(summary)

	if err := s.store.DeleteRefreshToken(r.Context(), refreshToken); err != nil {
		log.Printf("failed to remove old refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}

	tokens, err := s.mintTokens(r.Context(), record.User, roles, primaryOrg, record.Scopes)
	if err != nil {
		log.Printf("failed to rotate tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to issue refreshed tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
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

func (s *Server) requireAuth(next func(http.ResponseWriter, *http.Request, brokerPrincipal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if header == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}
		if len(header) < len("Bearer ") || !strings.EqualFold(header[:7], "Bearer ") {
			writeError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}

		token := strings.TrimSpace(header[7:])
		claims, err := s.parseToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		principal, err := principalFromClaims(claims)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}

		next(w, r, principal)
	}
}

func (s *Server) parseToken(token string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}

	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		switch {
		case s.signer.rsaKey != nil:
			return &s.signer.rsaKey.PublicKey, nil
		case s.signer.ecKey != nil:
			return &s.signer.ecKey.PublicKey, nil
		default:
			return nil, errors.New("no signing key configured")
		}
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("token invalid")
	}

	if iss, _ := claims["iss"].(string); iss != s.cfg.Issuer {
		return nil, errors.New("issuer mismatch")
	}
	if aud := claims["aud"]; aud != nil {
		if !matchAudience(aud, s.cfg.Audience) {
			return nil, errors.New("audience mismatch")
		}
	}

	return claims, nil
}

func principalFromClaims(claims jwt.MapClaims) (brokerPrincipal, error) {
	userIDStr := stringClaim(claims["user_id"])
	if userIDStr == "" {
		if sub := stringClaim(claims["sub"]); strings.HasPrefix(sub, "user:") {
			userIDStr = strings.TrimPrefix(sub, "user:")
		}
	}
	if userIDStr == "" {
		return brokerPrincipal{}, errors.New("token missing user identifier")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return brokerPrincipal{}, errors.New("token contains invalid user identifier")
	}

	roles, err := stringSliceFromClaim(claims["roles"])
	if err != nil {
		return brokerPrincipal{}, fmt.Errorf("invalid roles claim: %w", err)
	}
	if len(roles) == 0 {
		return brokerPrincipal{}, errors.New("token missing roles")
	}

	principal := brokerPrincipal{
		UserID:   userID,
		Roles:    roles,
		Email:    stringClaim(claims["email"]),
		Name:     stringClaim(claims["name"]),
		Username: stringClaim(claims["preferred_username"]),
	}
	return principal, nil
}

func stringSliceFromClaim(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
		return out, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, errors.New("roles must be strings")
			}
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, nil
		}
		return []string{trimmed}, nil
	default:
		return nil, fmt.Errorf("unexpected roles type %T", value)
	}
}

func stringClaim(value interface{}) string {
	s, _ := value.(string)
	return strings.TrimSpace(s)
}

func matchAudience(aud interface{}, expected string) bool {
	switch v := aud.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), expected)
	case []string:
		for _, item := range v {
			if matchAudience(item, expected) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if matchAudience(item, expected) {
				return true
			}
		}
	}
	return false
}

func selectPrimaryOrg(summary persistence.RoleSummary) uuid.UUID {
	for _, org := range summary.Organizations {
		if org.IsAdmin {
			return org.OrganizationID
		}
	}
	for _, org := range summary.Organizations {
		return org.OrganizationID
	}
	for _, project := range summary.Projects {
		if project.OrganizationID != uuid.Nil {
			return project.OrganizationID
		}
	}
	return uuid.Nil
}

func containsRole(roles []string, role string) bool {
	role = strings.ToLower(role)
	for _, r := range roles {
		if strings.ToLower(r) == role {
			return true
		}
	}
	return false
}

func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	return strings.Join(scopes, " ")
}

func (s *Server) mintTokens(ctx context.Context, user persistence.User, roles []string, orgID uuid.UUID, scopes []string) (oauthTokenResponse, error) {
	now := time.Now().UTC()
	accessExpires := now.Add(s.cfg.AccessTokenTTL)
	refreshExpires := now.Add(s.cfg.RefreshTokenTTL)

	jti, err := generateRandomToken()
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to generate jti: %w", err)
	}

	claims := jwt.MapClaims{
		"iss":                s.cfg.Issuer,
		"aud":                s.cfg.Audience,
		"sub":                fmt.Sprintf("user:%s", user.ID.String()),
		"user_id":            user.ID.String(),
		"github_user_id":     user.GitHubUserID,
		"exp":                accessExpires.Unix(),
		"iat":                now.Unix(),
		"email":              user.Email,
		"email_verified":     true,
		"name":               user.Name,
		"preferred_username": user.Username,
		"scope":              joinScopes(scopes),
		"roles":              roles,
		"jti":                jti,
	}

	if orgID != uuid.Nil {
		claims["org_id"] = orgID.String()
	}

	accessToken, err := s.signer.Sign(claims)
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshToken, err := generateRandomToken()
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to mint refresh token: %w", err)
	}

	record := persistence.RefreshTokenRecord{
		TokenID:        uuid.New(),
		User:           user,
		OrganizationID: orgID,
		Scopes:         append([]string(nil), scopes...),
		IssuedAt:       now,
		ExpiresAt:      refreshExpires,
	}

	if err := s.store.SaveRefreshToken(ctx, refreshToken, record); err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to persist refresh token: %w", err)
	}

	response := oauthTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		Scope:        joinScopes(scopes),
		IDToken:      accessToken,
	}
	return response, nil
}

func generateRandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Server) handleCreateOrganization(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !principal.HasAnyRole("owner", "pending") {
		writeError(w, http.StatusForbidden, "owner or pending role required")
		return
	}

	type request struct {
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		Project struct {
			Name          string   `json:"name"`
			RepoURL       string   `json:"repo_url"`
			DefaultBranch string   `json:"default_branch"`
			PathScope     []string `json:"path_scope"`
		} `json:"project"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	input := persistence.CreateOrgInput{
		Name: strings.TrimSpace(req.Name),
		Slug: strings.TrimSpace(req.Slug),
		Project: persistence.ProjectInput{
			Name:          strings.TrimSpace(req.Project.Name),
			RepoURL:       strings.TrimSpace(req.Project.RepoURL),
			DefaultBranch: strings.TrimSpace(req.Project.DefaultBranch),
			PathScope:     append([]string(nil), req.Project.PathScope...),
		},
	}

	org, project, err := s.store.CreateOrganizationWithProject(r.Context(), principal.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, persistence.ErrOrganizationSlugUsed):
			writeError(w, http.StatusConflict, "organization slug already in use")
			return
		default:
			log.Printf("failed to create organization: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create organization")
			return
		}
	}

	response := map[string]interface{}{
		"organization": map[string]interface{}{
			"id":         org.ID.String(),
			"name":       org.Name,
			"slug":       org.Slug,
			"created_at": org.CreatedAt.Format(time.RFC3339),
		},
		"project": map[string]interface{}{
			"id":             project.ID.String(),
			"name":           project.Name,
			"repo_url":       project.RepoURL,
			"default_branch": project.DefaultBranch,
			"path_scope":     project.PathScope,
			"created_at":     project.CreatedAt.Format(time.RFC3339),
		},
		"message": "organization created; run `rocketship login` to refresh credentials",
	}

	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleProjectRoutes(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/projects/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "project id required")
		return
	}

	projectID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if len(segments) < 2 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	switch segments[1] {
	case "members":
		s.handleProjectMembers(w, r, principal, projectID, segments[2:])
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

func (s *Server) handleProjectMembers(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID, remainder []string) {
	if len(remainder) == 0 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !principal.HasRole("owner") {
			writeError(w, http.StatusForbidden, "owner role required")
			return
		}

		members, err := s.store.ListProjectMembers(r.Context(), projectID)
		if err != nil {
			log.Printf("failed to list project members: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to list members")
			return
		}

		payload := make([]map[string]interface{}, 0, len(members))
		for _, m := range members {
			payload = append(payload, map[string]interface{}{
				"user_id":    m.UserID.String(),
				"email":      m.Email,
				"name":       m.Name,
				"username":   m.Username,
				"role":       m.Role,
				"joined_at":  m.JoinedAt.Format(time.RFC3339),
				"updated_at": m.UpdatedAt.Format(time.RFC3339),
			})
		}

		writeJSON(w, http.StatusOK, payload)
		return
	}

	if len(remainder) != 1 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	userID, err := uuid.Parse(remainder[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		if !principal.HasRole("owner") {
			writeError(w, http.StatusForbidden, "owner role required")
			return
		}
		var body struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if err := s.store.SetProjectMemberRole(r.Context(), projectID, userID, body.Role); err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				writeError(w, http.StatusNotFound, "membership not found")
			default:
				log.Printf("failed to update member role: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to update member")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if !principal.HasRole("owner") {
			writeError(w, http.StatusForbidden, "owner role required")
			return
		}
		if err := s.store.RemoveProjectMember(r.Context(), projectID, userID); err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				writeError(w, http.StatusNotFound, "membership not found")
			default:
				log.Printf("failed to remove project member: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to remove member")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
