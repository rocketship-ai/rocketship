package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// executeStorageCreateBucket handles storage bucket creation
func executeStorageCreateBucket(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Storage == nil || config.Storage.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required for storage create bucket operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/storage/v1/bucket", strings.TrimSuffix(config.URL, "/"))

	// Build request body
	reqBody := map[string]interface{}{
		"name":   config.Storage.Bucket,
		"public": config.Storage.Public,
	}

	// Serialize data
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal storage data: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}

// executeStorageDeleteBucket handles storage bucket deletion
func executeStorageDeleteBucket(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Storage == nil || config.Storage.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required for storage delete bucket operation")
	}

	// Build URL - bucket deletion uses the bucket ID/name in the path
	endpoint := fmt.Sprintf("%s/storage/v1/bucket/%s", strings.TrimSuffix(config.URL, "/"), config.Storage.Bucket)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}

// executeStorageUpload handles file upload to storage
func executeStorageUpload(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Storage == nil || config.Storage.Bucket == "" || config.Storage.Path == "" {
		return nil, fmt.Errorf("bucket and path are required for storage upload operation")
	}

	if config.Storage.FileContent == "" {
		return nil, fmt.Errorf("file_content is required for storage upload operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s",
		strings.TrimSuffix(config.URL, "/"),
		config.Storage.Bucket,
		config.Storage.Path)

	// Create request with file content
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(config.Storage.FileContent))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)

	// Set content type if provided
	if config.Storage.ContentType != "" {
		req.Header.Set("Content-Type", config.Storage.ContentType)
	} else {
		req.Header.Set("Content-Type", "text/plain")
	}

	// Set cache control if provided
	if config.Storage.CacheControl != "" {
		req.Header.Set("Cache-Control", config.Storage.CacheControl)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}

// executeStorageDownload handles file download from storage
func executeStorageDownload(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Storage == nil || config.Storage.Bucket == "" || config.Storage.Path == "" {
		return nil, fmt.Errorf("bucket and path are required for storage download operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s",
		strings.TrimSuffix(config.URL, "/"),
		config.Storage.Bucket,
		config.Storage.Path)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// For downloads, we want to read the file content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Create response object
	response := &SupabaseResponse{
		Metadata: &ResponseMetadata{
			StatusCode: resp.StatusCode,
			Headers:    make(map[string]string),
		},
	}

	// Copy relevant headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Metadata.Headers[key] = values[0]
		}
	}

	if resp.StatusCode >= 400 {
		var supabaseErr SupabaseError
		if err := json.Unmarshal(body, &supabaseErr); err == nil {
			response.Error = &supabaseErr
		} else {
			response.Error = &SupabaseError{
				Message: string(body),
			}
		}
		return response, nil
	}

	// For successful download, return the file content as data
	response.Data = string(body)
	return response, nil
}

// executeStorageDelete handles file deletion from storage
func executeStorageDelete(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Storage == nil || config.Storage.Bucket == "" || config.Storage.Path == "" {
		return nil, fmt.Errorf("bucket and path are required for storage delete operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s",
		strings.TrimSuffix(config.URL, "/"),
		config.Storage.Bucket,
		config.Storage.Path)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}
