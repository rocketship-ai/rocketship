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
	Logger.Debug("connecting to engine", "address", address)

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

// HealthCheck performs a health check against the engine
func (c *EngineClient) HealthCheck(ctx context.Context) error {
	resp, err := c.client.Health(ctx, &generated.HealthRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("engine reported unhealthy status: %s", resp.Status)
	}

	return nil
}

func (c *EngineClient) RunTest(ctx context.Context, yamlData []byte) (string, error) {
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

func (c *EngineClient) AddLog(ctx context.Context, runID, workflowID, message, color string, bold bool) error {
	_, err := c.client.AddLog(ctx, &generated.AddLogRequest{
		RunId:      runID,
		WorkflowId: workflowID,
		Message:    message,
		Color:      color,
		Bold:       bold,
	})
	return err
}
