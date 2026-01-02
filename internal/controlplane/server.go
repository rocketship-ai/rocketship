package controlplane

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// Server exposes OAuth-compatible endpoints backed by GitHub device flow and web application flow.
type Server struct {
	cfg          Config
	signer       *Signer
	github       githubProvider
	githubApp    *GitHubAppClient
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

func (s *Server) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
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

	githubApp, err := NewGitHubAppClient(cfg.GitHubApp, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App client: %w", err)
	}

	return newServerWithComponents(cfg, signer, NewGitHubClient(cfg.GitHub, nil), githubApp, store, mailer)
}

func newServerWithComponents(cfg Config, signer *Signer, github githubProvider, githubApp *GitHubAppClient, dataStore dataStore, mail mailer) (*Server, error) {
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
		githubApp:    githubApp,
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
	s.mux.HandleFunc("/healthz", s.handleHealth)

	// API endpoints
	s.mux.HandleFunc("/api/users/me", s.requireAuth(s.handleCurrentUser))
	s.mux.HandleFunc("/api/profile", s.requireAuth(s.handleProfile))
	s.mux.HandleFunc("/api/orgs/registration/start", s.requireAuth(s.handleOrgRegistrationStart))
	s.mux.HandleFunc("/api/orgs/registration/resend", s.requireAuth(s.handleOrgRegistrationResend))
	s.mux.HandleFunc("/api/orgs/registration/complete", s.requireAuth(s.handleOrgRegistrationComplete))
	s.mux.HandleFunc("/api/orgs/invites/accept", s.requireAuth(s.handleOrgInviteAccept))
	s.mux.HandleFunc("/api/orgs/", s.requireAuth(s.handleOrgRoutes))
	s.mux.HandleFunc("/api/projects", s.requireAuth(s.handleConsoleProjectRoutesDispatch))
	s.mux.HandleFunc("/api/projects/", s.requireAuth(s.handleConsoleProjectRoutesDispatch))
	s.mux.HandleFunc("/api/suites/", s.requireAuth(s.handleConsoleSuiteRoutesDispatch))
	s.mux.HandleFunc("/api/runs/", s.requireAuth(s.handleRunRoutesDispatch))
	s.mux.HandleFunc("/api/test-runs/", s.requireAuth(s.handleTestRunRoutesDispatch))

	// Onboarding API
	s.mux.HandleFunc("/api/overview/setup", s.requireAuth(s.handleOverviewSetup))

	// CI Token management
	s.mux.HandleFunc("/api/ci-tokens", s.requireAuth(s.handleCITokensDispatch))
	s.mux.HandleFunc("/api/ci-tokens/", s.requireAuth(s.handleCITokensDispatch))

	// GitHub App routes (for repo access)
	s.mux.HandleFunc("/api/github/app/status", s.requireAuth(s.handleGitHubAppStatus))
	s.mux.HandleFunc("/api/github/app/repos", s.requireAuth(s.handleGitHubAppRepos))
	s.mux.HandleFunc("/api/github/app/repos/connect", s.requireAuth(s.handleGitHubAppConnect))
	s.mux.HandleFunc("/api/github/app/sync", s.requireAuth(s.handleGitHubAppSync))
	s.mux.HandleFunc("/github-app/callback", s.handleGitHubAppCallback)
	s.mux.HandleFunc("/github-app/webhook", s.handleGitHubAppWebhook)
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
		s.cfg.Issuer,                // Self-hosted/staging: allow same-origin deployments
		"http://localhost:5173",     // Vite dev server
		"http://localhost:5174",     // Vite dev server (alt)
		"http://localhost:5175",     // Vite dev server (alt 2)
		"http://localhost:4173",     // Vite preview
		"http://localhost:3000",     // Common React dev port
		"https://app.rocketship.sh", // Production (future)
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
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
