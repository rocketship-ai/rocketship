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
	"time"

	"github.com/itchyny/gojq"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"go.temporal.io/sdk/activity"
)

// Auto-register the plugin when the package is imported
func init() {
	plugins.RegisterPlugin(&SupabasePlugin{})
}

// GetType returns the plugin type identifier
func (sp *SupabasePlugin) GetType() string {
	return "supabase"
}

// Activity executes Supabase operations and returns results
func (sp *SupabasePlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	logger := activity.GetLogger(ctx)

	// Debug: Log all parameter keys
	paramKeys := make([]string, 0, len(p))
	for k := range p {
		paramKeys = append(paramKeys, k)
	}
	logger.Info("SUPABASE Activity called", "paramKeys", paramKeys)
	if saveData, ok := p["save"]; ok {
		saveDataJSON, _ := json.Marshal(saveData)
		logger.Info("SUPABASE save parameter exists", "save", string(saveDataJSON))
	} else {
		logger.Info("SUPABASE save parameter NOT FOUND")
	}

	// Parse configuration from parameters
	configData, ok := p["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	// Get state for variable replacement
	state := make(map[string]string)
	if stateInterface, ok := p["state"]; ok {
		if stateMap, ok := stateInterface.(map[string]interface{}); ok {
			for k, v := range stateMap {
				state[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	config := &SupabaseConfig{}
	if err := parseConfig(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse Supabase config: %w", err)
	}
	
	// Process runtime variables in string fields
	config.URL = replaceVariables(config.URL, state)
	config.Key = replaceVariables(config.Key, state)
	if config.Table != "" {
		config.Table = replaceVariables(config.Table, state)
	}
	
	// Process runtime variables in RPC parameters
	if config.RPC != nil && config.RPC.Params != nil {
		config.RPC.Params = processVariablesInMap(config.RPC.Params, state)
	}
	
	// Process runtime variables in other operation types
	if config.Insert != nil {
		if config.Insert.Data != nil {
			if dataMap, ok := config.Insert.Data.(map[string]interface{}); ok {
				config.Insert.Data = processVariablesInMap(dataMap, state)
			}
		}
	}
	
	if config.Update != nil {
		if config.Update.Data != nil {
			config.Update.Data = processVariablesInMap(config.Update.Data, state)
		}
		config.Update.Filters = processFilters(config.Update.Filters, state)
	}
	
	if config.Select != nil {
		config.Select.Filters = processFilters(config.Select.Filters, state)
	}
	
	if config.Delete != nil {
		config.Delete.Filters = processFilters(config.Delete.Filters, state)
	}

	// Process runtime variables in Auth config
	if config.Auth != nil {
		config.Auth.Email = replaceVariables(config.Auth.Email, state)
		config.Auth.Password = replaceVariables(config.Auth.Password, state)
		config.Auth.UserID = replaceVariables(config.Auth.UserID, state)
		if config.Auth.UserMetadata != nil {
			config.Auth.UserMetadata = processVariablesInMap(config.Auth.UserMetadata, state)
		}
		if config.Auth.AppMetadata != nil {
			config.Auth.AppMetadata = processVariablesInMap(config.Auth.AppMetadata, state)
		}
	}

	// Log parsed config for debugging
	logger.Info("Parsed Supabase config",
		"operation", config.Operation,
		"hasRPC", config.RPC != nil,
		"rpcFunction", func() string {
			if config.RPC != nil {
				return config.RPC.Function
			}
			return "nil"
		}(),
		"rpcParams", func() string {
			if config.RPC != nil && config.RPC.Params != nil {
				paramsJSON, _ := json.Marshal(config.RPC.Params)
				return string(paramsJSON)
			}
			return "nil"
		}())

	// Validate required fields
	if config.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if config.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if config.Operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	logger.Info("Executing Supabase operation", "operation", config.Operation, "table", config.Table)

	// Set default timeout
	timeout := 30 * time.Second
	if config.Timeout != "" {
		if parsedTimeout, err := time.ParseDuration(config.Timeout); err == nil {
			timeout = parsedTimeout
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{Timeout: timeout}
	
	startTime := time.Now()
	response, err := executeSupabaseOperation(ctx, client, config)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("Supabase operation failed", "error", err, "duration", duration)
		return nil, err
	}

	// Check if the response contains an API error (HTTP 4xx/5xx)
	if response.Error != nil {
		statusCode := 0
		if response.Metadata != nil {
			statusCode = response.Metadata.StatusCode
		}
		logger.Error("Supabase API returned error",
			"error_message", response.Error.Message,
			"error_code", response.Error.Code,
			"status_code", statusCode,
			"duration", duration)
		return nil, fmt.Errorf("supabase api error (status %d): %s", statusCode, response.Error.Message)
	}

	// Add metadata
	if response.Metadata == nil {
		response.Metadata = &ResponseMetadata{}
	}
	response.Metadata.Operation = config.Operation
	response.Metadata.Table = config.Table
	response.Metadata.Duration = duration.String()

	logger.Info("Supabase operation completed", "operation", config.Operation, "duration", duration)

	// Process assertions
	if assertions, ok := p["assertions"].([]interface{}); ok {
		if err := processAssertions(response, assertions, p); err != nil {
			logger.Error("Assertion failed", "error", err)
			return nil, fmt.Errorf("assertion failed: %w", err)
		}
	}

	// Handle save operations
	saved := make(map[string]string)
	if saveConfigs, ok := p["save"].([]interface{}); ok {
		logger.Info("Processing save configs", "count", len(saveConfigs))
		for _, saveConfigInterface := range saveConfigs {
			if saveConfig, ok := saveConfigInterface.(map[string]interface{}); ok {
				logger.Info("Processing save config", "config", saveConfig)
				if err := processSave(ctx, response, saveConfig, saved); err != nil {
					logger.Error("Failed to save value", "error", err, "config", saveConfig)
				} else {
					logger.Info("Successfully saved value", "saved", saved)
				}
			}
		}
	}
	logger.Info("Final saved values", "saved", saved)

	return &ActivityResponse{
		Response: response,
		Saved:    saved,
	}, nil
}

// executeSupabaseOperation performs the actual Supabase operation
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

// executeAuthCreateUser handles auth user creation (admin)
func executeAuthCreateUser(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Auth == nil || config.Auth.Email == "" {
		return nil, fmt.Errorf("email is required for auth create user operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/auth/v1/admin/users", strings.TrimSuffix(config.URL, "/"))

	// Build request body
	reqBody := map[string]interface{}{
		"email": config.Auth.Email,
	}
	if config.Auth.Password != "" {
		reqBody["password"] = config.Auth.Password
	}
	if config.Auth.EmailConfirm {
		reqBody["email_confirm"] = true
	}
	if config.Auth.UserMetadata != nil {
		reqBody["user_metadata"] = config.Auth.UserMetadata
	}
	if config.Auth.AppMetadata != nil {
		reqBody["app_metadata"] = config.Auth.AppMetadata
	}

	// Serialize data
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
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

	// Parse the raw response
	response, err := parseSupabaseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Transform response to match expected structure
	// Raw API returns user object directly: { id, email, ... }
	// Tests expect: { user: { id, email, ... } }
	if response.Data != nil && response.Error == nil {
		// Wrap the user data in a user object for consistency
		response.Data = map[string]interface{}{
			"user": response.Data,
		}
	}

	return response, nil
}

// executeAuthDeleteUser handles auth user deletion (admin)
func executeAuthDeleteUser(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Auth == nil || config.Auth.UserID == "" {
		return nil, fmt.Errorf("user_id is required for auth delete user operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/auth/v1/admin/users/%s", strings.TrimSuffix(config.URL, "/"), config.Auth.UserID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", endpoint, nil)
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

// executeAuthSignUp handles user sign up
func executeAuthSignUp(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Auth == nil || config.Auth.Email == "" || config.Auth.Password == "" {
		return nil, fmt.Errorf("email and password are required for auth sign up operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/auth/v1/signup", strings.TrimSuffix(config.URL, "/"))

	// Build request body
	reqBody := map[string]interface{}{
		"email":    config.Auth.Email,
		"password": config.Auth.Password,
	}
	if config.Auth.UserMetadata != nil {
		reqBody["data"] = config.Auth.UserMetadata
	}

	// Serialize data
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the raw response
	response, err := parseSupabaseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Transform response to match Supabase SDK structure
	// Raw API returns: { access_token, refresh_token, user, ... }
	// SDK expects: { user, session: { access_token, refresh_token, ... } }
	if response.Data != nil && response.Error == nil {
		if authData, ok := response.Data.(map[string]interface{}); ok {
			// Extract user and session data
			user := authData["user"]

			// Build session object with all token-related fields
			session := make(map[string]interface{})
			if accessToken, ok := authData["access_token"]; ok {
				session["access_token"] = accessToken
			}
			if refreshToken, ok := authData["refresh_token"]; ok {
				session["refresh_token"] = refreshToken
			}
			if tokenType, ok := authData["token_type"]; ok {
				session["token_type"] = tokenType
			}
			if expiresIn, ok := authData["expires_in"]; ok {
				session["expires_in"] = expiresIn
			}
			if expiresAt, ok := authData["expires_at"]; ok {
				session["expires_at"] = expiresAt
			}

			// Create SDK-compatible structure
			response.Data = map[string]interface{}{
				"user":    user,
				"session": session,
			}
		}
	}

	return response, nil
}

// executeAuthSignIn handles user sign in
func executeAuthSignIn(ctx context.Context, client *http.Client, config *SupabaseConfig) (*SupabaseResponse, error) {
	if config.Auth == nil || config.Auth.Email == "" || config.Auth.Password == "" {
		return nil, fmt.Errorf("email and password are required for auth sign in operation")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/auth/v1/token?grant_type=password", strings.TrimSuffix(config.URL, "/"))

	// Build request body
	reqBody := map[string]interface{}{
		"email":    config.Auth.Email,
		"password": config.Auth.Password,
	}

	// Serialize data
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("apikey", config.Key)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the raw response
	response, err := parseSupabaseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Transform response to match Supabase SDK structure
	// Raw API returns: { access_token, refresh_token, user, ... }
	// SDK expects: { user, session: { access_token, refresh_token, ... } }
	if response.Data != nil && response.Error == nil {
		if authData, ok := response.Data.(map[string]interface{}); ok {
			// Extract user and session data
			user := authData["user"]

			// Build session object with all token-related fields
			session := make(map[string]interface{})
			if accessToken, ok := authData["access_token"]; ok {
				session["access_token"] = accessToken
			}
			if refreshToken, ok := authData["refresh_token"]; ok {
				session["refresh_token"] = refreshToken
			}
			if tokenType, ok := authData["token_type"]; ok {
				session["token_type"] = tokenType
			}
			if expiresIn, ok := authData["expires_in"]; ok {
				session["expires_in"] = expiresIn
			}
			if expiresAt, ok := authData["expires_at"]; ok {
				session["expires_at"] = expiresAt
			}

			// Create SDK-compatible structure
			response.Data = map[string]interface{}{
				"user":    user,
				"session": session,
			}
		}
	}

	return response, nil
}

// Storage operations implementation
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

// Helper functions

// buildFilterParam builds a filter parameter for Supabase PostgREST API
func buildFilterParam(filter FilterConfig) string {
	switch filter.Operator {
	case OpEq:
		return fmt.Sprintf("eq.%v", filter.Value)
	case OpNeq:
		return fmt.Sprintf("neq.%v", filter.Value)
	case OpGt:
		return fmt.Sprintf("gt.%v", filter.Value)
	case OpGte:
		return fmt.Sprintf("gte.%v", filter.Value)
	case OpLt:
		return fmt.Sprintf("lt.%v", filter.Value)
	case OpLte:
		return fmt.Sprintf("lte.%v", filter.Value)
	case OpLike:
		return fmt.Sprintf("like.%v", filter.Value)
	case OpILike:
		return fmt.Sprintf("ilike.%v", filter.Value)
	case OpIs:
		return fmt.Sprintf("is.%v", filter.Value)
	case OpIn:
		// Handle array values for IN operator
		if arr, ok := filter.Value.([]interface{}); ok {
			values := make([]string, len(arr))
			for i, v := range arr {
				values[i] = fmt.Sprintf("%v", v)
			}
			return fmt.Sprintf("in.(%s)", strings.Join(values, ","))
		}
		return fmt.Sprintf("in.(%v)", filter.Value)
	default:
		return fmt.Sprintf("%s.%v", filter.Operator, filter.Value)
	}
}

// parseSupabaseResponse parses HTTP response into SupabaseResponse
func parseSupabaseResponse(resp *http.Response) (*SupabaseResponse, error) {
	// Read response
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

	// Handle errors
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
	if len(body) > 0 {
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to parse response JSON: %w", err)
		}
		response.Data = data
	}

	return response, nil
}

// parseContentRange parses Content-Range header for count information
func parseContentRange(contentRange string) int {
	// Format: "0-24/573" or "*/573"
	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return -1
	}
	
	count, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1
	}
	
	return count
}

// parseConfig parses configuration from map to struct
func parseConfig(configData map[string]interface{}, config *SupabaseConfig) error {
	// Simple field mapping - in production you might want to use a library like mapstructure
	if url, ok := configData["url"].(string); ok {
		config.URL = url
	}
	if key, ok := configData["key"].(string); ok {
		config.Key = key
	}
	if operation, ok := configData["operation"].(string); ok {
		config.Operation = operation
	}
	if table, ok := configData["table"].(string); ok {
		config.Table = table
	}
	if timeout, ok := configData["timeout"].(string); ok {
		config.Timeout = timeout
	}

	// Parse operation-specific configs
	if selectData, ok := configData["select"].(map[string]interface{}); ok {
		config.Select = &SelectConfig{}
		parseSelectConfig(selectData, config.Select)
	}
	
	if insertData, ok := configData["insert"].(map[string]interface{}); ok {
		config.Insert = &InsertConfig{}
		parseInsertConfig(insertData, config.Insert)
	}
	
	if updateData, ok := configData["update"].(map[string]interface{}); ok {
		config.Update = &UpdateConfig{}
		parseUpdateConfig(updateData, config.Update)
	}
	
	if deleteData, ok := configData["delete"].(map[string]interface{}); ok {
		config.Delete = &DeleteConfig{}
		parseDeleteConfig(deleteData, config.Delete)
	}
	
	if rpcData, ok := configData["rpc"].(map[string]interface{}); ok {
		config.RPC = &RPCConfig{}
		parseRPCConfig(rpcData, config.RPC)
	}
	
	if authData, ok := configData["auth"].(map[string]interface{}); ok {
		config.Auth = &AuthConfig{}
		parseAuthConfig(authData, config.Auth)
	}
	
	if storageData, ok := configData["storage"].(map[string]interface{}); ok {
		config.Storage = &StorageConfig{}
		parseStorageConfig(storageData, config.Storage)
	}

	return nil
}

// Helper parsing functions for each config type
func parseSelectConfig(data map[string]interface{}, config *SelectConfig) {
	if columns, ok := data["columns"].([]interface{}); ok {
		config.Columns = make([]string, len(columns))
		for i, col := range columns {
			if colStr, ok := col.(string); ok {
				config.Columns[i] = colStr
			}
		}
	}
	
	if filters, ok := data["filters"].([]interface{}); ok {
		config.Filters = parseFilters(filters)
	}
	
	if order, ok := data["order"].([]interface{}); ok {
		config.Order = parseOrder(order)
	}
	
	if limit, ok := data["limit"].(float64); ok {
		limitInt := int(limit)
		config.Limit = &limitInt
	}
	
	if offset, ok := data["offset"].(float64); ok {
		offsetInt := int(offset)
		config.Offset = &offsetInt
	}
	
	if count, ok := data["count"].(string); ok {
		config.Count = count
	}
}

func parseInsertConfig(data map[string]interface{}, config *InsertConfig) {
	if dataField, ok := data["data"]; ok {
		config.Data = dataField
	}
	if upsert, ok := data["upsert"].(bool); ok {
		config.Upsert = upsert
	}
	if onConflict, ok := data["on_conflict"].(string); ok {
		config.OnConflict = onConflict
	}
}

func parseUpdateConfig(data map[string]interface{}, config *UpdateConfig) {
	if dataField, ok := data["data"].(map[string]interface{}); ok {
		config.Data = dataField
	}
	if filters, ok := data["filters"].([]interface{}); ok {
		config.Filters = parseFilters(filters)
	}
}

func parseDeleteConfig(data map[string]interface{}, config *DeleteConfig) {
	if filters, ok := data["filters"].([]interface{}); ok {
		config.Filters = parseFilters(filters)
	}
}

func parseRPCConfig(data map[string]interface{}, config *RPCConfig) {
	if function, ok := data["function"].(string); ok {
		config.Function = function
	}
	if params, ok := data["params"].(map[string]interface{}); ok {
		config.Params = params
	}
}

func parseAuthConfig(data map[string]interface{}, config *AuthConfig) {
	if email, ok := data["email"].(string); ok {
		config.Email = email
	}
	if password, ok := data["password"].(string); ok {
		config.Password = password
	}
	if userID, ok := data["user_id"].(string); ok {
		config.UserID = userID
	}
	if emailConfirm, ok := data["email_confirm"].(bool); ok {
		config.EmailConfirm = emailConfirm
	}
	if userMetadata, ok := data["user_metadata"].(map[string]interface{}); ok {
		config.UserMetadata = userMetadata
	}
	if appMetadata, ok := data["app_metadata"].(map[string]interface{}); ok {
		config.AppMetadata = appMetadata
	}
}

func parseStorageConfig(data map[string]interface{}, config *StorageConfig) {
	if bucket, ok := data["bucket"].(string); ok {
		config.Bucket = bucket
	}
	if path, ok := data["path"].(string); ok {
		config.Path = path
	}
	if fileContent, ok := data["file_content"].(string); ok {
		config.FileContent = fileContent
	}
	if filePath, ok := data["file_path"].(string); ok {
		config.FilePath = filePath
	}
	if public, ok := data["public"].(bool); ok {
		config.Public = public
	}
	if cacheControl, ok := data["cache_control"].(string); ok {
		config.CacheControl = cacheControl
	}
	if contentType, ok := data["content_type"].(string); ok {
		config.ContentType = contentType
	}
}

func parseFilters(filters []interface{}) []FilterConfig {
	result := make([]FilterConfig, 0, len(filters))
	for _, filterInterface := range filters {
		if filterMap, ok := filterInterface.(map[string]interface{}); ok {
			filter := FilterConfig{}
			if column, ok := filterMap["column"].(string); ok {
				filter.Column = column
			}
			if operator, ok := filterMap["operator"].(string); ok {
				filter.Operator = operator
			}
			if value, ok := filterMap["value"]; ok {
				filter.Value = value
			}
			result = append(result, filter)
		}
	}
	return result
}

func parseOrder(order []interface{}) []OrderConfig {
	result := make([]OrderConfig, 0, len(order))
	for _, orderInterface := range order {
		if orderMap, ok := orderInterface.(map[string]interface{}); ok {
			orderConfig := OrderConfig{Ascending: true} // default
			if column, ok := orderMap["column"].(string); ok {
				orderConfig.Column = column
			}
			if ascending, ok := orderMap["ascending"].(bool); ok {
				orderConfig.Ascending = ascending
			}
			result = append(result, orderConfig)
		}
	}
	return result
}

// processSave handles saving values from response
func processSave(ctx context.Context, response *SupabaseResponse, saveConfig map[string]interface{}, saved map[string]string) error {
	asName, ok := saveConfig["as"].(string)
	if !ok {
		return fmt.Errorf("'as' field is required for save config")
	}

	var value interface{}

	// Check for JSON path extraction
	if jsonPath, ok := saveConfig["json_path"].(string); ok {
		// Parse the JSON path using gojq
		query, err := gojq.Parse(jsonPath)
		if err != nil {
			return fmt.Errorf("failed to parse JSON path %s: %w", jsonPath, err)
		}

		// Run the query on the response data
		iter := query.Run(response.Data)
		v, ok := iter.Next()
		if !ok {
			responseDataJSON, _ := json.Marshal(response.Data)
			return fmt.Errorf("no results from JSON path %s on data %s", jsonPath, string(responseDataJSON))
		}
		if err, ok := v.(error); ok {
			return fmt.Errorf("error evaluating JSON path %s: %w", jsonPath, err)
		}
		value = v
	} else if header, ok := saveConfig["header"].(string); ok {
		// Extract from headers
		if response.Metadata != nil && response.Metadata.Headers != nil {
			value = response.Metadata.Headers[header]
		}
	} else {
		return fmt.Errorf("either 'json_path' or 'header' must be specified for save config")
	}

	// Convert value to string
	if value != nil {
		saved[asName] = fmt.Sprintf("%v", value)
	}

	return nil
}

// processAssertions validates assertions against the Supabase response
func processAssertions(response *SupabaseResponse, assertions []interface{}, params map[string]interface{}) error {
	// Rebuild state from parameters for variable replacement
	state := make(map[string]string)
	if stateInterface, ok := params["state"]; ok {
		if stateMap, ok := stateInterface.(map[string]interface{}); ok {
			for k, v := range stateMap {
				state[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	for _, assertionInterface := range assertions {
		assertion, ok := assertionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		assertionType, ok := assertion["type"].(string)
		if !ok {
			return fmt.Errorf("assertion type is required")
		}

		switch assertionType {
		case "status_code":
			if err := processStatusCodeAssertion(response, assertion, state); err != nil {
				return err
			}
		case "json_path":
			if err := processJSONPathAssertion(response, assertion, state); err != nil {
				return err
			}
		case "row_count":
			if err := processRowCountAssertion(response, assertion, state); err != nil {
				return err
			}
		case "error_code":
			if err := processErrorCodeAssertion(response, assertion, state); err != nil {
				return err
			}
		case "error_message":
			if err := processErrorMessageAssertion(response, assertion, state); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported assertion type: %s", assertionType)
		}
	}

	return nil
}

// processStatusCodeAssertion validates HTTP status code
func processStatusCodeAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for status_code assertion")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)
	expectedCode, err := strconv.Atoi(expectedStr)
	if err != nil {
		return fmt.Errorf("expected status code must be an integer: %s", expectedStr)
	}

	actualCode := response.Metadata.StatusCode
	if actualCode != expectedCode {
		return fmt.Errorf("status code assertion failed: expected %d, got %d", expectedCode, actualCode)
	}

	return nil
}

// processJSONPathAssertion validates JSON path expressions
func processJSONPathAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	path, ok := assertion["path"].(string)
	if !ok {
		return fmt.Errorf("path is required for json_path assertion")
	}

	// Parse the JSON path using gojq
	query, err := gojq.Parse(path)
	if err != nil {
		return fmt.Errorf("failed to parse JSON path %s: %w", path, err)
	}

	// Run the query on the response data
	iter := query.Run(response.Data)
	actualValue, ok := iter.Next()
	if !ok {
		return fmt.Errorf("no results from JSON path %s", path)
	}
	if err, ok := actualValue.(error); ok {
		return fmt.Errorf("error evaluating JSON path %s: %w", path, err)
	}

	// Check if we just need to verify existence
	if _, hasExpected := assertion["expected"]; !hasExpected {
		// Just checking for existence - if we got here, it exists
		return nil
	}

	// Compare with expected value
	expected := assertion["expected"]
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)

	// Convert actual value to string for comparison
	actualStr := fmt.Sprintf("%v", actualValue)

	if actualStr != expectedStr {
		return fmt.Errorf("json_path assertion failed at %s: expected %s, got %s", path, expectedStr, actualStr)
	}

	return nil
}

// processRowCountAssertion validates row count for select operations
func processRowCountAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for row_count assertion")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)
	expectedCount, err := strconv.Atoi(expectedStr)
	if err != nil {
		return fmt.Errorf("expected row count must be an integer: %s", expectedStr)
	}

	// Count rows in response data
	var actualCount int
	if response.Data != nil {
		if dataArray, ok := response.Data.([]interface{}); ok {
			actualCount = len(dataArray)
		} else {
			// Single object response counts as 1
			actualCount = 1
		}
	} else {
		actualCount = 0
	}

	if actualCount != expectedCount {
		return fmt.Errorf("row_count assertion failed: expected %d, got %d", expectedCount, actualCount)
	}

	return nil
}

// processErrorCodeAssertion validates error code in error responses
func processErrorCodeAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for error_code assertion")
	}

	if response.Error == nil {
		return fmt.Errorf("error_code assertion failed: no error in response")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)
	actualCode := response.Error.Code

	if actualCode != expectedStr {
		return fmt.Errorf("error_code assertion failed: expected %s, got %s", expectedStr, actualCode)
	}

	return nil
}

// processErrorMessageAssertion validates error message in error responses
func processErrorMessageAssertion(response *SupabaseResponse, assertion map[string]interface{}, state map[string]string) error {
	expected, ok := assertion["expected"]
	if !ok {
		return fmt.Errorf("expected value is required for error_message assertion")
	}

	if response.Error == nil {
		return fmt.Errorf("error_message assertion failed: no error in response")
	}

	// Replace variables in expected value
	expectedStr := replaceVariables(fmt.Sprintf("%v", expected), state)
	actualMessage := response.Error.Message

	if actualMessage != expectedStr {
		return fmt.Errorf("error_message assertion failed: expected %s, got %s", expectedStr, actualMessage)
	}

	return nil
}

// replaceVariables replaces template variables using the central DSL template system
func replaceVariables(input string, state map[string]string) string {
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	// Create template context with runtime variables
	// Config variables ({{ .vars.* }}) are processed earlier by CLI
	// Environment variables ({{ .env.* }}) are handled by DSL template system
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	// Use centralized template processing for consistent variable handling
	// This supports runtime vars, environment vars, and escaped handlebars
	result, err := dsl.ProcessTemplate(input, context)
	if err != nil {
		// If template processing fails, return original input to maintain backward compatibility
		// This can happen if the input contains invalid template syntax
		return input
	}
	
	return result
}

// processVariablesInMap recursively processes variables in a map structure using DSL template system
func processVariablesInMap(data map[string]interface{}, state map[string]string) map[string]interface{} {
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	// Create template context with runtime variables
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	// Use DSL recursive processing which handles all data types consistently
	// This leverages the same logic as ProcessConfigVariablesRecursive but for runtime vars
	result := processRuntimeVariablesRecursive(data, context)
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}
	
	// Fallback to original data if processing fails
	return data
}

// processRuntimeVariablesRecursive processes runtime variables in any nested data structure
func processRuntimeVariablesRecursive(data interface{}, context dsl.TemplateContext) interface{} {
	switch v := data.(type) {
	case string:
		result, err := dsl.ProcessTemplate(v, context)
		if err != nil {
			return v // Return original on error
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Process the key itself (in case it contains variables)
			processedKey, err := dsl.ProcessTemplate(key, context)
			if err != nil {
				processedKey = key // Use original key on error
			}
			// Process the value recursively
			result[processedKey] = processRuntimeVariablesRecursive(value, context)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = processRuntimeVariablesRecursive(item, context)
		}
		return result
	default:
		// For non-string types (numbers, booleans, etc.), return as-is
		return data
	}
}

// processFilters processes variables in filter configurations using DSL template system
func processFilters(filters []FilterConfig, state map[string]string) []FilterConfig {
	if filters == nil {
		return nil
	}
	
	// Convert state to interface{} map for DSL compatibility
	runtime := make(map[string]interface{})
	for k, v := range state {
		runtime[k] = v
	}
	
	// Create template context with runtime variables
	context := dsl.TemplateContext{
		Runtime: runtime,
	}
	
	result := make([]FilterConfig, len(filters))
	for i, filter := range filters {
		result[i] = FilterConfig{
			Column:   replaceVariables(filter.Column, state),
			Operator: filter.Operator,
			Value:    processRuntimeVariablesRecursive(filter.Value, context),
		}
	}
	return result
}