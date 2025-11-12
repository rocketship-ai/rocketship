package authbroker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// Server exposes OAuth-compatible endpoints backed by GitHub device flow and web application flow.
type Server struct {
	cfg          Config
	signer       *Signer
	github       githubProvider
	store        dataStore
	mailer       mailer
	mux          *http.ServeMux
	pending      map[string]deviceSession
	authSessions map[string]authSession
	mu           sync.Mutex
	now          func() time.Time
}

func (s *Server) Close() error {
	if closer, ok := s.store.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

type dataStore interface {
	UpsertGitHubUser(ctx context.Context, input persistence.GitHubUserInput) (persistence.User, error)
	UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error
	RoleSummary(ctx context.Context, userID uuid.UUID) (persistence.RoleSummary, error)
	SaveRefreshToken(ctx context.Context, token string, rec persistence.RefreshTokenRecord) error
	GetRefreshToken(ctx context.Context, token string) (persistence.RefreshTokenRecord, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]persistence.ProjectMember, error)
	SetProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role string) error
	RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
	ProjectOrganizationID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	IsOrganizationAdmin(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
	DeleteOrgRegistrationsForUser(ctx context.Context, userID uuid.UUID) error
	CreateOrgRegistration(ctx context.Context, rec persistence.OrganizationRegistration) (persistence.OrganizationRegistration, error)
	GetOrgRegistration(ctx context.Context, id uuid.UUID) (persistence.OrganizationRegistration, error)
	LatestOrgRegistrationForUser(ctx context.Context, userID uuid.UUID) (persistence.OrganizationRegistration, error)
	UpdateOrgRegistrationForResend(ctx context.Context, id uuid.UUID, hash, salt []byte, expiresAt, resendAt time.Time) (persistence.OrganizationRegistration, error)
	IncrementOrgRegistrationAttempts(ctx context.Context, id uuid.UUID) error
	DeleteOrgRegistration(ctx context.Context, id uuid.UUID) error
	CreateOrganization(ctx context.Context, userID uuid.UUID, name, slug string) (persistence.Organization, error)
	OrganizationSlugExists(ctx context.Context, slug string) (bool, error)
	AddOrganizationAdmin(ctx context.Context, orgID, userID uuid.UUID) error
	CreateOrgInvite(ctx context.Context, invite persistence.OrganizationInvite) (persistence.OrganizationInvite, error)
	FindPendingOrgInvites(ctx context.Context, email string) ([]persistence.OrganizationInvite, error)
	MarkOrgInviteAccepted(ctx context.Context, inviteID, userID uuid.UUID) error
}

const (
	orgRegistrationTTL         = time.Hour
	orgRegistrationResendDelay = time.Minute
	verificationCodeLength     = 6
	maxOrgNameLength           = 120
	maxInviteEmailLength       = 320
	defaultSlugSuffixLength    = 4
	maxRegistrationAttempts    = 5
)

func (s *Server) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func newVerificationSecret(length int) (code string, salt, hash []byte, err error) {
	if length <= 0 {
		length = verificationCodeLength
	}
	const digits = "0123456789"
	buf := make([]byte, length)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, nil, err
	}
	b := make([]byte, length)
	for i := range buf {
		b[i] = digits[int(buf[i])%len(digits)]
	}
	code = string(b)

	salt = make([]byte, 16)
	if _, err = rand.Read(salt); err != nil {
		return "", nil, nil, err
	}
	sum := sha256.Sum256(append(salt, []byte(code)...))
	hash = sum[:]
	return code, salt, hash, nil
}

func verifyCode(code string, salt, hash []byte) bool {
	sum := sha256.Sum256(append(salt, []byte(strings.TrimSpace(code))...))
	return subtle.ConstantTimeCompare(hash, sum[:]) == 1
}

func slugifyName(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var builder strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = "org"
	}
	return slug
}

func randomSuffix(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	if length <= 0 {
		length = defaultSlugSuffixLength
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}

func (s *Server) ensureUniqueSlug(ctx context.Context, name string) (string, error) {
	base := slugifyName(name)
	try := base

	for attempt := 0; attempt < 10; attempt++ {
		exists, err := s.store.OrganizationSlugExists(ctx, try)
		if err != nil {
			return "", err
		}
		if !exists {
			return try, nil
		}
		suffix, err := randomSuffix(defaultSlugSuffixLength)
		if err != nil {
			return "", err
		}
		try = fmt.Sprintf("%s-%s", base, suffix)
	}
	return "", fmt.Errorf("could not reserve unique slug")
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
	ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier string) (TokenResponse, error)
	FetchUser(ctx context.Context, accessToken string) (GitHubUser, error)
}

type deviceSession struct {
	clientID  string
	scopes    []string
	expiresAt time.Time
}

type authSession struct {
	state         string
	codeChallenge string
	redirectURI   string
	scopes        []string
	expiresAt     time.Time
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

	mailer, err := newPostmarkMailer(cfg.Email)
	if err != nil {
		return nil, err
	}

	return newServerWithComponents(cfg, signer, NewGitHubClient(cfg.GitHub, nil), store, mailer)
}

func newServerWithComponents(cfg Config, signer *Signer, github githubProvider, dataStore dataStore, mail mailer) (*Server, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is required")
	}
	if github == nil {
		return nil, fmt.Errorf("github client is required")
	}
	if dataStore == nil {
		return nil, fmt.Errorf("data store is required")
	}
	if mail == nil {
		return nil, fmt.Errorf("mailer is required")
	}

	srv := &Server{
		cfg:          cfg,
		signer:       signer,
		github:       github,
		store:        dataStore,
		mailer:       mail,
		mux:          http.NewServeMux(),
		pending:      make(map[string]deviceSession),
		authSessions: make(map[string]authSession),
		now:          time.Now,
	}
	srv.routes()
	return srv, nil
}

func (s *Server) routes() {
	// OAuth Device Flow (for CLI)
	s.mux.HandleFunc("/device/code", s.handleDeviceCode)

	// OAuth Web Application Flow (for browser apps)
	s.mux.HandleFunc("/authorize", s.handleAuthorize)
	s.mux.HandleFunc("/callback", s.handleCallback)

	// Token endpoints (support both device_code and authorization_code grants)
	s.mux.HandleFunc("/token", s.handleToken)
	s.mux.HandleFunc("/refresh", s.handleRefreshEndpoint)
	s.mux.HandleFunc("/logout", s.handleLogout)
	s.mux.HandleFunc("/api/token", s.handleGetToken)

	// JWKS and health
	s.mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API endpoints
	s.mux.HandleFunc("/api/users/me", s.requireAuth(s.handleCurrentUser))
	s.mux.HandleFunc("/api/orgs/registration/start", s.requireAuth(s.handleOrgRegistrationStart))
	s.mux.HandleFunc("/api/orgs/registration/resend", s.requireAuth(s.handleOrgRegistrationResend))
	s.mux.HandleFunc("/api/orgs/registration/complete", s.requireAuth(s.handleOrgRegistrationComplete))
	s.mux.HandleFunc("/api/orgs/invites/accept", s.requireAuth(s.handleOrgInviteAccept))
	s.mux.HandleFunc("/api/orgs/", s.requireAuth(s.handleOrgRoutes))
	s.mux.HandleFunc("/api/projects/", s.requireAuth(s.handleProjectRoutes))
}

// ServeHTTP satisfies http.Handler with CORS support.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.corsMiddleware(s.mux).ServeHTTP(w, r)
}

// corsMiddleware adds CORS headers for browser requests
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if s.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the request origin is allowed for CORS
func (s *Server) isAllowedOrigin(origin string) bool {
	// Allow localhost for local development
	allowedOrigins := []string{
		"http://localhost:5173",    // Vite dev server
		"http://localhost:5174",    // Vite dev server (alt)
		"http://localhost:5175",    // Vite dev server (alt 2)
		"http://localhost:4173",    // Vite preview
		"http://localhost:3000",    // Common React dev port
		"https://app.rocketship.sh", // Production (future)
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
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
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
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
		var token string

		// Try to get token from Authorization header first
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if header != "" {
			if len(header) >= len("Bearer ") && strings.EqualFold(header[:7], "Bearer ") {
				token = strings.TrimSpace(header[7:])
			}
		}

		// If no token in header, try cookie (for browser clients)
		if token == "" {
			if cookie, err := r.Cookie("access_token"); err == nil {
				token = cookie.Value
			}
		}

		// No token found in either location
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing authentication")
			return
		}

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

func (s *Server) handleCurrentUser(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	summary, err := s.store.RoleSummary(ctx, principal.UserID)
	if err != nil {
		log.Printf("failed to load role summary: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load user state")
		return
	}

	roles := summary.AggregatedRoles()
	status := "ready"
	if len(roles) == 0 || (len(roles) == 1 && strings.EqualFold(roles[0], "pending")) {
		status = "pending"
	}

	resp := map[string]interface{}{
		"user": map[string]string{
			"id":       principal.UserID.String(),
			"email":    principal.Email,
			"name":     principal.Name,
			"username": principal.Username,
		},
		"roles":  roles,
		"status": status,
	}

	if reg, err := s.store.LatestOrgRegistrationForUser(ctx, principal.UserID); err == nil {
		if reg.ExpiresAt.After(s.nowUTC()) {
			resp["pending_registration"] = map[string]interface{}{
				"registration_id":     reg.ID.String(),
				"org_name":            reg.OrgName,
				"email":               reg.Email,
				"expires_at":          reg.ExpiresAt.Format(time.RFC3339),
				"resend_available_at": reg.ResendAvailableAt.Format(time.RFC3339),
				"attempts":            reg.Attempts,
				"max_attempts":        reg.MaxAttempts,
			}
		} else {
			_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("failed to inspect pending registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load registration state")
		return
	}

	if principal.Email != "" {
		invites, err := s.store.FindPendingOrgInvites(ctx, principal.Email)
		if err != nil {
			log.Printf("failed to list invites: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to load invites")
			return
		}
		if len(invites) > 0 {
			list := make([]map[string]interface{}, 0, len(invites))
			now := s.nowUTC()
			for _, inv := range invites {
				if inv.ExpiresAt.Before(now) {
					continue
				}
				list = append(list, map[string]interface{}{
					"invite_id":         inv.ID.String(),
					"organization_id":   inv.OrganizationID.String(),
					"organization_name": inv.OrganizationName,
					"role":              inv.Role,
					"expires_at":        inv.ExpiresAt.Format(time.RFC3339),
				})
			}
			if len(list) > 0 {
				resp["pending_invites"] = list
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleOrgRegistrationStart(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !principal.HasAnyRole("owner", "pending") {
		writeError(w, http.StatusForbidden, "owner or pending role required")
		return
	}
	if strings.TrimSpace(principal.Email) == "" {
		writeError(w, http.StatusBadRequest, "email address is required to start registration")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "organization name is required")
		return
	}
	if len(name) > maxOrgNameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("organization name must be <= %d characters", maxOrgNameLength))
		return
	}

	// Use provided email or fall back to principal's GitHub email
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = principal.Email
	}
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "valid email address is required")
		return
	}

	ctx := r.Context()
	if err := s.store.DeleteOrgRegistrationsForUser(ctx, principal.UserID); err != nil {
		log.Printf("failed to clear previous registrations: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to reset registration state")
		return
	}

	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate verification code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to prepare verification code")
		return
	}

	now := s.nowUTC()
	reg := persistence.OrganizationRegistration{
		ID:                uuid.New(),
		UserID:            principal.UserID,
		Email:             email,
		OrgName:           name,
		CodeHash:          hash,
		CodeSalt:          salt,
		Attempts:          0,
		MaxAttempts:       maxRegistrationAttempts,
		ExpiresAt:         now.Add(orgRegistrationTTL),
		ResendAvailableAt: now.Add(orgRegistrationResendDelay),
	}

	rec, err := s.store.CreateOrgRegistration(ctx, reg)
	if err != nil {
		log.Printf("failed to persist registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to start registration")
		return
	}

	if err := s.mailer.SendOrgVerification(ctx, email, name, code, rec.ExpiresAt); err != nil {
		_ = s.store.DeleteOrgRegistration(ctx, rec.ID)
		log.Printf("failed to send verification email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send verification email")
		return
	}

	response := map[string]interface{}{
		"registration_id":     rec.ID.String(),
		"org_name":            rec.OrgName,
		"email":               rec.Email,
		"expires_at":          rec.ExpiresAt.Format(time.RFC3339),
		"resend_available_at": rec.ResendAvailableAt.Format(time.RFC3339),
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleOrgRegistrationResend(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		RegistrationID string `json:"registration_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	regID, err := uuid.Parse(strings.TrimSpace(req.RegistrationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid registration id")
		return
	}

	ctx := r.Context()
	reg, err := s.store.GetOrgRegistration(ctx, regID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "registration not found")
			return
		}
		log.Printf("failed to load registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load registration")
		return
	}
	if reg.UserID != principal.UserID {
		writeError(w, http.StatusForbidden, "registration does not belong to caller")
		return
	}

	now := s.nowUTC()
	if reg.ExpiresAt.Before(now) {
		_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
		writeError(w, http.StatusGone, "registration expired")
		return
	}
	if reg.ResendAvailableAt.After(now) {
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("resend available after %s", reg.ResendAvailableAt.Format(time.RFC3339)))
		return
	}

	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate resend code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate new code")
		return
	}

	updated, err := s.store.UpdateOrgRegistrationForResend(ctx, reg.ID, hash, salt, now.Add(orgRegistrationTTL), now.Add(orgRegistrationResendDelay))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "registration not found")
			return
		}
		log.Printf("failed to update registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update registration")
		return
	}

	if err := s.mailer.SendOrgVerification(ctx, updated.Email, updated.OrgName, code, updated.ExpiresAt); err != nil {
		log.Printf("failed to send verification email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send verification email")
		return
	}

	response := map[string]interface{}{
		"registration_id":     updated.ID.String(),
		"org_name":            updated.OrgName,
		"email":               updated.Email,
		"expires_at":          updated.ExpiresAt.Format(time.RFC3339),
		"resend_available_at": updated.ResendAvailableAt.Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOrgRegistrationComplete(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RegistrationID string `json:"registration_id"`
		Code           string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "verification code is required")
		return
	}

	regID, err := uuid.Parse(strings.TrimSpace(req.RegistrationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid registration id")
		return
	}

	ctx := r.Context()
	reg, err := s.store.GetOrgRegistration(ctx, regID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "registration not found")
			return
		}
		log.Printf("failed to load registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load registration")
		return
	}
	if reg.UserID != principal.UserID {
		writeError(w, http.StatusForbidden, "registration does not belong to caller")
		return
	}

	now := s.nowUTC()
	if reg.ExpiresAt.Before(now) {
		_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
		writeError(w, http.StatusGone, "registration expired")
		return
	}

	if !verifyCode(req.Code, reg.CodeSalt, reg.CodeHash) {
		_ = s.store.IncrementOrgRegistrationAttempts(ctx, reg.ID)
		if reg.Attempts+1 >= reg.MaxAttempts {
			_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
			writeError(w, http.StatusTooManyRequests, "verification failed too many times; restart registration")
			return
		}
		writeError(w, http.StatusUnauthorized, "verification code invalid")
		return
	}

	if err := s.store.UpdateUserEmail(ctx, principal.UserID, reg.Email); err != nil {
		if errors.Is(err, persistence.ErrEmailInUse) {
			writeError(w, http.StatusConflict, "email already associated with another account")
			return
		}
		log.Printf("failed to update user email: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update user email")
		return
	}

	var org persistence.Organization
	for attempt := 0; attempt < 5; attempt++ {
		slug, err := s.ensureUniqueSlug(ctx, reg.OrgName)
		if err != nil {
			log.Printf("failed to ensure slug: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to prepare organization")
			return
		}
		org, err = s.store.CreateOrganization(ctx, principal.UserID, reg.OrgName, slug)
		if err == nil {
			break
		}
		if errors.Is(err, persistence.ErrOrganizationSlugUsed) {
			continue
		}
		log.Printf("failed to create organization: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}
	if org.ID == uuid.Nil {
		log.Printf("failed to allocate unique slug for %s", reg.OrgName)
		writeError(w, http.StatusConflict, "failed to reserve organization slug")
		return
	}

	if err := s.store.DeleteOrgRegistration(ctx, reg.ID); err != nil {
		log.Printf("failed to clear registration: %v", err)
	}

	response := map[string]interface{}{
		"organization": map[string]interface{}{
			"id":         org.ID.String(),
			"name":       org.Name,
			"slug":       org.Slug,
			"created_at": org.CreatedAt.Format(time.RFC3339),
		},
		"needs_claim_refresh": true,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOrgInviteAccept(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if strings.TrimSpace(principal.Email) == "" {
		writeError(w, http.StatusBadRequest, "email address required to accept invite")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code is required")
		return
	}

	ctx := r.Context()
	invites, err := s.store.FindPendingOrgInvites(ctx, principal.Email)
	if err != nil {
		log.Printf("failed to list invites: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to look up invites")
		return
	}
	if len(invites) == 0 {
		writeError(w, http.StatusNotFound, "no invites found for this account")
		return
	}

	now := s.nowUTC()
	var matched *persistence.OrganizationInvite
	for _, inv := range invites {
		if inv.ExpiresAt.Before(now) {
			continue
		}
		if verifyCode(code, inv.CodeSalt, inv.CodeHash) {
			matched = &inv
			break
		}
	}
	if matched == nil {
		writeError(w, http.StatusUnauthorized, "invite code invalid")
		return
	}

	if err := s.store.AddOrganizationAdmin(ctx, matched.OrganizationID, principal.UserID); err != nil {
		log.Printf("failed to add organization admin: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to apply invite")
		return
	}
	if err := s.store.MarkOrgInviteAccepted(ctx, matched.ID, principal.UserID); err != nil {
		log.Printf("failed to mark invite accepted: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update invite")
		return
	}

	response := map[string]interface{}{
		"organization": map[string]interface{}{
			"id":   matched.OrganizationID.String(),
			"name": matched.OrganizationName,
		},
		"needs_claim_refresh": true,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOrgRoutes(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/orgs/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "organization id required")
		return
	}

	orgID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid organization id")
		return
	}

	if len(segments) < 2 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	switch segments[1] {
	case "invites":
		s.handleOrgInvites(w, r, principal, orgID, segments[2:])
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

func (s *Server) handleOrgInvites(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, orgID uuid.UUID, tail []string) {
	if len(tail) > 0 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !principal.HasRole("owner") {
		writeError(w, http.StatusForbidden, "owner role required")
		return
	}

	ctx := r.Context()
	isAdmin, err := s.store.IsOrganizationAdmin(ctx, orgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check org admin: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to authorize request")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "owner role required for target organization")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "invite email is required")
		return
	}
	if len(email) > maxInviteEmailLength || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "invite email appears invalid")
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "admin"
	}
	if role != "admin" {
		writeError(w, http.StatusBadRequest, "only admin role is supported for invites")
		return
	}

	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate invite code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate invite code")
		return
	}

	invite := persistence.OrganizationInvite{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		CodeHash:       hash,
		CodeSalt:       salt,
		InvitedBy:      principal.UserID,
		ExpiresAt:      s.nowUTC().Add(orgRegistrationTTL),
	}
	record, err := s.store.CreateOrgInvite(ctx, invite)
	if err != nil {
		log.Printf("failed to create invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	displayName := principal.Name
	if displayName == "" {
		displayName = principal.Email
	}

	if err := s.mailer.SendOrgInvite(ctx, email, record.OrganizationName, code, record.ExpiresAt, displayName); err != nil {
		log.Printf("failed to send invite email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send invite email")
		return
	}

	response := map[string]interface{}{
		"invite_id":         record.ID.String(),
		"organization_id":   record.OrganizationID.String(),
		"organization_name": record.OrganizationName,
		"role":              record.Role,
		"expires_at":        record.ExpiresAt.Format(time.RFC3339),
		"invite_code":       code,
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

	orgID, err := s.store.ProjectOrganizationID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		log.Printf("failed to resolve project organization: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve project")
		return
	}

	isAdmin, err := s.store.IsOrganizationAdmin(r.Context(), orgID, principal.UserID)
	if err != nil {
		log.Printf("failed to verify organization admin: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to authorize request")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "owner role required for target organization")
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

// setAuthCookies sets secure httpOnly cookies for authentication
func (s *Server) setAuthCookies(w http.ResponseWriter, r *http.Request, tokens oauthTokenResponse) {
	// Determine if we should use Secure flag (HTTPS only)
	// Use Secure=true for production, but allow HTTP for local development
	secure := !strings.Contains(r.Host, "localhost") && !strings.Contains(r.Host, "127.0.0.1") && !strings.Contains(r.Host, ".local")

	// Set access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    tokens.AccessToken,
		Path:     "/",
		MaxAge:   tokens.ExpiresIn,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Set refresh token cookie if present
	if tokens.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    tokens.RefreshToken,
			Path:     "/",
			MaxAge:   int(s.cfg.RefreshTokenTTL.Seconds()),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
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

	// Get refresh token record from database
	record, err := s.store.GetRefreshToken(r.Context(), refreshCookie.Value)
	if err != nil {
		if errors.Is(err, persistence.ErrRefreshTokenNotFound) {
			writeError(w, http.StatusUnauthorized, "refresh token invalid or expired")
			return
		}
		log.Printf("failed to load refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to validate refresh token")
		return
	}

	// Check if refresh token is expired
	now := time.Now().UTC()
	if now.After(record.ExpiresAt) {
		_ = s.store.DeleteRefreshToken(r.Context(), refreshCookie.Value)
		writeError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	// Get current roles
	summary, err := s.store.RoleSummary(r.Context(), record.User.ID)
	if err != nil {
		log.Printf("failed to load roles during refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve roles")
		return
	}
	roles := summary.AggregatedRoles()
	if len(roles) == 1 && containsRole(roles, "pending") {
		_ = s.store.DeleteRefreshToken(r.Context(), refreshCookie.Value)
		writeError(w, http.StatusUnauthorized, "user has no active organization")
		return
	}
	primaryOrg := selectPrimaryOrg(summary)

	// Delete old refresh token (token rotation)
	if err := s.store.DeleteRefreshToken(r.Context(), refreshCookie.Value); err != nil {
		log.Printf("failed to remove old refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}

	// Mint new tokens
	tokens, err := s.mintTokens(r.Context(), record.User, roles, primaryOrg, record.Scopes)
	if err != nil {
		log.Printf("failed to mint refreshed tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to issue refreshed tokens")
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
