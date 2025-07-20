package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
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

	// Base dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Check if authentication is configured
	if isAuthConfigured() {
		Logger.Debug("authentication is configured, setting up auth interceptor")
		
		// Create token provider function
		tokenProvider := func(ctx context.Context) (string, error) {
			return getAccessToken(ctx)
		}

		// Create auth interceptor
		authInterceptor := auth.NewClientInterceptor(tokenProvider)
		
		// Add interceptors to dial options
		dialOpts = append(dialOpts,
			grpc.WithUnaryInterceptor(authInterceptor.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(authInterceptor.StreamClientInterceptor()),
		)
	} else {
		Logger.Debug("authentication not configured, connecting without auth")
	}

	conn, err := grpc.NewClient(address, dialOpts...)
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

// isAuthConfigured checks if authentication is configured
func isAuthConfigured() bool {
	// Use external issuer for CLI clients
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER_EXTERNAL")
	if issuer == "" {
		issuer = os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	}
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	
	return issuer != "" && clientID != ""
}

// getAccessToken retrieves the current access token
func getAccessToken(ctx context.Context) (string, error) {
	// Get auth configuration
	config, err := getAuthConfig()
	if err != nil {
		// If auth is not configured, return empty token (no auth)
		return "", nil
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		// If we can't create OIDC client (e.g., OIDC provider not reachable), 
		// return empty token to allow unauthenticated access
		Logger.Debug("failed to create OIDC client, proceeding without auth", "error", err)
		return "", nil
	}

	// Create keyring storage
	storage := auth.NewKeyringStorage()

	// Create auth manager
	manager := auth.NewManager(client, storage)

	// Check if authenticated
	if !manager.IsAuthenticated(ctx) {
		// Not authenticated, return empty token
		return "", nil
	}

	// Get valid token
	token, err := manager.GetValidToken(ctx)
	if err != nil {
		// If we can't get a valid token, return empty
		Logger.Debug("failed to get valid token, proceeding without auth", "error", err)
		return "", nil
	}

	return token, nil
}

