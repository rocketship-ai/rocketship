package main

import (
	"log"
	"net"
	"os"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc"
)

func main() {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		panic("TEMPORAL_HOST is not set")
	}

	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	engine := orchestrator.NewEngine(c)
	startGRPCServer(engine)
}

func startGRPCServer(engine generated.EngineServer) {
	lis, err := net.Listen("tcp", ":7700")
	if err != nil {
		log.Fatalf("Failed to listen on port 7700: %v", err)
	}

	grpcServer := grpc.NewServer()
	generated.RegisterEngineServer(grpcServer, engine)

	log.Println("gRPC server listening on :7700 !")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}
