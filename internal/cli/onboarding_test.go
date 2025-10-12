package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/cli/auth"
)

// newTestAuthManager creates a test auth manager with a temporary directory
func newTestAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	tmpDir := t.TempDir()

	// Since Manager is a struct with unexported fields, we need to create it properly
	// Let's set the env var to disable keyring and create a new manager
	_ = os.Setenv("ROCKETSHIP_DISABLE_KEYRING", "true")
	t.Cleanup(func() { _ = os.Unsetenv("ROCKETSHIP_DISABLE_KEYRING") })

	// Temporarily change home dir for testing
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	// Create .rocketship directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".rocketship"), 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	mgr, err := auth.NewManager()
	if err != nil {
		t.Fatalf("failed to create auth manager: %v", err)
	}

	// Save test token
	testToken := auth.TokenData{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Issuer:       "http://test",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	if err := mgr.Save("test", testToken); err != nil {
		t.Fatalf("failed to save test token: %v", err)
	}

	return mgr
}

// newTestBrokerClient creates a test broker client with mock dependencies
func newTestBrokerClient(t *testing.T, serverURL string, httpClient *http.Client) *brokerClient {
	t.Helper()
	return &brokerClient{
		baseURL:    serverURL,
		profile:    "test",
		manager:    newTestAuthManager(t),
		httpClient: httpClient,
	}
}

func TestBrokerClient_CurrentUser(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		serverBody     string
		expectErr      bool
		checkResponse  func(*testing.T, onboardingState)
	}{
		{
			name:           "ready user with roles",
			serverResponse: http.StatusOK,
			serverBody: `{
				"status": "ready",
				"roles": ["owner", "editor"],
				"user": {
					"id": "` + uuid.New().String() + `",
					"email": "user@example.com"
				}
			}`,
			expectErr: false,
			checkResponse: func(t *testing.T, state onboardingState) {
				if state.Status != "ready" {
					t.Errorf("status mismatch: got %q, want %q", state.Status, "ready")
				}
				if len(state.Roles) != 2 {
					t.Errorf("roles length mismatch: got %d, want 2", len(state.Roles))
				}
				if state.Roles[0] != "owner" || state.Roles[1] != "editor" {
					t.Errorf("roles mismatch: got %v", state.Roles)
				}
			},
		},
		{
			name:           "pending user with registration",
			serverResponse: http.StatusOK,
			serverBody: `{
				"status": "pending",
				"roles": ["pending"],
				"user": {"id": "` + uuid.New().String() + `", "email": "user@example.com"},
				"pending_registration": {
					"registration_id": "` + uuid.New().String() + `",
					"org_name": "test-org",
					"email": "test@example.com",
					"expires_at": "` + time.Now().Add(15*time.Minute).Format(time.RFC3339) + `",
					"resend_available_at": "` + time.Now().Add(5*time.Minute).Format(time.RFC3339) + `",
					"attempts": 0,
					"max_attempts": 10
				}
			}`,
			expectErr: false,
			checkResponse: func(t *testing.T, state onboardingState) {
				if state.Status != "pending" {
					t.Errorf("status mismatch: got %q, want %q", state.Status, "pending")
				}
				if state.PendingRegistration == nil {
					t.Fatal("expected pending registration to be non-nil")
				}
				if state.PendingRegistration.OrgName != "test-org" {
					t.Errorf("org name mismatch: got %q, want %q", state.PendingRegistration.OrgName, "test-org")
				}
				if state.PendingRegistration.Email != "test@example.com" {
					t.Errorf("email mismatch: got %q, want %q", state.PendingRegistration.Email, "test@example.com")
				}
			},
		},
		{
			name:           "server error",
			serverResponse: http.StatusInternalServerError,
			serverBody:     `{"error": "internal server error"}`,
			expectErr:      true,
		},
		{
			name:           "unauthorized",
			serverResponse: http.StatusUnauthorized,
			serverBody:     `{"error": "unauthorized"}`,
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/users/me" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}

				w.WriteHeader(tt.serverResponse)
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client := newTestBrokerClient(t, server.URL, server.Client())

			state, err := client.currentUser(context.Background())

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkResponse != nil {
					tt.checkResponse(t, state)
				}
			}
		})
	}
}

func TestBrokerClient_StartRegistration(t *testing.T) {
	tests := []struct {
		name           string
		orgName        string
		email          string
		serverResponse int
		serverBody     string
		expectErr      bool
		checkRequest   func(*testing.T, map[string]string)
		checkResponse  func(*testing.T, registrationInfo)
	}{
		{
			name:           "successful registration",
			orgName:        "test-org",
			email:          "test@example.com",
			serverResponse: http.StatusCreated,
			serverBody: `{
				"registration_id": "` + uuid.New().String() + `",
				"org_name": "test-org",
				"email": "test@example.com",
				"expires_at": "` + time.Now().Add(15*time.Minute).Format(time.RFC3339) + `",
				"resend_available_at": "` + time.Now().Add(5*time.Minute).Format(time.RFC3339) + `"
			}`,
			expectErr: false,
			checkRequest: func(t *testing.T, body map[string]string) {
				if body["name"] != "test-org" {
					t.Errorf("name mismatch: got %q, want %q", body["name"], "test-org")
				}
				if body["email"] != "test@example.com" {
					t.Errorf("email mismatch: got %q, want %q", body["email"], "test@example.com")
				}
			},
			checkResponse: func(t *testing.T, info registrationInfo) {
				if info.OrgName != "test-org" {
					t.Errorf("org name mismatch: got %q, want %q", info.OrgName, "test-org")
				}
				if info.Email != "test@example.com" {
					t.Errorf("email mismatch: got %q, want %q", info.Email, "test@example.com")
				}
				if info.ID == "" {
					t.Error("registration ID should not be empty")
				}
			},
		},
		{
			name:           "org name already exists",
			orgName:        "existing-org",
			email:          "test@example.com",
			serverResponse: http.StatusConflict,
			serverBody:     `{"error": "organization name already exists"}`,
			expectErr:      true,
		},
		{
			name:           "invalid email",
			orgName:        "test-org",
			email:          "invalid-email",
			serverResponse: http.StatusBadRequest,
			serverBody:     `{"error": "invalid email address"}`,
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/orgs/registration/start" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				w.WriteHeader(tt.serverResponse)
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client := newTestBrokerClient(t, server.URL, server.Client())

			info, err := client.startRegistration(context.Background(), tt.orgName, tt.email)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, receivedBody)
				}
				if tt.checkResponse != nil {
					tt.checkResponse(t, info)
				}
			}
		})
	}
}

func TestBrokerClient_CompleteRegistration(t *testing.T) {
	tests := []struct {
		name           string
		registrationID string
		code           string
		serverResponse int
		serverBody     string
		expectErr      bool
		checkRequest   func(*testing.T, map[string]string)
		checkResponse  func(*testing.T, completionResult)
	}{
		{
			name:           "successful completion",
			registrationID: uuid.New().String(),
			code:           "123456",
			serverResponse: http.StatusOK,
			serverBody:     `{"needs_claim_refresh": true}`,
			expectErr:      false,
			checkRequest: func(t *testing.T, body map[string]string) {
				if body["code"] != "123456" {
					t.Errorf("code mismatch: got %q, want %q", body["code"], "123456")
				}
			},
			checkResponse: func(t *testing.T, result completionResult) {
				if !result.NeedsRefresh {
					t.Error("expected NeedsRefresh to be true")
				}
			},
		},
		{
			name:           "invalid code",
			registrationID: uuid.New().String(),
			code:           "000000",
			serverResponse: http.StatusUnauthorized,
			serverBody:     `{"error": "invalid verification code"}`,
			expectErr:      true,
		},
		{
			name:           "expired registration",
			registrationID: uuid.New().String(),
			code:           "123456",
			serverResponse: http.StatusGone,
			serverBody:     `{"error": "registration expired"}`,
			expectErr:      true,
		},
		{
			name:           "max attempts exceeded",
			registrationID: uuid.New().String(),
			code:           "123456",
			serverResponse: http.StatusTooManyRequests,
			serverBody:     `{"error": "too many attempts"}`,
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/orgs/registration/complete" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				w.WriteHeader(tt.serverResponse)
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client := newTestBrokerClient(t, server.URL, server.Client())

			result, err := client.completeRegistration(context.Background(), tt.registrationID, tt.code)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, receivedBody)
				}
				if tt.checkResponse != nil {
					tt.checkResponse(t, result)
				}
			}
		})
	}
}

func TestBrokerClient_AcceptInvite(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		serverResponse int
		serverBody     string
		expectErr      bool
		checkRequest   func(*testing.T, map[string]string)
	}{
		{
			name:           "successful acceptance",
			code:           "INVITE123",
			serverResponse: http.StatusOK,
			serverBody:     `{"needs_claim_refresh": true}`,
			expectErr:      false,
			checkRequest: func(t *testing.T, body map[string]string) {
				if body["code"] != "INVITE123" {
					t.Errorf("code mismatch: got %q, want %q", body["code"], "INVITE123")
				}
			},
		},
		{
			name:           "invalid invite code",
			code:           "INVALID",
			serverResponse: http.StatusNotFound,
			serverBody:     `{"error": "invite not found"}`,
			expectErr:      true,
		},
		{
			name:           "expired invite",
			code:           "EXPIRED123",
			serverResponse: http.StatusGone,
			serverBody:     `{"error": "invite expired"}`,
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/orgs/invites/accept" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				w.WriteHeader(tt.serverResponse)
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client := newTestBrokerClient(t, server.URL, server.Client())

			result, err := client.acceptInvite(context.Background(), tt.code)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !result.NeedsRefresh {
					t.Error("expected NeedsRefresh to be true for successful invite acceptance")
				}
				if tt.checkRequest != nil {
					tt.checkRequest(t, receivedBody)
				}
			}
		})
	}
}

func TestBrokerClient_DecodeError(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		body         string
		expectErrMsg string
	}{
		{
			name:         "json error response",
			statusCode:   http.StatusBadRequest,
			body:         `{"error": "invalid request"}`,
			expectErrMsg: "invalid request",
		},
		{
			name:         "plain text error",
			statusCode:   http.StatusInternalServerError,
			body:         "internal server error",
			expectErrMsg: "internal server error",
		},
		{
			name:         "empty body",
			statusCode:   http.StatusNotFound,
			body:         "",
			expectErrMsg: "Not Found",
		},
		{
			name:         "malformed json",
			statusCode:   http.StatusBadRequest,
			body:         `{invalid json`,
			expectErrMsg: "{invalid json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
				Body:       http.NoBody,
			}

			if tt.body != "" {
				resp.Body = &closingStringReader{Reader: strings.NewReader(tt.body)}
			}

			client := newTestBrokerClient(t, "http://test", http.DefaultClient)
			err := client.decodeError(resp)

			if err == nil {
				t.Fatal("expected error but got none")
			}

			apiErr, ok := err.(*apiError)
			if !ok {
				t.Fatalf("expected *apiError, got %T", err)
			}

			if apiErr.Status != tt.statusCode {
				t.Errorf("status mismatch: got %d, want %d", apiErr.Status, tt.statusCode)
			}

			if !strings.Contains(apiErr.Message, tt.expectErrMsg) {
				t.Errorf("error message mismatch: got %q, want to contain %q", apiErr.Message, tt.expectErrMsg)
			}
		})
	}
}

// Helper to make strings.Reader closeable
type closingStringReader struct {
	*strings.Reader
}

func (c *closingStringReader) Close() error {
	return nil
}

func TestRegistrationInfo_Struct(t *testing.T) {
	// Test that registrationInfo struct has all expected fields
	now := time.Now()
	info := registrationInfo{
		ID:                "test-id",
		OrgName:           "test-org",
		Email:             "test@example.com",
		ExpiresAt:         now.Add(15 * time.Minute),
		ResendAvailableAt: now.Add(5 * time.Minute),
		Attempts:          2,
		MaxAttempts:       10,
	}

	if info.ID != "test-id" {
		t.Errorf("ID mismatch: got %q, want %q", info.ID, "test-id")
	}
	if info.OrgName != "test-org" {
		t.Errorf("OrgName mismatch: got %q, want %q", info.OrgName, "test-org")
	}
	if info.Email != "test@example.com" {
		t.Errorf("Email mismatch: got %q, want %q", info.Email, "test@example.com")
	}
	if info.Attempts != 2 {
		t.Errorf("Attempts mismatch: got %d, want %d", info.Attempts, 2)
	}
	if info.MaxAttempts != 10 {
		t.Errorf("MaxAttempts mismatch: got %d, want %d", info.MaxAttempts, 10)
	}
}

func TestInviteInfo_Struct(t *testing.T) {
	// Test that inviteInfo struct has all expected fields
	now := time.Now()
	info := inviteInfo{
		InviteID:         "invite-123",
		OrganizationID:   "org-456",
		OrganizationName: "test-org",
		Role:             "member",
		ExpiresAt:        now.Add(24 * time.Hour),
	}

	if info.InviteID != "invite-123" {
		t.Errorf("InviteID mismatch: got %q, want %q", info.InviteID, "invite-123")
	}
	if info.OrganizationID != "org-456" {
		t.Errorf("OrganizationID mismatch: got %q, want %q", info.OrganizationID, "org-456")
	}
	if info.OrganizationName != "test-org" {
		t.Errorf("OrganizationName mismatch: got %q, want %q", info.OrganizationName, "test-org")
	}
	if info.Role != "member" {
		t.Errorf("Role mismatch: got %q, want %q", info.Role, "member")
	}
}

func TestCompletionResult_Struct(t *testing.T) {
	result := completionResult{
		NeedsRefresh: true,
		Message:      "success",
	}

	if !result.NeedsRefresh {
		t.Error("expected NeedsRefresh to be true")
	}
	if result.Message != "success" {
		t.Errorf("Message mismatch: got %q, want %q", result.Message, "success")
	}
}

func TestOnboardingState_Struct(t *testing.T) {
	state := onboardingState{
		Status: "ready",
		Roles:  []string{"owner", "editor"},
		PendingRegistration: &registrationInfo{
			ID:      "reg-123",
			OrgName: "test-org",
			Email:   "test@example.com",
		},
		PendingInvites: []inviteInfo{
			{
				InviteID:         "inv-123",
				OrganizationName: "other-org",
			},
		},
	}

	if state.Status != "ready" {
		t.Errorf("Status mismatch: got %q, want %q", state.Status, "ready")
	}
	if len(state.Roles) != 2 {
		t.Errorf("Roles length mismatch: got %d, want 2", len(state.Roles))
	}
	if state.PendingRegistration == nil {
		t.Error("expected PendingRegistration to be non-nil")
	}
	if len(state.PendingInvites) != 1 {
		t.Errorf("PendingInvites length mismatch: got %d, want 1", len(state.PendingInvites))
	}
}
