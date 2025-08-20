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

// ResolveEngineAddress determines the engine address to use based on:
// 1. Explicit address (if provided)
// 2. Active profile's engine address
// 3. Default localhost:7700
func ResolveEngineAddress(explicitAddress string) (string, error) {
	// If an explicit address is provided, use it
	if explicitAddress != "" {
		Logger.Debug("Using explicitly provided engine address", "address", explicitAddress)
		return explicitAddress, nil
	}

	// Try to load config and use active profile
	config, err := LoadConfig()
	if err != nil {
		// Config load failed, use default
		Logger.Debug("Failed to load config, using default address", "error", err, "address", "localhost:7700")
		return "localhost:7700", nil
	}

	// Check if there's an active profile
	if config.DefaultProfile != "" {
		profile, exists := config.GetProfile(config.DefaultProfile)
		if exists && profile.EngineAddress != "" {
			Logger.Debug("Using engine address from active profile", "profile", profile.Name, "address", profile.EngineAddress)
			return profile.EngineAddress, nil
		} else {
			Logger.Debug("Active profile has no engine address, using default", "profile", config.DefaultProfile, "address", "localhost:7700")
		}
	} else {
		Logger.Debug("No active profile, using default address", "address", "localhost:7700")
	}

	return "localhost:7700", nil
}

func NewEngineClient(address string) (*EngineClient, error) {
	// Resolve the actual address to use
	resolvedAddress, err := ResolveEngineAddress(address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve engine address: %w", err)
	}

	Logger.Debug("connecting to engine", "address", resolvedAddress)

	conn, err := grpc.NewClient(resolvedAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("connection timed out - is the engine running at %s?", resolvedAddress)
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

func (c *EngineClient) RunTestWithContext(ctx context.Context, yamlData []byte, runCtx *generated.RunContext) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.CreateRun(reqCtx, &generated.CreateRunRequest{
		YamlPayload: yamlData,
		Context:     runCtx,
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

func (c *EngineClient) AddLogWithContext(ctx context.Context, runID, workflowID, message, color string, bold bool, testName, stepName string) error {
	_, err := c.client.AddLog(ctx, &generated.AddLogRequest{
		RunId:      runID,
		WorkflowId: workflowID,
		Message:    message,
		Color:      color,
		Bold:       bold,
		TestName:   testName,
		StepName:   stepName,
	})
	return err
}

func (c *EngineClient) CancelRun(ctx context.Context, runID string) error {
	cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	resp, err := c.client.CancelRun(cancelCtx, &generated.CancelRunRequest{
		RunId: runID,
	})
	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("timed out waiting for engine to cancel run")
		}
		if s, ok := status.FromError(err); ok {
			return fmt.Errorf("failed to cancel run: %s", s.Message())
		}
		return fmt.Errorf("failed to cancel run: %w", err)
	}
	
	if !resp.Success {
		return fmt.Errorf("failed to cancel run: %s", resp.Message)
	}
	
	Logger.Info("Run cancelled successfully", "run_id", runID, "message", resp.Message)
	return nil
}

// ServerInfo represents server auth capabilities
type ServerInfo struct {
	AuthEnabled   bool
	AuthType      string // "none", "cloud", "oidc", "token"
	AuthEndpoint  string // OAuth/OIDC endpoint for authentication flows
	WorkspaceId   string // Current workspace/tenant ID (if authenticated)
	UserId        string // Current user ID (if authenticated)
}

// GetServerInfo gets server capabilities and configuration
func (c *EngineClient) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	infoCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	
	resp, err := c.client.GetAuthConfig(infoCtx, &generated.GetAuthConfigRequest{})
	if err != nil {
		// Gracefully handle engines that don't support GetAuthConfig (pre-profile system)
		if s, ok := status.FromError(err); ok {
			if s.Code() == 12 { // UNIMPLEMENTED
				Logger.Debug("Engine doesn't support GetAuthConfig, assuming local-only")
				return &ServerInfo{
					AuthEnabled:  false,
					AuthType:     "none",
					AuthEndpoint: "",
					WorkspaceId:  "",
					UserId:       "",
				}, nil
			}
		}
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}
	
	return &ServerInfo{
		AuthEnabled:  resp.AuthEnabled,
		AuthType:     resp.AuthType,
		AuthEndpoint: resp.AuthEndpoint,
		WorkspaceId:  resp.WorkspaceId,
		UserId:       resp.UserId,
	}, nil
}
