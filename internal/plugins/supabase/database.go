package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go.temporal.io/sdk/activity"
)

// executeSelect handles SELECT operations
func executeSelect(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Table == "" {
		return nil, fmt.Errorf("table is required for select operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/rest/v1/%s", strings.TrimSuffix(config.URL, "/"), config.Table)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Build query parameters
	query := u.Query()

	// Handle columns selection
	if config.Select != nil {
		if len(config.Select.Columns) > 0 {
			query.Set("select", strings.Join(config.Select.Columns, ","))
		}

		// Handle filters
		for _, filter := range config.Select.Filters {
			filterParam := buildFilterParam(filter)
			if filterParam != "" {
				query.Set(filter.Column, filterParam)
			}
		}

		// Handle ordering
		if len(config.Select.Order) > 0 {
			orderParts := make([]string, len(config.Select.Order))
			for i, order := range config.Select.Order {
				if order.Ascending {
					orderParts[i] = order.Column
				} else {
					orderParts[i] = order.Column + ".desc"
				}
			}
			query.Set("order", strings.Join(orderParts, ","))
		}

		// Handle limit and offset
		if config.Select.Limit != nil {
			query.Set("limit", strconv.Itoa(*config.Select.Limit))
		}
		if config.Select.Offset != nil {
			query.Set("offset", strconv.Itoa(*config.Select.Offset))
		}

		// Count is handled in the Prefer header, not as a query parameter
	}

	u.RawQuery = query.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")
	if config.Select != nil && config.Select.Count != "" {
		req.Header.Set("Prefer", "count="+config.Select.Count)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
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

	// Handle count header
	if countHeader := resp.Header.Get("Content-Range"); countHeader != "" {
		if count := parseContentRange(countHeader); count >= 0 {
			response.Count = &count
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

	// Parse successful response
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	response.Data = data
	return response, nil
}

// executeInsert handles INSERT operations
func executeInsert(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Table == "" {
		return nil, fmt.Errorf("table is required for insert operation")
	}
	if config.Insert == nil || config.Insert.Data == nil {
		return nil, fmt.Errorf("insert data is required")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/rest/v1/%s", strings.TrimSuffix(config.URL, "/"), config.Table)

	// Serialize data
	jsonData, err := json.Marshal(config.Insert.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal insert data: %w", err)
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
	req.Header.Set("Prefer", "return=representation")

	// Handle upsert
	if config.Insert.Upsert {
		prefer := "resolution=merge-duplicates"
		if config.Insert.OnConflict != "" {
			prefer = fmt.Sprintf("resolution=merge-duplicates,on_conflict=%s", config.Insert.OnConflict)
		}
		req.Header.Set("Prefer", prefer)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}

// executeUpdate handles UPDATE operations
func executeUpdate(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Table == "" {
		return nil, fmt.Errorf("table is required for update operation")
	}
	if config.Update == nil || config.Update.Data == nil {
		return nil, fmt.Errorf("update data is required")
	}

	// Build URL with filters
	endpoint := fmt.Sprintf("%s/rest/v1/%s", strings.TrimSuffix(config.URL, "/"), config.Table)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Apply filters
	if len(config.Update.Filters) > 0 {
		query := u.Query()
		for _, filter := range config.Update.Filters {
			filterParam := buildFilterParam(filter)
			if filterParam != "" {
				query.Set(filter.Column, filterParam)
			}
		}
		u.RawQuery = query.Encode()
	}

	// Serialize data
	jsonData, err := json.Marshal(config.Update.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update data: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "PATCH", u.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}

// executeDelete handles DELETE operations
func executeDelete(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Table == "" {
		return nil, fmt.Errorf("table is required for delete operation")
	}
	if config.Delete == nil || len(config.Delete.Filters) == 0 {
		return nil, fmt.Errorf("filters are required for delete operation (safety measure)")
	}

	// Build URL with filters
	endpoint := fmt.Sprintf("%s/rest/v1/%s", strings.TrimSuffix(config.URL, "/"), config.Table)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Apply filters
	query := u.Query()
	for _, filter := range config.Delete.Filters {
		filterParam := buildFilterParam(filter)
		if filterParam != "" {
			query.Set(filter.Column, filterParam)
		}
	}
	u.RawQuery = query.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseSupabaseResponse(resp)
}

// executeRPC handles RPC function calls
func executeRPC(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.RPC == nil || config.RPC.Function == "" {
		return nil, fmt.Errorf("function name is required for RPC operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/rest/v1/rpc/%s", strings.TrimSuffix(config.URL, "/"), config.RPC.Function)

	// Serialize parameters
	var jsonData []byte
	var err error
	if config.RPC.Params != nil {
		jsonData, err = json.Marshal(config.RPC.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal RPC params: %w", err)
		}
	} else {
		jsonData = []byte("{}")
	}

	// Log the RPC request details for debugging
	logger := activity.GetLogger(ctx)
	logger.Info("Executing RPC request",
		"endpoint", endpoint,
		"function", config.RPC.Function,
		"params", string(jsonData))

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
