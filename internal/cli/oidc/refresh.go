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

const httpTimeout = 30 * time.Second

// RefreshAccessToken exchanges a refresh token for new credentials.
func RefreshAccessToken(ctx context.Context, current auth.TokenData) (auth.TokenData, error) {
	if current.TokenEndpoint == "" {
		return auth.TokenData{}, fmt.Errorf("token endpoint is not set")
	}
	if current.RefreshToken == "" {
		return auth.TokenData{}, fmt.Errorf("refresh token missing")
	}
	if current.ClientID == "" {
		return auth.TokenData{}, fmt.Errorf("client id missing for refresh")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", current.RefreshToken)
	form.Set("client_id", current.ClientID)
	if current.Audience != "" {
		form.Set("audience", current.Audience)
	}
	if len(current.Scopes) > 0 {
		form.Set("scope", strings.Join(current.Scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, current.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.TokenData{}, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return auth.TokenData{}, fmt.Errorf("refresh token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		snippet := strings.TrimSpace(string(b))
		if snippet == "" {
			snippet = resp.Status
		}
		return auth.TokenData{}, fmt.Errorf("refresh token request failed: %s", snippet)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return auth.TokenData{}, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	updated := current
	updated.AccessToken = tr.AccessToken
	updated.TokenType = tr.TokenType
	if tr.ExpiresIn > 0 {
		updated.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		updated.Expiry = time.Time{}
	}
	if tr.RefreshToken != "" {
		updated.RefreshToken = tr.RefreshToken
	}
	if tr.IDToken != "" {
		updated.IDToken = tr.IDToken
	}
	if tr.Scope != "" {
		updated.Scopes = strings.Fields(tr.Scope)
	}

	return updated, nil
}

// tokenResponse captures the subset of OAuth token response fields we care about.
