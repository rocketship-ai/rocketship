package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

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
