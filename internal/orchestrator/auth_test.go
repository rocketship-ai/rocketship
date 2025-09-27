package orchestrator

import (
	"context"
	"testing"

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
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	t.Run("missing metadata", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, info, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
	})

	t.Run("invalid header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Token abc"))
		_, err := interceptor(ctx, nil, info, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated for bad prefix, got %v", err)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer other"))
		_, err := interceptor(ctx, nil, info, handler)
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected permission denied, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer secret-token"))
		resp, err := interceptor(ctx, nil, info, handler)
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
	handler := func(_ interface{}, _ grpc.ServerStream) error {
		handlerCalled = true
		return nil
	}

	t.Run("missing token", func(t *testing.T) {
		handlerCalled = false
		err := interceptor(nil, &testServerStream{ctx: context.Background()}, info, handler)
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
		if err := interceptor(nil, &testServerStream{ctx: ctx}, info, handler); err != nil {
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

func TestMustConfigureTokenPanicsOnWhitespace(t *testing.T) {
	engine := NewEngine(&noopTemporalClient{})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for whitespace token")
		}
	}()
	engine.MustConfigureToken("\n\t ")
}
