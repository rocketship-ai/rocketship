package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

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

	token, err := loadEngineToken()
	if err != nil {
		logger.Error("failed to load token", "error", err)
		os.Exit(1)
	}
	engine.MustConfigureToken(token)
	if engine.TokenAuthEnabled() {
		logger.Info("token authentication enabled")
	}
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
