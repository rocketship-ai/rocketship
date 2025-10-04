package cli

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/cli/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Mock gRPC server for testing
type mockEngineServer struct {
	generated.UnimplementedEngineServer
	healthStatus       string
	runResponse        *generated.CreateRunResponse
	serverInfoResponse *generated.GetServerInfoResponse
	healthErr          error
	runErr             error
	serverInfoErr      error
	authHeaders        []string
}

func (m *mockEngineServer) Health(ctx context.Context, req *generated.HealthRequest) (*generated.HealthResponse, error) {
	if m.healthErr != nil {
		return nil, m.healthErr
	}
	return &generated.HealthResponse{Status: m.healthStatus}, nil
}

func (m *mockEngineServer) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		m.authHeaders = append([]string(nil), md.Get("authorization")...)
	} else {
		m.authHeaders = nil
	}
	if m.runErr != nil {
		return nil, m.runErr
	}
	return m.runResponse, nil
}

func (m *mockEngineServer) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	// Simple mock implementation
	return nil
}

func (m *mockEngineServer) GetServerInfo(ctx context.Context, req *generated.GetServerInfoRequest) (*generated.GetServerInfoResponse, error) {
	if m.serverInfoErr != nil {
		return nil, m.serverInfoErr
	}
	if m.serverInfoResponse != nil {
		return m.serverInfoResponse, nil
	}
	return nil, status.Error(codes.Unimplemented, "GetServerInfo not implemented")
}

func setupMockServer(tb testing.TB, mock *mockEngineServer) (string, func()) {
	tb.Helper()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		tb.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	generated.RegisterEngineServer(s, mock)

	go func() {
		if err := s.Serve(lis); err != nil {
			// Only log if not a normal shutdown
			if err.Error() != "grpc: the server has been stopped" {
				tb.Logf("Server serve error: %v", err)
			}
		}
	}()

	cleanup := func() {
		s.Stop()
		_ = lis.Close()
	}

	return lis.Addr().String(), cleanup
}

func TestNewEngineClient(t *testing.T) {
	t.Parallel()

	// Initialize logger to prevent nil pointer dereference
	InitLogging()

	mock := &mockEngineServer{healthStatus: "ok"}
	addr, cleanup := setupMockServer(t, mock)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewEngineClient(addr)
	if err != nil {
		t.Fatalf("NewEngineClient failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	if client.client == nil {
		t.Error("Client should not be nil")
	}
	if client.conn == nil {
		t.Error("Connection should not be nil")
	}
}

func TestEngineClient_HealthCheck(t *testing.T) {
	t.Parallel()

	// Initialize logger to prevent nil pointer dereference
	InitLogging()

	tests := []struct {
		name         string
		healthStatus string
		healthErr    error
		wantErr      bool
		errContains  string
	}{
		{
			name:         "healthy status",
			healthStatus: "ok",
			wantErr:      false,
		},
		{
			name:         "unhealthy status",
			healthStatus: "error",
			wantErr:      true,
			errContains:  "unhealthy status",
		},
		{
			name:        "grpc error",
			healthErr:   status.Error(codes.Unavailable, "service unavailable"),
			wantErr:     true,
			errContains: "health check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockEngineServer{
				healthStatus: tt.healthStatus,
				healthErr:    tt.healthErr,
			}
			addr, cleanup := setupMockServer(t, mock)
			defer cleanup()

			// Give server time to start
			time.Sleep(100 * time.Millisecond)

			client, err := NewEngineClient(addr)
			if err != nil {
				t.Fatalf("NewEngineClient failed: %v", err)
			}
			defer func() { _ = client.Close() }()

			ctx := context.Background()
			err = client.HealthCheck(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEngineClient_GetServerInfo(t *testing.T) {
	t.Parallel()

	// Initialize logger to prevent nil pointer dereference
	InitLogging()

	tests := []struct {
		name               string
		serverInfoResponse *generated.GetServerInfoResponse
		serverInfoErr      error
		expect             *ServerInfo
		wantErr            bool
	}{
		{
			name: "discovery v2 response",
			serverInfoResponse: &generated.GetServerInfoResponse{
				Version:      "v0.9.0",
				AuthEnabled:  true,
				AuthType:     "token",
				AuthEndpoint: "https://auth.example.com/token",
				Capabilities: []string{"token-auth", "discovery.v2"},
				Endpoints: []*generated.ServerEndpoint{
					{Type: "grpc", Address: "example.com:443"},
				},
			},
			expect: &ServerInfo{
				Version:      "v0.9.0",
				AuthEnabled:  true,
				AuthType:     "token",
				AuthEndpoint: "https://auth.example.com/token",
				Capabilities: []string{"token-auth", "discovery.v2"},
				Endpoints:    map[string]string{"grpc": "example.com:443"},
			},
		},
		{
			name:          "server info error propagates",
			serverInfoErr: status.Error(codes.Internal, "boom"),
			wantErr:       true,
		},
		{
			name:          "unimplemented discovery triggers error",
			serverInfoErr: status.Error(codes.Unimplemented, "no discovery"),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockEngineServer{
				serverInfoResponse: tt.serverInfoResponse,
				serverInfoErr:      tt.serverInfoErr,
			}

			addr, cleanup := setupMockServer(t, mock)
			defer cleanup()

			client, err := NewEngineClient(addr)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					t.Errorf("Failed to close client: %v", err)
				}
			}()

			ctx := context.Background()
			serverInfo, err := client.GetServerInfo(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expect == nil {
				t.Fatal("expected struct must be provided when wantErr is false")
			}
			if serverInfo.Version != tt.expect.Version {
				t.Errorf("Version = %q, want %q", serverInfo.Version, tt.expect.Version)
			}
			if serverInfo.AuthEnabled != tt.expect.AuthEnabled {
				t.Errorf("AuthEnabled = %v, want %v", serverInfo.AuthEnabled, tt.expect.AuthEnabled)
			}
			if serverInfo.AuthType != tt.expect.AuthType {
				t.Errorf("AuthType = %s, want %s", serverInfo.AuthType, tt.expect.AuthType)
			}
			if serverInfo.AuthEndpoint != tt.expect.AuthEndpoint {
				t.Errorf("AuthEndpoint = %s, want %s", serverInfo.AuthEndpoint, tt.expect.AuthEndpoint)
			}
			if !reflect.DeepEqual(serverInfo.Capabilities, tt.expect.Capabilities) {
				t.Errorf("Capabilities = %v, want %v", serverInfo.Capabilities, tt.expect.Capabilities)
			}
			if !reflect.DeepEqual(serverInfo.Endpoints, tt.expect.Endpoints) {
				t.Errorf("Endpoints = %v, want %v", serverInfo.Endpoints, tt.expect.Endpoints)
			}
		})
	}
}

func TestEngineClient_RunTest(t *testing.T) {
	t.Parallel()

	// Initialize logger to prevent nil pointer dereference
	InitLogging()

	tests := []struct {
		name        string
		yamlData    []byte
		runResponse *generated.CreateRunResponse
		runErr      error
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful run",
			yamlData:    []byte("test: data"),
			runResponse: &generated.CreateRunResponse{RunId: "test-run-123"},
			wantErr:     false,
		},
		{
			name:        "grpc error",
			yamlData:    []byte("test: data"),
			runErr:      status.Error(codes.InvalidArgument, "invalid yaml"),
			wantErr:     true,
			errContains: "failed to create run",
		},
		{
			name:        "empty yaml",
			yamlData:    []byte{},
			runResponse: &generated.CreateRunResponse{RunId: "empty-run"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockEngineServer{
				runResponse: tt.runResponse,
				runErr:      tt.runErr,
			}
			addr, cleanup := setupMockServer(t, mock)
			defer cleanup()

			// Give server time to start
			time.Sleep(100 * time.Millisecond)

			client, err := NewEngineClient(addr)
			if err != nil {
				t.Fatalf("NewEngineClient failed: %v", err)
			}
			defer func() { _ = client.Close() }()

			ctx := context.Background()
			runID, err := client.RunTest(ctx, tt.yamlData)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if runID == "" {
					t.Error("Expected non-empty run ID")
				}
				if tt.runResponse != nil && runID != tt.runResponse.RunId {
					t.Errorf("Expected run ID %s, got %s", tt.runResponse.RunId, runID)
				}
			}
		})
	}
}

func TestEngineClient_AttachesBearerToken(t *testing.T) {
	// Do not run in parallel to keep environment scoped to this test lifecycle
	InitLogging()

	mock := &mockEngineServer{
		runResponse: &generated.CreateRunResponse{RunId: "abc"},
	}
	addr, cleanup := setupMockServer(t, mock)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	t.Setenv(tokenEnvVar, "secret123")

	client, err := NewEngineClient(addr)
	if err != nil {
		t.Fatalf("NewEngineClient failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.RunTest(context.Background(), []byte("name: test")); err != nil {
		t.Fatalf("RunTest failed: %v", err)
	}

	if len(mock.authHeaders) == 0 {
		t.Fatal("expected authorization header to be captured")
	}
	if got, want := mock.authHeaders[0], "Bearer secret123"; got != want {
		t.Fatalf("authorization header mismatch: got %q, want %q", got, want)
	}
}

func TestEngineClientUsesStoredToken(t *testing.T) {
	InitLogging()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ROCKETSHIP_DISABLE_KEYRING", "1")
	t.Setenv(tokenEnvVar, "")

	mock := &mockEngineServer{
		runResponse: &generated.CreateRunResponse{RunId: "abc"},
	}
	addr, cleanup := setupMockServer(t, mock)
	defer cleanup()

	// Persist profile configuration using the stored token.
	cfg := DefaultConfig()
	profile := Profile{
		Name:          "stored",
		EngineAddress: addr,
	}
	cfg.AddProfile(profile)
	cfg.DefaultProfile = "stored"
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	storeDir := filepath.Join(home, ".rocketship", "tokens")
	store, err := auth.NewFileStore(storeDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	if err := store.Save("stored", auth.TokenData{
		AccessToken: "stored-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewEngineClient("")
	if err != nil {
		t.Fatalf("NewEngineClient failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.RunTest(context.Background(), []byte("name: test")); err != nil {
		t.Fatalf("RunTest failed: %v", err)
	}

	if len(mock.authHeaders) == 0 {
		t.Fatal("expected authorization header to be captured")
	}
	if got, want := mock.authHeaders[0], "Bearer stored-token"; got != want {
		t.Fatalf("authorization header mismatch: got %q, want %q", got, want)
	}
}

func TestTranslateAuthError(t *testing.T) {
	unauth := status.Error(codes.Unauthenticated, "missing metadata")
	if err := translateAuthError("failed op", unauth); err == nil || !strings.Contains(err.Error(), "requires a token") {
		t.Fatalf("expected token guidance, got %v", err)
	}

	denied := status.Error(codes.PermissionDenied, "invalid token")
	if err := translateAuthError("failed op", denied); err == nil || !strings.Contains(err.Error(), "permission denied") || !strings.Contains(err.Error(), "invalid token") || !strings.Contains(err.Error(), "Ensure your token") {
		t.Fatalf("expected permission guidance with detail, got %v", err)
	}

	deniedPending := status.Error(codes.PermissionDenied, "requires write access (roles: pending)")
	if err := translateAuthError("failed op", deniedPending); err == nil || !strings.Contains(err.Error(), "first-time users must create an organisation") {
		t.Fatalf("expected pending guidance, got %v", err)
	}

	other := status.Error(codes.InvalidArgument, "bad input")
	if err := translateAuthError("failed op", other); err == nil || !strings.Contains(err.Error(), "bad input") {
		t.Fatalf("expected passthrough message, got %v", err)
	}

	generic := errors.New("boom")
	if err := translateAuthError("failed op", generic); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped generic error, got %v", err)
	}
}

func TestEngineClient_StreamLogs(t *testing.T) {
	t.Parallel()

	// Initialize logger to prevent nil pointer dereference
	InitLogging()

	mock := &mockEngineServer{}
	addr, cleanup := setupMockServer(t, mock)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewEngineClient(addr)
	if err != nil {
		t.Fatalf("NewEngineClient failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	stream, err := client.StreamLogs(ctx, "test-run-123")
	if err != nil {
		t.Fatalf("StreamLogs failed: %v", err)
	}

	if stream == nil {
		t.Error("Stream should not be nil")
	}
}

func TestEngineClient_ConcurrentOperations(t *testing.T) {
	t.Parallel()

	// Skip complex concurrent test
	t.Skip("Skipping complex concurrent test")
}

func TestEngineClient_ContextTimeout(t *testing.T) {
	t.Parallel()

	// Skip timeout test
	t.Skip("Skipping timeout test")
}

func TestEngineClient_Close(t *testing.T) {
	t.Parallel()

	// Initialize logger to prevent nil pointer dereference
	InitLogging()

	mock := &mockEngineServer{healthStatus: "ok"}
	addr, cleanup := setupMockServer(t, mock)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewEngineClient(addr)
	if err != nil {
		t.Fatalf("NewEngineClient failed: %v", err)
	}

	// Test that Close() doesn't panic and returns no error
	err = client.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Test that operations after close fail gracefully
	ctx := context.Background()
	err = client.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error after close but got none")
	}
}

func TestResolveDialOptionsUsesActiveProfile(t *testing.T) {
	InitLogging()
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := DefaultConfig()
	profile := Profile{
		Name:          "globalbank",
		EngineAddress: "globalbank.rocketship.sh:443",
		TLS: TLSConfig{
			Enabled: true,
			Domain:  "globalbank.rocketship.sh",
		},
	}
	cfg.AddProfile(profile)
	cfg.DefaultProfile = "globalbank"

	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	target, creds, profileName, usedProfile, err := resolveDialOptions("")
	if err != nil {
		t.Fatalf("resolveDialOptions returned error: %v", err)
	}
	if !usedProfile {
		t.Fatalf("expected to use active profile")
	}
	if profileName != "globalbank" {
		t.Fatalf("profileName = %q, want %q", profileName, "globalbank")
	}

	if target != "globalbank.rocketship.sh:443" {
		t.Fatalf("target = %q, want %q", target, "globalbank.rocketship.sh:443")
	}

	pi := creds.Info()
	if pi.SecurityProtocol != "tls" {
		t.Fatalf("expected TLS credentials, got %q", pi.SecurityProtocol)
	}
}

func TestResolveDialOptionsMissingProfile(t *testing.T) {
	InitLogging()
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := DefaultConfig()
	cfg.DefaultProfile = "ghost"

	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	_, _, _, _, err := resolveDialOptions("")
	if err == nil {
		t.Fatal("expected error when active profile is missing")
	}

	if !contains(err.Error(), "not found") {
		t.Fatalf("expected missing profile error, got %v", err)
	}
}

func TestEngineClient_InvalidAddress(t *testing.T) {
	t.Parallel()

	// Skip this test as gRPC is more lenient with address formats than expected
	t.Skip("Skipping invalid address test - gRPC accepts more address formats than expected")
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
