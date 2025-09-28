package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/rocketship-ai/rocketship/internal/orchestrator"
	"go.temporal.io/sdk/client"
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

	logger.Debug("creating engine orchestrator")
	engine := orchestrator.NewEngine(c)

	if err := configureAuthentication(engine); err != nil {
		logger.Error("failed to configure authentication", "error", err)
		os.Exit(1)
	}
	logger.Info("authentication configured", "mode", engine.AuthMode())
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

	logger.Debug("starting grpc server", "port", ":7700")
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

	logger.Info("grpc server listening", "port", ":7700")
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("failed to serve grpc", "error", err)
		os.Exit(1)
	}
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
