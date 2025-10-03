package orchestrator

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type noopTemporalClient struct{ client.Client }

type testServerStream struct {
	ctx context.Context
}

func (s *testServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *testServerStream) SendHeader(metadata.MD) error { return nil }
func (s *testServerStream) SetTrailer(metadata.MD)       {}
func (s *testServerStream) Context() context.Context     { return s.ctx }
func (s *testServerStream) SendMsg(interface{}) error    { return nil }
func (s *testServerStream) RecvMsg(interface{}) error    { return nil }

func TestAuthInterceptor_AllowsRequestsWhenDisabled(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})
	interceptor := engine.NewAuthUnaryInterceptor()

	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/rocketship.v1.Engine/CreateRun"}
	if _, err := interceptor(context.Background(), nil, info, handler); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("handler was not invoked")
	}
}

func TestAuthInterceptor_EnforcesToken(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})
	engine.ConfigureToken("secret-token")

	interceptor := engine.NewAuthUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/rocketship.v1.Engine/CreateRun"}

	t.Run("missing metadata", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
	})

	t.Run("invalid header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Token abc"))
		_, err := interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated for bad prefix, got %v", err)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer other"))
		_, err := interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected permission denied, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer secret-token"))
		resp, err := interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			principal, ok := PrincipalFromContext(ctx)
			if !ok {
				t.Fatal("expected principal in context")
			}
			if principal.Subject != "token" {
				t.Fatalf("unexpected principal subject %q", principal.Subject)
			}
			return "ok", nil
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if resp != "ok" {
			t.Fatalf("unexpected response %v", resp)
		}
	})
}

func TestAuthInterceptor_ExemptMethods(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})
	engine.ConfigureToken("secret-token")
	interceptor := engine.NewAuthUnaryInterceptor()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
	info := &grpc.UnaryServerInfo{FullMethod: "/rocketship.v1.Engine/Health"}
	if _, err := interceptor(ctx, nil, info, handler); err != nil {
		t.Fatalf("expected exempt method to succeed, got %v", err)
	}
}

func TestAuthStreamInterceptor(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})
	engine.ConfigureToken("secret-token")
	interceptor := engine.NewAuthStreamInterceptor()

	info := &grpc.StreamServerInfo{FullMethod: "/rocketship.v1.Engine/StreamLogs"}
	handlerCalled := false

	t.Run("missing token", func(t *testing.T) {
		handlerCalled = false
		err := interceptor(nil, &testServerStream{ctx: context.Background()}, info, func(_ interface{}, stream grpc.ServerStream) error {
			handlerCalled = true
			return nil
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
		if handlerCalled {
			t.Fatal("handler should not be called when auth fails")
		}
	})

	t.Run("success", func(t *testing.T) {
		handlerCalled = false
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer secret-token"))
		err := interceptor(nil, &testServerStream{ctx: ctx}, info, func(_ interface{}, stream grpc.ServerStream) error {
			handlerCalled = true
			principal, ok := PrincipalFromContext(stream.Context())
			if !ok {
				t.Fatal("expected principal on stream context")
			}
			if len(principal.Roles) == 0 {
				t.Fatal("expected principal roles to be populated")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if !handlerCalled {
			t.Fatal("handler was not invoked")
		}
	})
}

func TestConfigureServerInfo(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})
	resp, err := engine.GetServerInfo(context.Background(), &generated.GetServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo returned error: %v", err)
	}
	if resp.AuthEnabled {
		t.Fatalf("expected auth disabled by default")
	}
	if resp.AuthType != "none" {
		t.Fatalf("expected auth type none, got %s", resp.AuthType)
	}

	engine.ConfigureToken("secret-token")
	resp, err = engine.GetServerInfo(context.Background(), &generated.GetServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo returned error: %v", err)
	}
	if !resp.AuthEnabled {
		t.Fatalf("expected auth enabled after token configure")
	}
	if resp.AuthType != "token" {
		t.Fatalf("expected auth type token, got %s", resp.AuthType)
	}
	if !containsString(resp.Capabilities, "auth.token") {
		t.Fatalf("expected auth.token capability present, capabilities: %v", resp.Capabilities)
	}
}

func TestOIDCValidationRSA(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	jwksJSON := buildRSAJWKS(key)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	settings := OIDCSettings{
		Issuer:         "https://example.com",
		Audience:       "api",
		ClientID:       "rocketship-cli",
		JWKSURL:        server.URL,
		TokenEndpoint:  "https://example.com/token",
		DeviceEndpoint: "https://example.com/device",
		Scopes:         []string{"openid"},
	}

	token := signJWTRSA(key, settings.Issuer, settings.Audience)

	ctx := context.Background()
	if err := engine.ConfigureOIDC(ctx, settings); err != nil {
		t.Fatalf("ConfigureOIDC failed: %v", err)
	}

	principal, err := engine.authConfig.Validate(ctx, token)
	if err != nil {
		t.Fatalf("expected token to validate, got %v", err)
	}
	if principal == nil || principal.Subject != "user" {
		t.Fatalf("unexpected principal %+v", principal)
	}

	// Wrong audience should fail
	bad := signJWTRSA(key, settings.Issuer, "wrong")
	if _, err := engine.authConfig.Validate(ctx, bad); err == nil {
		t.Fatal("expected audience mismatch to fail")
	}
}

func TestOIDCValidationEC(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})

	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ec key: %v", err)
	}

	jwksJSON := buildECJWKS(ek)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	settings := OIDCSettings{
		Issuer:            "https://example.com",
		Audience:          "api",
		ClientID:          "rocketship-cli",
		JWKSURL:           server.URL,
		TokenEndpoint:     "https://example.com/token",
		DeviceEndpoint:    "https://example.com/device",
		Scopes:            []string{"openid"},
		AllowedAlgorithms: []string{"ES256"},
	}

	ctx := context.Background()
	if err := engine.ConfigureOIDC(ctx, settings); err != nil {
		t.Fatalf("ConfigureOIDC failed: %v", err)
	}

	token := signJWTEC(ek, settings.Issuer, settings.Audience)
	principal, err := engine.authConfig.Validate(ctx, token)
	if err != nil {
		t.Fatalf("expected token to validate, got %v", err)
	}
	if principal == nil || principal.Subject != "user" {
		t.Fatalf("unexpected principal %+v", principal)
	}

	bad := signJWTEC(ek, settings.Issuer, "wrong")
	if _, err := engine.authConfig.Validate(ctx, bad); err == nil {
		t.Fatal("expected audience mismatch to fail")
	}
}

func TestAuthorizeRequiresWriteRole(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	jwksJSON := buildRSAJWKS(key)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	settings := OIDCSettings{
		Issuer:         "https://example.com",
		Audience:       "api",
		ClientID:       "rocketship-cli",
		JWKSURL:        server.URL,
		TokenEndpoint:  "https://example.com/token",
		DeviceEndpoint: "https://example.com/device",
		Scopes:         []string{"openid"},
	}

	if err := engine.ConfigureOIDC(context.Background(), settings); err != nil {
		t.Fatalf("ConfigureOIDC failed: %v", err)
	}

	viewerToken := signJWTRSAWithRoles(key, settings.Issuer, settings.Audience, []string{"viewer"})
	viewerCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+viewerToken))
	unary := engine.NewAuthUnaryInterceptor()

	writeInfo := &grpc.UnaryServerInfo{FullMethod: "/rocketship.v1.Engine/CreateRun"}
	_, err = unary(viewerCtx, nil, writeInfo, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied for viewer, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "write access") {
		t.Fatalf("expected write access error detail, got %v", err)
	}

	readInfo := &grpc.UnaryServerInfo{FullMethod: "/rocketship.v1.Engine/ListRuns"}
	resp, err := unary(viewerCtx, nil, readInfo, func(ctx context.Context, req interface{}) (interface{}, error) {
		principal, ok := PrincipalFromContext(ctx)
		if !ok {
			t.Fatal("expected principal in read context")
		}
		if len(principal.Roles) == 0 || principal.Roles[0] != "viewer" {
			t.Fatalf("unexpected roles: %v", principal.Roles)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected viewer to list runs, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}

	serviceToken := signJWTRSAWithRoles(key, settings.Issuer, settings.Audience, []string{"service_account"})
	serviceCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+serviceToken))
	resp, err = unary(serviceCtx, nil, writeInfo, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected service account to have write access, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response for service account: %v", resp)
	}
}

func buildRSAJWKS(key *rsa.PrivateKey) string {
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	buf := make([]byte, 0)
	e := key.E
	for e > 0 {
		buf = append([]byte{byte(e % 256)}, buf...)
		e = e / 256
	}
	encodedE := base64.RawURLEncoding.EncodeToString(buf)
	return fmt.Sprintf(`{"keys":[{"kty":"RSA","kid":"test","use":"sig","alg":"RS256","n":"%s","e":"%s"}]}`, n, encodedE)
}

func buildECJWKS(key *ecdsa.PrivateKey) string {
	x := base64.RawURLEncoding.EncodeToString(key.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(key.Y.Bytes())
	return fmt.Sprintf(`{"keys":[{"kty":"EC","kid":"test","use":"sig","alg":"ES256","crv":"P-256","x":"%s","y":"%s"}]}`, x, y)
}

func signJWTRSA(key *rsa.PrivateKey, issuer, audience string) string {
	return signJWTRSAWithRoles(key, issuer, audience, []string{"owner"})
}

func signJWTRSAWithRoles(key *rsa.PrivateKey, issuer, audience string, roles []string) string {
	if len(roles) == 0 {
		roles = []string{"owner"}
	}
	claims := jwt.MapClaims{
		"iss":   issuer,
		"sub":   "user",
		"aud":   audience,
		"exp":   time.Now().Add(time.Minute).Unix(),
		"iat":   time.Now().Unix(),
		"roles": roles,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test"
	signed, err := token.SignedString(key)
	if err != nil {
		panic(err)
	}
	return signed
}

func signJWTEC(key *ecdsa.PrivateKey, issuer, audience string) string {
	claims := jwt.MapClaims{
		"iss":   issuer,
		"sub":   "user",
		"aud":   audience,
		"exp":   time.Now().Add(time.Minute).Unix(),
		"iat":   time.Now().Unix(),
		"roles": []string{"owner"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "test"
	signed, err := token.SignedString(key)
	if err != nil {
		panic(err)
	}
	return signed
}
