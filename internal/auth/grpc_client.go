package auth

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ClientInterceptor provides gRPC client authentication
type ClientInterceptor struct {
	tokenProvider TokenProvider
}

// TokenProvider is a function that returns the current access token
type TokenProvider func(ctx context.Context) (string, error)

// NewClientInterceptor creates a new client interceptor
func NewClientInterceptor(tokenProvider TokenProvider) *ClientInterceptor {
	return &ClientInterceptor{
		tokenProvider: tokenProvider,
	}
}

// UnaryClientInterceptor returns a gRPC unary client interceptor
func (c *ClientInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Get token
		token, err := c.tokenProvider(ctx)
		if err != nil {
			return fmt.Errorf("failed to get authentication token: %w", err)
		}

		// Add token to metadata
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}

		// Invoke the RPC
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor returns a gRPC stream client interceptor
func (c *ClientInterceptor) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Get token
		token, err := c.tokenProvider(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get authentication token: %w", err)
		}

		// Add token to metadata
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}

		// Create the stream
		return streamer(ctx, desc, cc, method, opts...)
	}
}