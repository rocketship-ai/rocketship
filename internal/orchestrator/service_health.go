package orchestrator

import (
	"context"
	"log/slog"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/embedded"
)

// Health returns the health status of the engine
func (e *Engine) Health(ctx context.Context, _ *generated.HealthRequest) (*generated.HealthResponse, error) {
	return &generated.HealthResponse{Status: "ok"}, nil
}

// GetServerInfo returns server version and configuration information
func (e *Engine) GetServerInfo(ctx context.Context, _ *generated.GetServerInfoRequest) (*generated.GetServerInfoResponse, error) {
	version := embedded.DefaultVersion
	if version == "" {
		version = "dev"
	}

	resp := &generated.GetServerInfoResponse{
		Version:      version,
		AuthEnabled:  false,
		AuthType:     "none",
		AuthEndpoint: "",
		Capabilities: []string{"discovery.v2"},
	}

	e.authConfig.configureServerInfo(resp)
	return resp, nil
}

// WaitForCleanup waits for all suite cleanup workflows to complete
// This should be called before server shutdown to ensure cleanup completes
func (e *Engine) WaitForCleanup(ctx context.Context, req *generated.WaitForCleanupRequest) (*generated.WaitForCleanupResponse, error) {
	slog.Info("WaitForCleanup: Waiting for cleanup workflows to complete")

	// Wait for all cleanup workflows with a timeout
	done := make(chan struct{})
	go func() {
		slog.Debug("WaitForCleanup: Calling cleanupWg.Wait()")
		e.cleanupWg.Wait()
		slog.Debug("WaitForCleanup: cleanupWg.Wait() returned - all cleanups done")
		close(done)
	}()

	select {
	case <-done:
		slog.Info("WaitForCleanup: All cleanup workflows completed successfully")
		return &generated.WaitForCleanupResponse{Completed: true}, nil
	case <-ctx.Done():
		slog.Warn("WaitForCleanup: Context cancelled/timed out before cleanup completed", "error", ctx.Err())
		return &generated.WaitForCleanupResponse{Completed: false}, ctx.Err()
	}
}
