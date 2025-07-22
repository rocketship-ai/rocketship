package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/certs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type EngineClient struct {
	client generated.EngineClient
	conn   *grpc.ClientConn
}

func NewEngineClient(address string) (*EngineClient, error) {
	return NewEngineClientWithProfile(address, "")
}

func NewEngineClientWithProfile(address, profileName string) (*EngineClient, error) {
	Logger.Debug("connecting to engine", "address", address)

	// Load configuration to check for profile settings
	config, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load config, using environment variables", "error", err)
		return newEngineClientFromEnv(address)
	}
	
	// Get the profile to use
	var profile *Profile
	if profileName != "" {
		profile, err = config.GetProfile(profileName)
		if err != nil {
			return nil, fmt.Errorf("profile '%s' not found: %w", profileName, err)
		}
	} else {
		profile = config.GetActiveProfile()
	}
	
	// Use profile address if no explicit address provided
	if address == "" {
		address = profile.EngineAddress
	}
	
	Logger.Debug("using profile for connection", "profile", profile.Name, "address", address)
	
	// Get TLS settings from profile or environment
	tlsEnabled := profile.TLS.Enabled
	tlsDomain := profile.TLS.Domain
	
	// Environment variables can override profile settings
	if os.Getenv("ROCKETSHIP_TLS_ENABLED") != "" {
		tlsEnabled = os.Getenv("ROCKETSHIP_TLS_ENABLED") == "true"
	}
	if os.Getenv("ROCKETSHIP_TLS_DOMAIN") != "" {
		tlsDomain = os.Getenv("ROCKETSHIP_TLS_DOMAIN")
	}
	
	// Base dial options
	var dialOpts []grpc.DialOption
	
	if tlsEnabled {
		Logger.Debug("TLS enabled for gRPC client", "domain", tlsDomain)
		
		if tlsDomain == "" {
			return nil, fmt.Errorf("ROCKETSHIP_TLS_ENABLED is true but ROCKETSHIP_TLS_DOMAIN is not set")
		}
		
		// Determine if we should use custom certificate or system CA
		useCustomCert := shouldUseCustomCert(tlsDomain)
		
		if useCustomCert {
			Logger.Debug("loading custom certificate for TLS connection", "domain", tlsDomain)
			
			// Create certificate manager to load certificates
			certManager, err := certs.NewManager(&certs.Config{})
			if err != nil {
				return nil, fmt.Errorf("failed to create certificate manager: %w", err)
			}
			
			// Load certificate for domain
			cert, err := certManager.GetCertificate(tlsDomain)
			if err != nil {
				return nil, fmt.Errorf("failed to load certificate for domain %s: %w", tlsDomain, err)
			}
			
			// Create TLS config with custom certificate
			tlsConfig := &tls.Config{
				Certificates:       []tls.Certificate{*cert},
				InsecureSkipVerify: true, // Skip verification for self-signed certs
				ServerName:         tlsDomain,
			}
			
			creds := credentials.NewTLS(tlsConfig)
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		} else {
			Logger.Debug("using system CA for TLS connection", "domain", tlsDomain)
			
			// Use system CA certificates
			tlsConfig := &tls.Config{
				ServerName: tlsDomain,
			}
			
			creds := credentials.NewTLS(tlsConfig)
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		}
	} else {
		Logger.Debug("TLS disabled, using insecure connection")
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Check if authentication is configured
	authConfig := getProfileAuthConfig(profile)
	if isAuthConfiguredForProfile(authConfig) {
		Logger.Debug("authentication is configured, setting up auth interceptor")
		
		// Create token provider function
		tokenProvider := func(ctx context.Context) (string, error) {
			return getAccessTokenForProfile(ctx, profile)
		}

		// Create auth interceptor
		authInterceptor := auth.NewClientInterceptor(tokenProvider)
		
		// Add interceptors to dial options
		dialOpts = append(dialOpts,
			grpc.WithUnaryInterceptor(authInterceptor.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(authInterceptor.StreamClientInterceptor()),
		)
	} else {
		Logger.Debug("authentication not configured, connecting without auth")
	}

	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("connection timed out - is the engine running at %s?", address)
		}
		return nil, fmt.Errorf("failed to connect to engine: %w", err)
	}

	client := generated.NewEngineClient(conn)
	return &EngineClient{
		client: client,
		conn:   conn,
	}, nil
}

func (c *EngineClient) Close() error {
	return c.conn.Close()
}

// HealthCheck performs a health check against the engine
func (c *EngineClient) HealthCheck(ctx context.Context) error {
	resp, err := c.client.Health(ctx, &generated.HealthRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("engine reported unhealthy status: %s", resp.Status)
	}

	return nil
}

func (c *EngineClient) RunTest(ctx context.Context, yamlData []byte) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.CreateRun(runCtx, &generated.CreateRunRequest{
		YamlPayload: yamlData,
	})
	if err != nil {
		if err == context.DeadlineExceeded {
			return "", fmt.Errorf("timed out waiting for engine to respond")
		}
		if s, ok := status.FromError(err); ok {
			return "", fmt.Errorf("failed to create run: %s", s.Message())
		}
		return "", fmt.Errorf("failed to create run: %w", err)
	}

	return resp.RunId, nil
}

func (c *EngineClient) RunTestWithContext(ctx context.Context, yamlData []byte, runCtx *generated.RunContext) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.CreateRun(reqCtx, &generated.CreateRunRequest{
		YamlPayload: yamlData,
		Context:     runCtx,
	})
	if err != nil {
		if err == context.DeadlineExceeded {
			return "", fmt.Errorf("timed out waiting for engine to respond")
		}
		if s, ok := status.FromError(err); ok {
			return "", fmt.Errorf("failed to create run: %s", s.Message())
		}
		return "", fmt.Errorf("failed to create run: %w", err)
	}

	return resp.RunId, nil
}

func (c *EngineClient) StreamLogs(ctx context.Context, runID string) (generated.Engine_StreamLogsClient, error) {
	return c.client.StreamLogs(ctx, &generated.LogStreamRequest{
		RunId: runID,
	})
}

func (c *EngineClient) AddLog(ctx context.Context, runID, workflowID, message, color string, bold bool) error {
	_, err := c.client.AddLog(ctx, &generated.AddLogRequest{
		RunId:      runID,
		WorkflowId: workflowID,
		Message:    message,
		Color:      color,
		Bold:       bold,
	})
	return err
}

func (c *EngineClient) AddLogWithContext(ctx context.Context, runID, workflowID, message, color string, bold bool, testName, stepName string) error {
	_, err := c.client.AddLog(ctx, &generated.AddLogRequest{
		RunId:      runID,
		WorkflowId: workflowID,
		Message:    message,
		Color:      color,
		Bold:       bold,
		TestName:   testName,
		StepName:   stepName,
	})
	return err
}

func (c *EngineClient) CancelRun(ctx context.Context, runID string) error {
	cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	resp, err := c.client.CancelRun(cancelCtx, &generated.CancelRunRequest{
		RunId: runID,
	})
	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("timed out waiting for engine to cancel run")
		}
		if s, ok := status.FromError(err); ok {
			return fmt.Errorf("failed to cancel run: %s", s.Message())
		}
		return fmt.Errorf("failed to cancel run: %w", err)
	}
	
	if !resp.Success {
		return fmt.Errorf("failed to cancel run: %s", resp.Message)
	}
	
	Logger.Info("Run cancelled successfully", "run_id", runID, "message", resp.Message)
	return nil
}

// isAuthConfigured checks if authentication is configured
func isAuthConfigured() bool {
	// Use external issuer for CLI clients
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER_EXTERNAL")
	if issuer == "" {
		issuer = os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	}
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	
	return issuer != "" && clientID != ""
}

// getAccessToken retrieves the current access token
func getAccessToken(ctx context.Context) (string, error) {
	// Get auth configuration
	config, err := getAuthConfig()
	if err != nil {
		// If auth is not configured, return empty token (no auth)
		return "", nil
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		// If we can't create OIDC client (e.g., OIDC provider not reachable), 
		// return empty token to allow unauthenticated access
		Logger.Debug("failed to create OIDC client, proceeding without auth", "error", err)
		return "", nil
	}

	// Create keyring storage
	storage := auth.NewKeyringStorage()

	// Create auth manager
	manager := auth.NewManager(client, storage)

	// Check if authenticated
	if !manager.IsAuthenticated(ctx) {
		// Not authenticated, return empty token
		return "", nil
	}

	// Get valid token
	token, err := manager.GetValidToken(ctx)
	if err != nil {
		// If we can't get a valid token, return empty
		Logger.Debug("failed to get valid token, proceeding without auth", "error", err)
		return "", nil
	}

	return token, nil
}

// shouldUseCustomCert determines if we should use a custom certificate
// Returns true for localhost, self-signed domains, or when we have custom certs
func shouldUseCustomCert(domain string) bool {
	// Always use custom certs for localhost
	if strings.HasPrefix(domain, "localhost") || domain == "127.0.0.1" {
		return true
	}
	
	// Check if we have a custom certificate directory
	certManager, err := certs.NewManager(&certs.Config{})
	if err != nil {
		return false
	}
	
	// Try to load the certificate - if it exists, use it
	_, err = certManager.GetCertificate(domain)
	return err == nil
}

// newEngineClientFromEnv creates a client using only environment variables (fallback)
func newEngineClientFromEnv(address string) (*EngineClient, error) {
	Logger.Debug("creating client from environment variables")
	
	// Check if TLS is enabled
	tlsEnabled := os.Getenv("ROCKETSHIP_TLS_ENABLED") == "true"
	tlsDomain := os.Getenv("ROCKETSHIP_TLS_DOMAIN")
	
	// Base dial options
	var dialOpts []grpc.DialOption
	
	if tlsEnabled {
		Logger.Debug("TLS enabled for gRPC client", "domain", tlsDomain)
		
		if tlsDomain == "" {
			return nil, fmt.Errorf("ROCKETSHIP_TLS_ENABLED is true but ROCKETSHIP_TLS_DOMAIN is not set")
		}
		
		// Determine if we should use custom certificate or system CA
		useCustomCert := shouldUseCustomCert(tlsDomain)
		
		if useCustomCert {
			Logger.Debug("loading custom certificate for TLS connection", "domain", tlsDomain)
			
			// Create certificate manager to load certificates
			certManager, err := certs.NewManager(&certs.Config{})
			if err != nil {
				return nil, fmt.Errorf("failed to create certificate manager: %w", err)
			}
			
			// Load certificate for domain
			cert, err := certManager.GetCertificate(tlsDomain)
			if err != nil {
				return nil, fmt.Errorf("failed to load certificate for domain %s: %w", tlsDomain, err)
			}
			
			// Create TLS config with custom certificate
			tlsConfig := &tls.Config{
				Certificates:       []tls.Certificate{*cert},
				InsecureSkipVerify: true, // Skip verification for self-signed certs
				ServerName:         tlsDomain,
			}
			
			creds := credentials.NewTLS(tlsConfig)
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		} else {
			Logger.Debug("using system CA for TLS connection", "domain", tlsDomain)
			
			// Use system CA certificates
			tlsConfig := &tls.Config{
				ServerName: tlsDomain,
			}
			
			creds := credentials.NewTLS(tlsConfig)
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		}
	} else {
		Logger.Debug("TLS disabled, using insecure connection")
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Check if authentication is configured
	if isAuthConfigured() {
		Logger.Debug("authentication is configured, setting up auth interceptor")
		
		// Create token provider function
		tokenProvider := func(ctx context.Context) (string, error) {
			return getAccessToken(ctx)
		}

		// Create auth interceptor
		authInterceptor := auth.NewClientInterceptor(tokenProvider)
		
		// Add interceptors to dial options
		dialOpts = append(dialOpts,
			grpc.WithUnaryInterceptor(authInterceptor.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(authInterceptor.StreamClientInterceptor()),
		)
	} else {
		Logger.Debug("authentication not configured, connecting without auth")
	}

	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("connection timed out - is the engine running at %s?", address)
		}
		return nil, fmt.Errorf("failed to connect to engine: %w", err)
	}

	client := generated.NewEngineClient(conn)
	return &EngineClient{
		client: client,
		conn:   conn,
	}, nil
}

// getProfileAuthConfig returns auth configuration from profile or environment
func getProfileAuthConfig(profile *Profile) *auth.AuthConfig {
	if profile == nil {
		return getAuthConfigFromEnv()
	}
	
	// Use profile auth config if available
	if profile.Auth.Issuer != "" && profile.Auth.ClientID != "" {
		return &auth.AuthConfig{
			IssuerURL:    profile.Auth.Issuer,
			ClientID:     profile.Auth.ClientID,
			ClientSecret: profile.Auth.ClientSecret,
			RedirectURL:  "http://localhost:8000/callback",
			Scopes:       []string{"openid", "profile", "email"},
			AdminEmails:  profile.Auth.AdminEmails,
		}
	}
	
	// Fall back to environment variables
	return getAuthConfigFromEnv()
}

// getAuthConfigFromEnv gets auth config from environment variables
func getAuthConfigFromEnv() *auth.AuthConfig {
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER_EXTERNAL")
	if issuer == "" {
		issuer = os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	}
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	
	if issuer == "" || clientID == "" {
		return nil
	}
	
	return &auth.AuthConfig{
		IssuerURL:    issuer,
		ClientID:     clientID,
		ClientSecret: os.Getenv("ROCKETSHIP_OIDC_CLIENT_SECRET"),
		RedirectURL:  "http://localhost:8000/callback",
		Scopes:       []string{"openid", "profile", "email"},
		AdminEmails:  os.Getenv("ROCKETSHIP_ADMIN_EMAILS"),
	}
}

// isAuthConfiguredForProfile checks if authentication is configured for a profile
func isAuthConfiguredForProfile(authConfig *auth.AuthConfig) bool {
	return authConfig != nil && authConfig.IssuerURL != "" && authConfig.ClientID != ""
}

// getAccessTokenForProfile retrieves the access token for a specific profile
func getAccessTokenForProfile(ctx context.Context, profile *Profile) (string, error) {
	// Get auth configuration for the profile
	authConfig := getProfileAuthConfig(profile)
	if authConfig == nil {
		return "", nil
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		Logger.Debug("failed to create OIDC client, proceeding without auth", "error", err)
		return "", nil
	}

	// Create keyring storage with profile-specific key
	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)

	// Create auth manager
	manager := auth.NewManager(client, storage)

	// Check if authenticated
	if !manager.IsAuthenticated(ctx) {
		return "", nil
	}

	// Get valid token
	token, err := manager.GetValidToken(ctx)
	if err != nil {
		Logger.Debug("failed to get valid token, proceeding without auth", "error", err)
		return "", nil
	}

	return token, nil
}

