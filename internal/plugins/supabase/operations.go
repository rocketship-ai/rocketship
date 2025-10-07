package supabase

import (
	"context"
	"fmt"
	"net/http"
)

// executeSupabaseOperation dispatches to the appropriate operation handler
func executeSupabaseOperation(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	switch config.Operation {
	case OpSelect:
		return executeSelect(ctx, client, config)
	case OpInsert:
		return executeInsert(ctx, client, config)
	case OpUpdate:
		return executeUpdate(ctx, client, config)
	case OpDelete:
		return executeDelete(ctx, client, config)
	case OpRPC:
		return executeRPC(ctx, client, config)
	case OpAuthCreateUser:
		return executeAuthCreateUser(ctx, client, config)
	case OpAuthDeleteUser:
		return executeAuthDeleteUser(ctx, client, config)
	case OpAuthSignUp:
		return executeAuthSignUp(ctx, client, config)
	case OpAuthSignIn:
		return executeAuthSignIn(ctx, client, config)
	case OpStorageCreateBucket:
		return executeStorageCreateBucket(ctx, client, config)
	case OpStorageDeleteBucket:
		return executeStorageDeleteBucket(ctx, client, config)
	case OpStorageUpload:
		return executeStorageUpload(ctx, client, config)
	case OpStorageDownload:
		return executeStorageDownload(ctx, client, config)
	case OpStorageDelete:
		return executeStorageDelete(ctx, client, config)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", config.Operation)
	}
}
