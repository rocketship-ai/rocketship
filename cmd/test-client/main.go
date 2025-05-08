package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Read the YAML file
	yamlContent, err := os.ReadFile("examples/simple-http/rocketship.yaml")
	if err != nil {
		log.Fatalf("Failed to read YAML file: %v", err)
	}

	// Connect to the engine
	conn, err := grpc.NewClient("localhost:7700", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to engine: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Create client
	client := generated.NewEngineClient(conn)

	// Create run request
	req := &generated.CreateRunRequest{
		YamlPayload: yamlContent,
	}

	// Send request
	resp, err := client.CreateRun(context.Background(), req)
	if err != nil {
		log.Fatalf("Failed to create run: %v", err)
	}

	fmt.Printf("Run created with ID: %s\n", resp.RunId)

	// Stream logs
	stream, err := client.StreamLogs(context.Background(), &generated.LogStreamRequest{
		RunId: resp.RunId,
	})
	if err != nil {
		log.Fatalf("Failed to stream logs: %v", err)
	}

	for {
		logLine, err := stream.Recv()
		if err != nil {
			break
		}
		fmt.Printf("[%s] %s\n", logLine.Ts, logLine.Msg)
	}
}
