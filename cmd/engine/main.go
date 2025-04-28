package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc"
)

func main() {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		temporalHost = "localhost:7233"
	}

	c, err := client.NewClient(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	engine := orchestrator.NewEngine(c)

	go startGRPCServer(engine)

	startHTTPServer()
}

func startGRPCServer(engine generated.EngineServer) {
	lis, err := net.Listen("tcp", ":7700")
	if err != nil {
		log.Fatalf("Failed to listen on port 7700: %v", err)
	}

	grpcServer := grpc.NewServer()
	generated.RegisterEngineServer(grpcServer, engine)

	log.Println("gRPC server listening on :7700")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}

func startHTTPServer() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "pong")
	})

	log.Println("HTTP server listening on :7701")
	if err := http.ListenAndServe(":7701", nil); err != nil {
		log.Fatalf("Failed to serve HTTP: %v", err)
	}
}
