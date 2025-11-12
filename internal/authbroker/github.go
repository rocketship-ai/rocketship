package authbroker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 15 * time.Second

type GitHubClient struct {
	cfg    GitHubConfig
	client *http.Client
}

type DeviceCodeResponse struct {
	DeviceCode              string        `json:"device_code"`
	UserCode                string        `json:"user_code"`
	VerificationURI         string        `json:"verification_uri"`
	VerificationURIComplete string        `json:"verification_uri_complete"`
	ExpiresIn               time.Duration `json:"-"`
	Interval                time.Duration `json:"-"`
	RawExpiresIn            int           `json:"expires_in"`
	RawInterval             int           `json:"interval"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type tokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func NewGitHubClient(cfg GitHubConfig, client *http.Client) *GitHubClient {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &GitHubClient{cfg: cfg, client: client}
}

func (g *GitHubClient) RequestDeviceCode(ctx context.Context, scopes []string) (DeviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", g.cfg.ClientID)
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.DeviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCodeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return DeviceCodeResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		snippet := strings.TrimSpace(string(body))
		if snippet == "" {
			snippet = resp.Status
		}
		return DeviceCodeResponse{}, fmt.Errorf("github device code request failed: %s", snippet)
	}

	var dc DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return DeviceCodeResponse{}, err
	}
	if dc.RawExpiresIn <= 0 {
		dc.RawExpiresIn = 900
	}
	if dc.RawInterval <= 0 {
		dc.RawInterval = 5
	}
	dc.ExpiresIn = time.Duration(dc.RawExpiresIn) * time.Second
	dc.Interval = time.Duration(dc.RawInterval) * time.Second
	return dc, nil
}

func (g *GitHubClient) ExchangeDeviceCode(ctx context.Context, deviceCode string) (TokenResponse, tokenError, error) {
	form := url.Values{}
	form.Set("client_id", g.cfg.ClientID)
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, tokenError{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return TokenResponse{}, tokenError{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		var token TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
			return TokenResponse{}, tokenError{}, err
		}
		if strings.TrimSpace(token.AccessToken) == "" {
			return TokenResponse{}, tokenError{Error: "authorization_pending", ErrorDescription: "GitHub has not issued an access token yet"}, nil
		}
		return token, tokenError{}, nil
	}

	var terr tokenError
	if err := json.NewDecoder(resp.Body).Decode(&terr); err != nil {
		body, _ := io.ReadAll(resp.Body)
		return TokenResponse{}, tokenError{}, fmt.Errorf("github token exchange failed: %s", strings.TrimSpace(string(body)))
	}
	return TokenResponse{}, terr, nil
}

// ExchangeAuthorizationCode exchanges an authorization code for an access token (Web Application Flow).
// This is used for browser-based OAuth flows where the user is redirected back to the application.
func (g *GitHubClient) ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", g.cfg.ClientID)
	form.Set("client_secret", g.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	// GitHub OAuth Apps support PKCE alongside client_secret - both are required
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, fmt.Errorf("github authorization code exchange failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return TokenResponse{}, fmt.Errorf("failed to parse token response: %w", err)
	}

	if strings.TrimSpace(token.AccessToken) == "" {
		return TokenResponse{}, fmt.Errorf("github did not return an access token (response: %s)", string(body))
	}

	return token, nil
}

func (g *GitHubClient) FetchUser(ctx context.Context, accessToken string) (GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.cfg.UserURL, nil)
	if err != nil {
		return GitHubUser{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "rocketship-auth-broker")
	resp, err := g.client.Do(req)
	if err != nil {
		return GitHubUser{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return GitHubUser{}, fmt.Errorf("github user request failed: %s", strings.TrimSpace(string(body)))
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return GitHubUser{}, err
	}

	if user.Email == "" {
		email, err := g.fetchPrimaryEmail(ctx, accessToken)
		if err == nil {
			user.Email = email
		}
	}

	return user, nil
}

func (g *GitHubClient) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.cfg.EmailsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "rocketship-auth-broker")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to fetch primary email")
	}

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified && e.Email != "" {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified && e.Email != "" {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", errors.New("no email returned by GitHub")
}
