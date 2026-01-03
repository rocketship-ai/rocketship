package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
	"github.com/rocketship-ai/rocketship/internal/orchestrator"
	"go.temporal.io/sdk/client"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
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
	temporalNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if temporalNamespace == "" {
		temporalNamespace = "default"
	}

	logger.Debug("connecting to temporal", "host", temporalHost)
	c, err := client.Dial(client.Options{
		HostPort:  temporalHost,
		Namespace: temporalNamespace,
	})
	if err != nil {
		logger.Error("failed to create temporal client", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	logger.Debug("loading engine database configuration")
	var (
		runStore        orchestrator.RunStore
		requireOrgScope bool
	)
	dbURL := strings.TrimSpace(os.Getenv("ROCKETSHIP_ENGINE_DATABASE_URL"))
	if dbURL == "" {
		logger.Warn("ROCKETSHIP_ENGINE_DATABASE_URL not set; using in-memory run store")
		runStore = orchestrator.NewMemoryRunStore()
		requireOrgScope = false
	} else {
		storeCtx, storeCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer storeCancel()
		dbStore, err := persistence.NewStore(storeCtx, dbURL, nil)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := dbStore.Close(); err != nil {
				logger.Debug("failed to close run store", "error", err)
			}
		}()
		runStore = dbStore
		requireOrgScope = true
	}

	logger.Debug("creating engine orchestrator")
	engine := orchestrator.NewEngine(c, runStore, requireOrgScope)

	if err := configureAuthentication(engine); err != nil {
		logger.Error("failed to configure authentication", "error", err)
		os.Exit(1)
	}
	logger.Info("authentication configured", "mode", engine.AuthMode())

	// Start the scheduler if we have a database store that supports scheduling
	var scheduler *orchestrator.Scheduler
	if schedulerStore, ok := runStore.(orchestrator.SchedulerStore); ok {
		schedLogger := slog.Default().With("component", "scheduler")
		scheduler = orchestrator.NewScheduler(engine, schedulerStore, schedLogger)
		scheduler.Start()
		logger.Info("scheduler started")
	} else {
		logger.Debug("scheduler disabled (no database store)")
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig)
		if scheduler != nil {
			scheduler.Stop()
		}
		os.Exit(0)
	}()

	startHealthServer()
	startGRPCServer(engine)
}

type healthPayload struct {
	Status string `json:"status"`
}

func newHealthMux() http.Handler {
	mux := http.NewServeMux()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(healthPayload{Status: "ok"})
	})

	mux.HandleFunc("/", handler)
	mux.HandleFunc("/healthz", handler)
	return mux
}

func startGRPCServer(engine *orchestrator.Engine) {
	logger := cli.Logger

	lis, err := net.Listen("tcp", ":7700")
	if err != nil {
		logger.Error("failed to listen on port 7700", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(engine.NewAuthUnaryInterceptor()),
		grpc.ChainStreamInterceptor(engine.NewAuthStreamInterceptor()),
	)
	generated.RegisterEngineServer(grpcServer, engine)

	// Check if grpc-web should be disabled (for local development with kubectl port-forward)
	disableGrpcWeb := strings.ToLower(strings.TrimSpace(os.Getenv("ROCKETSHIP_DISABLE_GRPC_WEB"))) == "true"

	if disableGrpcWeb {
		// Serve pure gRPC without HTTP multiplexer
		// This is required for kubectl port-forward compatibility
		logger.Info("grpc server listening (native gRPC only, grpc-web disabled)", "port", ":7700")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("failed to serve", "error", err)
			os.Exit(1)
		}
		return
	}

	// Production mode: wrap gRPC server with grpc-web for browser compatibility
	logger.Debug("starting grpc server with grpc-web support", "port", ":7700")
	wrappedServer := grpcweb.WrapServer(grpcServer,
		grpcweb.WithOriginFunc(isAllowedOrigin),
		grpcweb.WithWebsockets(true), // Enable WebSocket for streaming
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return isAllowedOrigin(req.Header.Get("Origin"))
		}),
	)

	// Create HTTP handler that routes based on request type
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wrappedServer.IsGrpcWebRequest(r) ||
			wrappedServer.IsAcceptableGrpcCorsRequest(r) ||
			wrappedServer.IsGrpcWebSocketRequest(r) {
			// Handle browser requests (grpc-web over HTTP/1.1)
			wrappedServer.ServeHTTP(w, r)
		} else {
			// Handle native gRPC requests (CLI over HTTP/2)
			grpcServer.ServeHTTP(w, r)
		}
	})

	// Wrap with h2c to support both HTTP/1.1 (gRPC-Web) and HTTP/2 (native gRPC)
	h2s := &http2.Server{}
	httpServer := &http.Server{
		Addr:    ":7700",
		Handler: h2c.NewHandler(handler, h2s),
	}

	logger.Info("grpc server listening (native gRPC over h2c + grpc-web)", "port", ":7700")
	if err := httpServer.Serve(lis); err != nil {
		logger.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}

// isAllowedOrigin checks if the request origin is allowed for CORS
func isAllowedOrigin(origin string) bool {
	allowedOrigins := []string{
		"http://auth.minikube.local", // Local development (single-origin through ingress)
		"https://app.rocketship.sh",  // Production
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	// Also check environment variable for custom origins
	customOrigin := strings.TrimSpace(os.Getenv("ROCKETSHIP_ALLOWED_ORIGIN"))
	if customOrigin != "" && origin == customOrigin {
		return true
	}

	return false
}

func loadEngineToken() (string, error) {
	if path := strings.TrimSpace(os.Getenv("ROCKETSHIP_ENGINE_TOKEN_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read ROCKETSHIP_ENGINE_TOKEN_FILE: %w", err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("ROCKETSHIP_ENGINE_TOKEN_FILE %q is empty", path)
		}
		return token, nil
	}

	token := strings.TrimSpace(os.Getenv("ROCKETSHIP_ENGINE_TOKEN"))
	return token, nil
}

func configureAuthentication(engine *orchestrator.Engine) error {
	token, err := loadEngineToken()
	if err != nil {
		return err
	}

	mode := strings.ToLower(strings.TrimSpace(os.Getenv("ROCKETSHIP_AUTH_MODE")))
	issuer := strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_ISSUER"))
	if mode == "oidc" || issuer != "" {
		settings, err := loadOIDCSettings(issuer)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := engine.ConfigureOIDC(ctx, settings); err != nil {
			return err
		}
		return nil
	}

	engine.ConfigureToken(token)
	return nil
}

func loadOIDCSettings(explicitIssuer string) (orchestrator.OIDCSettings, error) {
	issuer := strings.TrimSpace(explicitIssuer)
	if issuer == "" {
		issuer = strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_ISSUER"))
	}
	if issuer == "" {
		return orchestrator.OIDCSettings{}, fmt.Errorf("ROCKETSHIP_OIDC_ISSUER is required for oidc auth")
	}
	clientID := strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID"))
	if clientID == "" {
		return orchestrator.OIDCSettings{}, fmt.Errorf("ROCKETSHIP_OIDC_CLIENT_ID is required for oidc auth")
	}

	settings := orchestrator.OIDCSettings{
		Issuer:         issuer,
		ClientID:       clientID,
		Audience:       strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_AUDIENCE")),
		DeviceEndpoint: strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_DEVICE_ENDPOINT")),
		TokenEndpoint:  strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_TOKEN_ENDPOINT")),
		JWKSURL:        strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_JWKS_URL")),
		Scopes:         parseScopes(os.Getenv("ROCKETSHIP_OIDC_SCOPES")),
	}

	if algs := strings.TrimSpace(os.Getenv("ROCKETSHIP_OIDC_ALLOWED_ALGS")); algs != "" {
		settings.AllowedAlgorithms = splitAndTrim(algs)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	settings.HTTPClient = client

	if settings.JWKSURL == "" || settings.TokenEndpoint == "" || settings.DeviceEndpoint == "" || len(settings.Scopes) == 0 {
		doc, err := fetchOIDCDiscovery(context.Background(), client, issuer)
		if err != nil {
			return orchestrator.OIDCSettings{}, err
		}
		if settings.JWKSURL == "" {
			settings.JWKSURL = doc.JWKSURI
		}
		if settings.TokenEndpoint == "" {
			settings.TokenEndpoint = doc.TokenEndpoint
		}
		if settings.DeviceEndpoint == "" {
			settings.DeviceEndpoint = doc.DeviceEndpoint
		}
		if len(settings.Scopes) == 0 && len(doc.ScopesSupported) > 0 {
			settings.Scopes = append([]string(nil), doc.ScopesSupported...)
		}
	}

	if len(settings.Scopes) == 0 {
		settings.Scopes = []string{"openid", "profile", "email", "offline_access"}
	}

	if settings.JWKSURL == "" {
		return orchestrator.OIDCSettings{}, fmt.Errorf("jwks_uri missing from discovery")
	}
	if settings.TokenEndpoint == "" {
		return orchestrator.OIDCSettings{}, fmt.Errorf("token_endpoint missing from discovery")
	}
	if settings.DeviceEndpoint == "" {
		return orchestrator.OIDCSettings{}, fmt.Errorf("device_authorization_endpoint missing from discovery")
	}

	return settings, nil
}

type discoveryDocument struct {
	JWKSURI         string   `json:"jwks_uri"`
	TokenEndpoint   string   `json:"token_endpoint"`
	DeviceEndpoint  string   `json:"device_authorization_endpoint"`
	ScopesSupported []string `json:"scopes_supported"`
}

func fetchOIDCDiscovery(ctx context.Context, client *http.Client, issuer string) (*discoveryDocument, error) {
	endpoint := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build discovery request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			cli.Logger.Debug("failed to close discovery response", "error", cerr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery request failed: %s", resp.Status)
	}
	var doc discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}
	return &doc, nil
}

func parseScopes(raw string) []string {
	parts := splitAndTrim(raw)
	return parts
}

func splitAndTrim(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\n' || r == '\t'
	})
	var out []string
	for _, f := range fields {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func startHealthServer() {
	logger := cli.Logger
	mux := newHealthMux()
	server := &http.Server{
		Addr:    ":7701",
		Handler: mux,
	}

	logger.Info("http health server listening", "port", ":7701")
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server error", "error", err)
		}
	}()
}
