package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type EngineClient struct {
	client generated.EngineClient
	conn   *grpc.ClientConn
}

func NewEngineClient(address string) (*EngineClient, error) {
	fmt.Printf("Attempting to connect to %s...\n", address)

	// Create a context with timeout for the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithBlock(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("connection timed out after 5 seconds - is the engine running at %s?", address)
		}
		return nil, fmt.Errorf("failed to connect to engine: %w", err)
	}

	fmt.Println("Successfully connected to engine")

	client := generated.NewEngineClient(conn)
	return &EngineClient{
		client: client,
		conn:   conn,
	}, nil
}

func (c *EngineClient) Close() error {
	fmt.Println("Closing gRPC connection...")
	return c.conn.Close()
}

func (c *EngineClient) RunTest(ctx context.Context, yamlData []byte) (string, error) {
	fmt.Printf("Sending run request with %d bytes of YAML data...\n", len(yamlData))

	// Create a new context with timeout for the CreateRun call
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Add deadline info to the context
	deadline, _ := runCtx.Deadline()
	fmt.Printf("Run request deadline: %v\n", deadline)

	resp, err := c.client.CreateRun(runCtx, &generated.CreateRunRequest{
		YamlPayload: yamlData,
	})
	if err != nil {
		if err == context.DeadlineExceeded {
			return "", fmt.Errorf("run request timed out after 10 seconds - engine may be unresponsive")
		}
		if s, ok := status.FromError(err); ok {
			return "", fmt.Errorf("run request failed with status %s: %s", s.Code(), s.Message())
		}
		return "", fmt.Errorf("failed to create run: %w", err)
	}

	fmt.Printf("Successfully created run with ID: %s\n", resp.RunId)
	return resp.RunId, nil
}

func (c *EngineClient) StreamLogs(ctx context.Context, runID string) (generated.Engine_StreamLogsClient, error) {
	fmt.Printf("Starting log stream for run %s...\n", runID)
	return c.client.StreamLogs(ctx, &generated.LogStreamRequest{
		RunId: runID,
	})
}
