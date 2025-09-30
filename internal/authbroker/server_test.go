package authbroker

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/rocketship-ai/rocketship/internal/authbroker/store"
)

type fakeGitHub struct {
	deviceResp DeviceCodeResponse
	tokenResp  TokenResponse
	tokenErr   tokenError
	user       GitHubUser

	mu          sync.Mutex
	deviceCalls int
	tokenCalls  int
}

func (f *fakeGitHub) RequestDeviceCode(_ context.Context, _ []string) (DeviceCodeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deviceCalls++
	return f.deviceResp, nil
}

func (f *fakeGitHub) ExchangeDeviceCode(_ context.Context, _ string) (TokenResponse, tokenError, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenCalls++
	return f.tokenResp, f.tokenErr, nil
}

func (f *fakeGitHub) FetchUser(_ context.Context, _ string) (GitHubUser, error) {
	return f.user, nil
}

type memoryStore struct {
	mu   sync.Mutex
	data map[string]store.RefreshRecord
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: make(map[string]store.RefreshRecord)}
}

func (m *memoryStore) Save(token string, record store.RefreshRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[token] = record
	return nil
}

func (m *memoryStore) Get(token string) (store.RefreshRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.data[token]
	if !ok {
		return store.RefreshRecord{}, store.ErrNotFound
	}
	return rec, nil
}

func (m *memoryStore) Delete(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[token]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, token)
	return nil
}

func TestServerDeviceFlowAndRefresh(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	signer, err := buildSigner(key, "test-key")
	if err != nil {
		t.Fatalf("failed to build signer: %v", err)
	}

	cfg := Config{
		Issuer:          "https://cli.test",
		Audience:        "rocketship-cli",
		ClientID:        "rocketship-cli",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
		Scopes:          []string{"openid", "profile", "email"},
		GitHub: GitHubConfig{
			ClientID:     "gh-client",
			ClientSecret: "gh-secret",
			Scopes:       []string{"read:user"},
		},
	}

	fake := &fakeGitHub{
		deviceResp: DeviceCodeResponse{
			DeviceCode:              "device-123",
			UserCode:                "code-456",
			VerificationURI:         "https://github.com/login/device",
			VerificationURIComplete: "https://github.com/login/device?user_code=code-456",
			RawExpiresIn:            600,
			RawInterval:             5,
			ExpiresIn:               10 * time.Minute,
			Interval:                5 * time.Second,
		},
		tokenResp: TokenResponse{
			AccessToken: "gh-access",
			TokenType:   "bearer",
			Scope:       "read:user",
		},
		user: GitHubUser{
			ID:    1234,
			Login: "octo",
			Name:  "Octo Cat",
			Email: "octo@example.com",
		},
	}

	memStore := newMemoryStore()

	srv, err := newServerWithComponents(cfg, signer, fake, memStore)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/device/code", strings.NewReader(url.Values{
		"client_id": {cfg.ClientID},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("device code request failed: %d", recorder.Code)
	}
	var dcResp struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &dcResp); err != nil {
		t.Fatalf("failed to parse device response: %v", err)
	}
	if dcResp.DeviceCode != "device-123" {
		t.Fatalf("unexpected device code: %s", dcResp.DeviceCode)
	}

	tokenReq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(url.Values{
		"client_id":   {cfg.ClientID},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {dcResp.DeviceCode},
	}.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRec := httptest.NewRecorder()
	srv.ServeHTTP(tokenRec, tokenReq)

	if tokenRec.Code != http.StatusOK {
		t.Fatalf("token exchange failed: %d", tokenRec.Code)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("failed to parse token response: %v", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		t.Fatalf("expected access and refresh tokens in response")
	}
	if tokenResp.TokenType != "Bearer" {
		t.Fatalf("unexpected token type: %s", tokenResp.TokenType)
	}

	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(tokenResp.AccessToken, claims, func(token *jwt.Token) (interface{}, error) {
		return &key.PublicKey, nil
	}); err != nil {
		t.Fatalf("issued token failed validation: %v", err)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokenResp.RefreshToken},
	}.Encode()))
	refreshReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	refreshRec := httptest.NewRecorder()
	srv.ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh grant failed: %d", refreshRec.Code)
	}

	var refreshResp struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
	}
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &refreshResp); err != nil {
		t.Fatalf("failed to parse refresh response: %v", err)
	}
	if refreshResp.RefreshToken == tokenResp.RefreshToken {
		t.Fatalf("expected refresh token rotation")
	}
	if refreshResp.AccessToken == tokenResp.AccessToken {
		t.Fatalf("expected new access token")
	}
}

func TestServerRejectsUnknownClient(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	signer, err := buildSigner(key, "test-key")
	if err != nil {
		t.Fatalf("failed to build signer: %v", err)
	}

	cfg := Config{
		Issuer:          "https://cli.test",
		Audience:        "rocketship-cli",
		ClientID:        "rocketship-cli",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
		Scopes:          []string{"openid"},
		GitHub:          GitHubConfig{ClientID: "gh", ClientSecret: "secret"},
	}

	fake := &fakeGitHub{deviceResp: DeviceCodeResponse{}}
	srv, err := newServerWithComponents(cfg, signer, fake, newMemoryStore())
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/device/code", strings.NewReader(url.Values{
		"client_id": {"other-client"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for unknown client, got %d", rec.Code)
	}
}

func TestServerJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	signer, err := buildSigner(key, "test-key")
	if err != nil {
		t.Fatalf("failed to build signer: %v", err)
	}

	cfg := Config{}
	fake := &fakeGitHub{}
	srv, err := newServerWithComponents(cfg, signer, fake, newMemoryStore())
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected OK status, got %d", rec.Code)
	}
	body := rec.Body.Bytes()
	if !bytes.Contains(body, []byte("\"keys\"")) {
		t.Fatalf("jwks response missing keys: %s", string(body))
	}
}
