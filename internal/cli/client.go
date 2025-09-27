package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type EngineClient struct {
	client generated.EngineClient
	conn   *grpc.ClientConn
}

const (
	tokenEnvVar        = "ROCKETSHIP_TOKEN"
	authorizationValue = "authorization"
)

func NewEngineClient(address string) (*EngineClient, error) {
	// Decide target and transport credentials (TLS vs insecure)
	target, creds, err := resolveDialOptions(address)
	if err != nil {
		return nil, err
	}

	Logger.Debug("connecting to engine", "address", target)

	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	if token := resolveAuthToken(); token != "" {
		Logger.Debug("attaching bearer token from environment")
		dialOpts = append(dialOpts,
			grpc.WithChainUnaryInterceptor(newTokenUnaryInterceptor(token)),
			grpc.WithChainStreamInterceptor(newTokenStreamInterceptor(token)),
		)
	}

	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("connection timed out - is the engine running at %s?", target)
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
		if wrapped := translateAuthError("failed to create run", err); wrapped != nil {
			return "", wrapped
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
		if wrapped := translateAuthError("failed to create run", err); wrapped != nil {
			return "", wrapped
		}
		return "", fmt.Errorf("failed to create run: %w", err)
	}

	return resp.RunId, nil
}

func (c *EngineClient) StreamLogs(ctx context.Context, runID string) (generated.Engine_StreamLogsClient, error) {
	stream, err := c.client.StreamLogs(ctx, &generated.LogStreamRequest{
		RunId: runID,
	})
	if err != nil {
		if wrapped := translateAuthError("failed to stream logs", err); wrapped != nil {
			return nil, wrapped
		}
		return nil, fmt.Errorf("failed to stream logs: %w", err)
	}
	return stream, nil
}

func (c *EngineClient) AddLog(ctx context.Context, runID, workflowID, message, color string, bold bool) error {
	_, err := c.client.AddLog(ctx, &generated.AddLogRequest{
		RunId:      runID,
		WorkflowId: workflowID,
		Message:    message,
		Color:      color,
		Bold:       bold,
	})
	if err != nil {
		if wrapped := translateAuthError("failed to add log", err); wrapped != nil {
			return wrapped
		}
	}
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
	if err != nil {
		if wrapped := translateAuthError("failed to add log", err); wrapped != nil {
			return wrapped
		}
	}
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
		if wrapped := translateAuthError("failed to cancel run", err); wrapped != nil {
			return wrapped
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
	Version      string
	AuthEnabled  bool
	AuthType     string // "none", "cloud", "oidc", "token"
	AuthEndpoint string // OAuth/OIDC endpoint for authentication flows
	Capabilities []string
	Endpoints    map[string]string
}

// GetServerInfo gets server capabilities and configuration
func (c *EngineClient) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	infoCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := c.client.GetServerInfo(infoCtx, &generated.GetServerInfoRequest{})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unimplemented {
			return nil, fmt.Errorf("failed to get server info: server does not support discovery.v2")
		}
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	info := &ServerInfo{
		Version:      resp.GetVersion(),
		AuthEnabled:  resp.GetAuthEnabled(),
		AuthType:     resp.GetAuthType(),
		AuthEndpoint: resp.GetAuthEndpoint(),
		Capabilities: append([]string(nil), resp.GetCapabilities()...),
		Endpoints:    make(map[string]string),
	}

	for _, ep := range resp.GetEndpoints() {
		if ep == nil {
			continue
		}
		typeName := ep.GetType()
		address := ep.GetAddress()
		if typeName == "" || address == "" {
			continue
		}
		info.Endpoints[typeName] = address
	}

	return info, nil
}

func resolveAuthToken() string {
	token := strings.TrimSpace(os.Getenv(tokenEnvVar))
	return token
}

func newTokenUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	value := fmt.Sprintf("Bearer %s", token)
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, authorizationValue, value)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func newTokenStreamInterceptor(token string) grpc.StreamClientInterceptor {
	value := fmt.Sprintf("Bearer %s", token)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = metadata.AppendToOutgoingContext(ctx, authorizationValue, value)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func translateAuthError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unauthenticated:
			return fmt.Errorf("%s: engine requires a token (set %s or update your profile)", prefix, tokenEnvVar)
		case codes.PermissionDenied:
			return fmt.Errorf("%s: provided token was rejected (%s)", prefix, s.Message())
		default:
			return fmt.Errorf("%s: %s", prefix, s.Message())
		}
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// --- Dial resolution helpers ---

// resolveDialOptions determines the gRPC target and the appropriate transport credentials.
// Priority:
// 1) If an explicit address is provided, honor its scheme when present (https/grpcs = TLS; http/grpc = insecure).
//    For URL schemes without ports, defaults are 443 for https/grpcs and 7700 for http/grpc.
// 2) If no explicit address, use the active profile's engine address and TLS settings.
// 3) Surface an error when profile configuration is unavailable so the caller can prompt the user.
func resolveDialOptions(explicitAddress string) (string, credentials.TransportCredentials, error) {
	// If explicit address provided, use it
	if explicitAddress != "" {
		target, useTLS, serverName, err := parseExplicitAddress(explicitAddress)
		if err != nil {
			return "", nil, fmt.Errorf("invalid engine address: %w", err)
		}
		if useTLS {
			tlsCfg := &tls.Config{}
			if serverName != "" {
				tlsCfg.ServerName = serverName
			}
			Logger.Debug("TLS enabled for explicit engine address", "server_name", tlsCfg.ServerName)
			return target, credentials.NewTLS(tlsCfg), nil
		}
		Logger.Debug("TLS disabled for explicit engine address")
		return target, insecure.NewCredentials(), nil
	}

	// No explicit address: use profile if available
	config, err := LoadConfig()
	if err != nil {
		return "", nil, fmt.Errorf("failed to load profile config: %w", err)
	}

	profileName := config.DefaultProfile
	if profileName == "" {
		profileName = "default"
	}

	profile, exists := config.GetProfile(profileName)
	if !exists {
		return "", nil, fmt.Errorf("active profile %q not found", profileName)
	}

	if profile.EngineAddress == "" {
		return "", nil, fmt.Errorf("active profile %q has no engine address configured", profile.Name)
	}

	Logger.Debug("Using engine address from active profile", "profile", profile.Name, "address", profile.EngineAddress)
	// Parse profile.EngineAddress which may be a URL or bare host[:port]
	if profile.TLS.Enabled {
		// TLS path
		var host, port string
		if hasScheme(profile.EngineAddress) {
			u, perr := url.Parse(profile.EngineAddress)
			if perr != nil {
				return "", nil, fmt.Errorf("invalid profile engine address: %w", perr)
			}
			host = u.Hostname()
			port = u.Port()
			if port == "" {
				port = "443"
			}
		} else {
			h, p, errSplit := net.SplitHostPort(profile.EngineAddress)
			if errSplit != nil {
				// Assume missing port, default 443
				host = profile.EngineAddress
				port = "443"
			} else {
				host = h
				port = p
			}
		}
		sni := profile.TLS.Domain
		if sni == "" {
			sni = host
		}
		target := net.JoinHostPort(host, port)
		tlsCfg := &tls.Config{ServerName: sni}
		Logger.Debug("TLS enabled from profile", "address", target, "server_name", sni)
		return target, credentials.NewTLS(tlsCfg), nil
	}
	// Insecure path
	if hasScheme(profile.EngineAddress) {
		u, perr := url.Parse(profile.EngineAddress)
		if perr != nil {
			return "", nil, fmt.Errorf("invalid profile engine address: %w", perr)
		}
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "7700"
		}
		target := net.JoinHostPort(host, port)
		Logger.Debug("Using insecure transport from profile", "address", target)
		return target, insecure.NewCredentials(), nil
	}
	// Bare host[:port]
	host, port, errSplit := net.SplitHostPort(profile.EngineAddress)
	if errSplit != nil {
		host = profile.EngineAddress
		port = "7700"
	}
	target := net.JoinHostPort(host, port)
	Logger.Debug("Using insecure transport from profile", "address", target)
	return target, insecure.NewCredentials(), nil
}

// parseExplicitAddress parses an explicit address which may be a URL with a scheme
// (http, https, grpc, grpcs) or a bare host:port. Returns the dial target, whether TLS is used,
// and the server name (for SNI) when TLS is used.
func parseExplicitAddress(addr string) (target string, useTLS bool, serverName string, err error) {
	if hasScheme(addr) {
		u, perr := url.Parse(addr)
		if perr != nil {
			return "", false, "", perr
		}
		host := u.Hostname()
		port := u.Port()
		switch u.Scheme {
		case "https", "grpcs":
			if port == "" {
				port = "443"
			}
			return net.JoinHostPort(host, port), true, host, nil
		case "http", "grpc":
			if port == "" {
				port = "7700"
			}
			return net.JoinHostPort(host, port), false, "", nil
		default:
			return "", false, "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
		}
	}
	// No scheme: treat as host[:port]
	host, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		// If missing port, default to 7700
		if addr != "" && !containsColon(addr) {
			return net.JoinHostPort(addr, "7700"), false, "", nil
		}
		return "", false, "", splitErr
	}
	if host == "" {
		return "", false, "", fmt.Errorf("invalid address: missing host")
	}
	if port == "" {
		port = "7700"
	}
	return net.JoinHostPort(host, port), false, "", nil
}

func hasScheme(s string) bool {
	return len(s) >= 7 && (s[:7] == "http://" || s[:8] == "https://" || s[:7] == "grpc://" || s[:8] == "grpcs://")
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
