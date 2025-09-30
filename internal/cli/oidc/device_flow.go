package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/cli/auth"
)

// FlowConfig captures the endpoints and identifiers required for device flow.
type FlowConfig struct {
	ClientID       string
	Audience       string
	Scopes         []string
	DeviceEndpoint string
	TokenEndpoint  string
	Issuer         string
}

// DeviceCode represents the response from the device authorization endpoint.
type DeviceCode struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               time.Duration
	Interval                time.Duration
}

// StartDeviceAuthorization initiates the device authorization grant.
func StartDeviceAuthorization(ctx context.Context, cfg FlowConfig) (*DeviceCode, error) {
	if cfg.DeviceEndpoint == "" {
		return nil, fmt.Errorf("device authorization endpoint missing")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("client id missing")
	}

	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	if len(cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	if cfg.Audience != "" {
		form.Set("audience", cfg.Audience)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DeviceEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device authorization request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		snippet := strings.TrimSpace(string(b))
		if snippet == "" {
			snippet = resp.Status
		}
		return nil, fmt.Errorf("device authorization failed: %s", snippet)
	}

	var dr deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, fmt.Errorf("failed to decode device authorization response: %w", err)
	}

	code := &DeviceCode{
		DeviceCode:              dr.DeviceCode,
		UserCode:                dr.UserCode,
		VerificationURI:         dr.VerificationURI,
		VerificationURIComplete: dr.VerificationURIComplete,
		ExpiresIn:               time.Duration(dr.ExpiresIn) * time.Second,
	}
	interval := dr.Interval
	if interval <= 0 {
		interval = 5
	}
	code.Interval = time.Duration(interval) * time.Second

	return code, nil
}

// PollToken polls the token endpoint until the user completes authentication.
func PollToken(ctx context.Context, cfg FlowConfig, code *DeviceCode) (auth.TokenData, error) {
	if cfg.TokenEndpoint == "" {
		return auth.TokenData{}, fmt.Errorf("token endpoint missing")
	}
	if code == nil {
		return auth.TokenData{}, fmt.Errorf("device code response missing")
	}

	deadline := time.Now().Add(code.ExpiresIn)
	interval := code.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	for {
		if time.Now().After(deadline) {
			return auth.TokenData{}, fmt.Errorf("device code expired before approval")
		}

		token, retry, err := exchangeDeviceCode(ctx, cfg, code.DeviceCode)
		if err == nil {
			token.DeviceEndpoint = cfg.DeviceEndpoint
			token.TokenEndpoint = cfg.TokenEndpoint
			token.ClientID = cfg.ClientID
			token.Audience = cfg.Audience
			token.Issuer = cfg.Issuer
			if len(token.Scopes) == 0 {
				token.Scopes = cfg.Scopes
			}
			return token, nil
		}

		switch retry {
		case retryContinue:
			select {
			case <-time.After(interval):
				continue
			case <-ctx.Done():
				return auth.TokenData{}, ctx.Err()
			}
		case retrySlowDown:
			interval += 5 * time.Second
			select {
			case <-time.After(interval):
				continue
			case <-ctx.Done():
				return auth.TokenData{}, ctx.Err()
			}
		default:
			return auth.TokenData{}, err
		}
	}
}

const (
	retryNone retryDirective = iota
	retryContinue
	retrySlowDown
)

type retryDirective int

func exchangeDeviceCode(ctx context.Context, cfg FlowConfig, deviceCode string) (auth.TokenData, retryDirective, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", deviceCode)
	form.Set("client_id", cfg.ClientID)
	if cfg.Audience != "" {
		form.Set("audience", cfg.Audience)
	}
	if len(cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.TokenData{}, retryNone, fmt.Errorf("failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return auth.TokenData{}, retryContinue, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		var tr tokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			return auth.TokenData{}, retryNone, fmt.Errorf("failed to decode token response: %w", err)
		}
		return tokenDataFromResponse(tr, cfg), retryNone, nil
	}

	var er tokenErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return auth.TokenData{}, retryNone, fmt.Errorf("unexpected token error: %s", resp.Status)
	}

	switch er.Error {
	case "authorization_pending":
		return auth.TokenData{}, retryContinue, fmt.Errorf("authorization pending")
	case "slow_down":
		return auth.TokenData{}, retrySlowDown, fmt.Errorf("slow down polling")
	case "expired_token":
		return auth.TokenData{}, retryNone, fmt.Errorf("device code expired")
	case "access_denied":
		return auth.TokenData{}, retryNone, fmt.Errorf("end user denied the request")
	default:
		desc := strings.TrimSpace(er.ErrorDescription)
		if desc == "" {
			desc = er.Error
		}
		return auth.TokenData{}, retryNone, fmt.Errorf("token exchange failed: %s", desc)
	}
}

func tokenDataFromResponse(tr tokenResponse, cfg FlowConfig) auth.TokenData {
	expiry := time.Time{}
	if tr.ExpiresIn > 0 {
		expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	scopes := cfg.Scopes
	if tr.Scope != "" {
		scopes = strings.Fields(tr.Scope)
	}
	return auth.TokenData{
		AccessToken:    tr.AccessToken,
		RefreshToken:   tr.RefreshToken,
		TokenType:      tr.TokenType,
		Expiry:         expiry,
		Scopes:         scopes,
		IDToken:        tr.IDToken,
		TokenEndpoint:  cfg.TokenEndpoint,
		DeviceEndpoint: cfg.DeviceEndpoint,
		ClientID:       cfg.ClientID,
		Audience:       cfg.Audience,
		Issuer:         cfg.Issuer,
	}
}

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval,omitempty"`
}
