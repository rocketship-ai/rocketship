package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/certs"
	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/rocketship-ai/rocketship/internal/orchestrator"
	"github.com/rocketship-ai/rocketship/internal/rbac"
	"github.com/rocketship-ai/rocketship/internal/tokens"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// Initialize logging
	cli.InitLogging()
	logger := cli.Logger

	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		logger.Error("TEMPORAL_HOST environment variable is not set")
		os.Exit(1)
	}

	logger.Debug("connecting to temporal", "host", temporalHost)
	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		logger.Error("failed to create temporal client", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	// Initialize authentication components
	authManager, tokenManager, rbacRepo := initializeAuth()

	logger.Debug("creating engine orchestrator")
	engine := orchestrator.NewEngine(c, rbacRepo)
	startGRPCServer(engine, authManager, tokenManager, rbacRepo)
}

func initializeAuth() (*auth.Manager, *tokens.Manager, *rbac.Repository) {
	logger := cli.Logger
	
	// Check if authentication is configured
	if !isAuthConfigured() {
		logger.Info("authentication not configured, running in open mode")
		logger.Info("to enable authentication, set ROCKETSHIP_OIDC_ISSUER, ROCKETSHIP_OIDC_CLIENT_ID, and ROCKETSHIP_DB_HOST")
		return nil, nil, nil
	}

	logger.Info("authentication is configured, initializing components")
	logger.Debug("OIDC issuer", "url", os.Getenv("ROCKETSHIP_OIDC_ISSUER"))
	logger.Debug("OIDC client ID", "client_id", os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID"))
	logger.Debug("Database host", "host", os.Getenv("ROCKETSHIP_DB_HOST"))

	// Initialize database connection for auth
	logger.Debug("initializing auth database connection")
	db, err := initDatabase()
	if err != nil {
		logger.Error("failed to initialize auth database", "error", err)
		logger.Error("falling back to open mode due to database connection failure")
		return nil, nil, nil
	}
	logger.Debug("auth database connection established")

	// Initialize RBAC repository
	logger.Debug("initializing RBAC repository")
	rbacRepo := rbac.NewRepository(db)

	// Initialize token manager
	logger.Debug("initializing token manager")
	tokenManager := tokens.NewManager(rbacRepo)

	// Initialize OIDC client with retry logic
	logger.Debug("initializing OIDC client")
	authConfig := &auth.AuthConfig{
		IssuerURL:    os.Getenv("ROCKETSHIP_OIDC_ISSUER"),
		ClientID:     os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("ROCKETSHIP_OIDC_CLIENT_SECRET"),
		RedirectURL:  "", // Not needed for token validation
		Scopes:       []string{"openid", "profile", "email"},
		AdminEmails:  os.Getenv("ROCKETSHIP_ADMIN_EMAILS"),
	}

	// Try to initialize OIDC client with retry
	var oidcClient auth.Client
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		logger.Debug("attempting OIDC client initialization", "attempt", i+1, "max_retries", maxRetries)
		client, err := auth.NewOIDCClient(context.Background(), authConfig)
		if err != nil {
			logger.Error("OIDC client initialization failed", "error", err, "attempt", i+1)
			if i < maxRetries-1 {
				logger.Info("retrying OIDC client initialization in 5 seconds")
				time.Sleep(5 * time.Second)
				continue
			} else {
				logger.Error("failed to initialize OIDC client after retries, falling back to open mode")
				return nil, nil, nil
			}
		}
		oidcClient = client
		logger.Debug("OIDC client initialized successfully")
		break
	}

	// Initialize token storage (use a simple in-memory storage for engine)
	logger.Debug("initializing token storage")
	tokenStorage := &auth.MemoryStorage{}

	// Initialize auth manager
	logger.Debug("initializing auth manager")
	authManager := auth.NewManager(oidcClient, tokenStorage)

	logger.Info("authentication components initialized successfully")
	return authManager, tokenManager, rbacRepo
}

func initDatabase() (*pgxpool.Pool, error) {
	dbHost := os.Getenv("ROCKETSHIP_DB_HOST")
	dbPort := os.Getenv("ROCKETSHIP_DB_PORT")
	dbName := os.Getenv("ROCKETSHIP_DB_NAME")
	dbUser := os.Getenv("ROCKETSHIP_DB_USER")
	dbPassword := os.Getenv("ROCKETSHIP_DB_PASSWORD")

	if dbPort == "" {
		dbPort = "5432"
	}

	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", 
		dbUser, dbPassword, dbHost, dbPort, dbName)
	
	return pgxpool.New(context.Background(), databaseURL)
}

func isAuthConfigured() bool {
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	dbHost := os.Getenv("ROCKETSHIP_DB_HOST")
	
	return issuer != "" && clientID != "" && dbHost != ""
}

func startGRPCServer(engine generated.EngineServer, authManager *auth.Manager, tokenManager *tokens.Manager, rbacRepo *rbac.Repository) {
	logger := cli.Logger

	// Check if TLS is enabled
	tlsEnabledRaw := os.Getenv("ROCKETSHIP_TLS_ENABLED")
	tlsEnabled := tlsEnabledRaw == "true"
	tlsDomain := os.Getenv("ROCKETSHIP_TLS_DOMAIN")
	port := ":7700"
	
	// Explicit debug of TLS environment variables
	logger.Debug("TLS environment check", "raw_enabled", tlsEnabledRaw, "enabled", tlsEnabled, "domain", tlsDomain)
	
	if tlsEnabled && tlsDomain == "" {
		logger.Error("ROCKETSHIP_TLS_ENABLED is true but ROCKETSHIP_TLS_DOMAIN is not set")
		os.Exit(1)
	}

	logger.Debug("starting grpc server", "port", port, "tls", tlsEnabled, "domain", tlsDomain)
	lis, err := net.Listen("tcp", port)
	if err != nil {
		logger.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	// Create gRPC server options
	var grpcOpts []grpc.ServerOption

	// Add TLS if enabled
	if tlsEnabled {
		logger.Info("loading TLS certificate", "domain", tlsDomain)
		
		// Create certificate manager to load certificates
		certManager, err := certs.NewManager(&certs.Config{})
		if err != nil {
			logger.Error("failed to create certificate manager", "error", err)
			os.Exit(1)
		}

		// Load certificate for domain
		cert, err := certManager.GetCertificate(tlsDomain)
		if err != nil {
			logger.Error("failed to load certificate", "domain", tlsDomain, "error", err)
			logger.Info("please generate a certificate first: rocketship certs generate --domain " + tlsDomain)
			os.Exit(1)
		}

		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{*cert},
			ClientAuth:   tls.NoClientCert,
		}

		// Add TLS credentials to gRPC options
		creds := credentials.NewTLS(tlsConfig)
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
		logger.Info("TLS enabled for gRPC server", "domain", tlsDomain)
	}

	// Add authentication interceptors if configured
	if authManager != nil || tokenManager != nil {
		authInterceptor := auth.NewAuthInterceptor(authManager, tokenManager, rbacRepo)
		grpcOpts = append(grpcOpts,
			grpc.UnaryInterceptor(authInterceptor.UnaryInterceptor()),
			grpc.StreamInterceptor(authInterceptor.StreamInterceptor()),
		)
		logger.Info("grpc server configured with authentication")
	} else {
		logger.Info("grpc server configured in open mode (no authentication)")
	}

	// Create gRPC server with options
	grpcServer := grpc.NewServer(grpcOpts...)
	generated.RegisterEngineServer(grpcServer, engine)

	if tlsEnabled {
		logger.Info("grpc server listening with TLS", "port", port, "domain", tlsDomain)
	} else {
		logger.Info("grpc server listening", "port", port)
	}
	
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("failed to serve grpc", "error", err)
		os.Exit(1)
	}
}
