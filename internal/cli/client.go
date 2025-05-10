package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type EngineClient struct {
	client generated.EngineClient
	conn   *grpc.ClientConn
}

func NewEngineClient(address string) (*EngineClient, error) {
	fmt.Printf("Connecting to engine at %s...\n", address)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

func (c *EngineClient) RunTest(ctx context.Context, yamlData []byte) (string, error) {
	fmt.Println("Creating run...")

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

func (c *EngineClient) StreamLogs(ctx context.Context, runID string) (generated.Engine_StreamLogsClient, error) {
	return c.client.StreamLogs(ctx, &generated.LogStreamRequest{
		RunId: runID,
	})
}
