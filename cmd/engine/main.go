package main

import (
	"net"
	"os"

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

	logger.Debug("connecting to temporal", "host", temporalHost)
	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		logger.Error("failed to create temporal client", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	logger.Debug("creating engine orchestrator")
	engine := orchestrator.NewEngine(c)
	startGRPCServer(engine)
}

func startGRPCServer(engine generated.EngineServer) {
	logger := cli.Logger

	logger.Debug("starting grpc server", "port", ":7700")
	lis, err := net.Listen("tcp", ":7700")
	if err != nil {
		logger.Error("failed to listen on port 7700", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	generated.RegisterEngineServer(grpcServer, engine)

	logger.Info("grpc server listening", "port", ":7700")
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("failed to serve grpc", "error", err)
		os.Exit(1)
	}
}
