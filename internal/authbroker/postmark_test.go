package authbroker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewPostmarkMailer(t *testing.T) {
	tests := []struct {
		name      string
		cfg       EmailConfig
		expectErr bool
	}{
		{
			name: "valid config",
			cfg: EmailConfig{
				PostmarkToken: "test-token",
				FromAddress:   "test@example.com",
			},
			expectErr: false,
		},
		{
			name: "missing token",
			cfg: EmailConfig{
				PostmarkToken: "",
				FromAddress:   "test@example.com",
			},
			expectErr: true,
		},
		{
			name: "missing from address",
			cfg: EmailConfig{
				PostmarkToken: "test-token",
				FromAddress:   "",
			},
			expectErr: true,
		},
		{
			name: "whitespace only token",
			cfg: EmailConfig{
				PostmarkToken: "   ",
				FromAddress:   "test@example.com",
			},
			expectErr: true,
		},
		{
			name: "whitespace only from address",
			cfg: EmailConfig{
				PostmarkToken: "test-token",
				FromAddress:   "   ",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailer, err := newPostmarkMailer(tt.cfg)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				if mailer != nil {
					t.Error("expected nil mailer on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if mailer == nil {
					t.Error("expected non-nil mailer")
				}
				if mailer != nil {
					if mailer.token != tt.cfg.PostmarkToken {
						t.Errorf("token mismatch: got %q, want %q", mailer.token, tt.cfg.PostmarkToken)
					}
					if mailer.from != tt.cfg.FromAddress {
						t.Errorf("from mismatch: got %q, want %q", mailer.from, tt.cfg.FromAddress)
					}
					if mailer.baseURL != defaultPostmarkURL {
						t.Errorf("baseURL mismatch: got %q, want %q", mailer.baseURL, defaultPostmarkURL)
					}
					if mailer.client == nil {
						t.Error("client should not be nil")
					}
				}
			}
		})
	}
}

func TestPostmarkMailer_SendOrgVerification(t *testing.T) {
	tests := []struct {
		name           string
		toEmail        string
		orgName        string
		code           string
		expiresAt      time.Time
		serverResponse int
		serverBody     string
		expectErr      bool
		expectToken    string
	}{
		{
			name:           "successful send",
			toEmail:        "user@example.com",
			orgName:        "test-org",
			code:           "123456",
			expiresAt:      time.Now().Add(15 * time.Minute),
			serverResponse: http.StatusOK,
			serverBody:     `{"MessageID":"test-message-id"}`,
			expectErr:      false,
			expectToken:    "test-token",
		},
		{
			name:           "postmark error response",
			toEmail:        "user@example.com",
			orgName:        "test-org",
			code:           "123456",
			expiresAt:      time.Now().Add(15 * time.Minute),
			serverResponse: http.StatusBadRequest,
			serverBody:     `{"ErrorCode":300,"Message":"Invalid email address"}`,
			expectErr:      true,
			expectToken:    "test-token",
		},
		{
			name:           "expired verification code",
			toEmail:        "user@example.com",
			orgName:        "test-org",
			code:           "123456",
			expiresAt:      time.Now().Add(-5 * time.Minute),
			serverResponse: http.StatusOK,
			serverBody:     `{"MessageID":"test-message-id"}`,
			expectErr:      false,
			expectToken:    "test-token",
		},
		{
			name:           "whitespace handling in email",
			toEmail:        "  user@example.com  ",
			orgName:        "  test-org  ",
			code:           "  123456  ",
			expiresAt:      time.Now().Add(15 * time.Minute),
			serverResponse: http.StatusOK,
			serverBody:     `{"MessageID":"test-message-id"}`,
			expectErr:      false,
			expectToken:    "test-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedMsg postmarkMessage
			var receivedToken string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedToken = r.Header.Get("X-Postmark-Server-Token")

				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}

				if err := json.Unmarshal(body, &receivedMsg); err != nil {
					t.Fatalf("failed to unmarshal request body: %v", err)
				}

				w.WriteHeader(tt.serverResponse)
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			mailer := &postmarkMailer{
				client:  server.Client(),
				token:   tt.expectToken,
				from:    "noreply@example.com",
				baseURL: server.URL,
			}

			ctx := context.Background()
			err := mailer.SendOrgVerification(ctx, tt.toEmail, tt.orgName, tt.code, tt.expiresAt)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Verify the token was sent correctly
				if receivedToken != tt.expectToken {
					t.Errorf("token mismatch: got %q, want %q", receivedToken, tt.expectToken)
				}

				// Verify the message structure
				if receivedMsg.From != "noreply@example.com" {
					t.Errorf("from mismatch: got %q, want %q", receivedMsg.From, "noreply@example.com")
				}
				if receivedMsg.To != strings.TrimSpace(tt.toEmail) {
					t.Errorf("to mismatch: got %q, want %q", receivedMsg.To, strings.TrimSpace(tt.toEmail))
				}
				if receivedMsg.Subject != "Confirm your Rocketship organization" {
					t.Errorf("subject mismatch: got %q", receivedMsg.Subject)
				}
				if !strings.Contains(receivedMsg.TextBody, strings.TrimSpace(tt.code)) {
					t.Errorf("code not found in body: %q", receivedMsg.TextBody)
				}
				if !strings.Contains(receivedMsg.TextBody, strings.TrimSpace(tt.orgName)) {
					t.Errorf("org name not found in body: %q", receivedMsg.TextBody)
				}
				if receivedMsg.MessageStream != "outbound" {
					t.Errorf("message stream mismatch: got %q, want %q", receivedMsg.MessageStream, "outbound")
				}
			}
		})
	}
}

func TestPostmarkMailer_SendOrgInvite(t *testing.T) {
	tests := []struct {
		name           string
		toEmail        string
		orgName        string
		code           string
		expiresAt      time.Time
		inviter        string
		serverResponse int
		serverBody     string
		expectErr      bool
	}{
		{
			name:           "successful send with inviter",
			toEmail:        "user@example.com",
			orgName:        "test-org",
			code:           "ABC123",
			expiresAt:      time.Now().Add(24 * time.Hour),
			inviter:        "Jane Doe",
			serverResponse: http.StatusOK,
			serverBody:     `{"MessageID":"test-message-id"}`,
			expectErr:      false,
		},
		{
			name:           "successful send without inviter",
			toEmail:        "user@example.com",
			orgName:        "test-org",
			code:           "ABC123",
			expiresAt:      time.Now().Add(24 * time.Hour),
			inviter:        "",
			serverResponse: http.StatusOK,
			serverBody:     `{"MessageID":"test-message-id"}`,
			expectErr:      false,
		},
		{
			name:           "postmark error",
			toEmail:        "invalid",
			orgName:        "test-org",
			code:           "ABC123",
			expiresAt:      time.Now().Add(24 * time.Hour),
			inviter:        "Jane Doe",
			serverResponse: http.StatusUnprocessableEntity,
			serverBody:     `{"ErrorCode":406,"Message":"Invalid email address"}`,
			expectErr:      true,
		},
		{
			name:           "whitespace in inviter name",
			toEmail:        "user@example.com",
			orgName:        "test-org",
			code:           "ABC123",
			expiresAt:      time.Now().Add(24 * time.Hour),
			inviter:        "  Jane Doe  ",
			serverResponse: http.StatusOK,
			serverBody:     `{"MessageID":"test-message-id"}`,
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedMsg postmarkMessage

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}

				if err := json.Unmarshal(body, &receivedMsg); err != nil {
					t.Fatalf("failed to unmarshal request body: %v", err)
				}

				w.WriteHeader(tt.serverResponse)
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			mailer := &postmarkMailer{
				client:  server.Client(),
				token:   "test-token",
				from:    "noreply@example.com",
				baseURL: server.URL,
			}

			ctx := context.Background()
			err := mailer.SendOrgInvite(ctx, tt.toEmail, tt.orgName, tt.code, tt.expiresAt, tt.inviter)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Verify the message structure
				expectedSubject := "Join " + strings.TrimSpace(tt.orgName) + " on Rocketship"
				if receivedMsg.Subject != expectedSubject {
					t.Errorf("subject mismatch: got %q, want %q", receivedMsg.Subject, expectedSubject)
				}

				if !strings.Contains(receivedMsg.TextBody, strings.TrimSpace(tt.code)) {
					t.Errorf("code not found in body: %q", receivedMsg.TextBody)
				}

				trimmedInviter := strings.TrimSpace(tt.inviter)
				if trimmedInviter != "" {
					if !strings.Contains(receivedMsg.TextBody, trimmedInviter) {
						t.Errorf("inviter name not found in body: %q", receivedMsg.TextBody)
					}
					if !strings.Contains(receivedMsg.TextBody, "invited you to join") {
						t.Errorf("expected 'invited you to join' in body: %q", receivedMsg.TextBody)
					}
				} else {
					if !strings.Contains(receivedMsg.TextBody, "You were invited to join") {
						t.Errorf("expected 'You were invited to join' in body: %q", receivedMsg.TextBody)
					}
				}
			}
		})
	}
}

func TestPostmarkMailer_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"MessageID":"test"}`))
	}))
	defer server.Close()

	mailer := &postmarkMailer{
		client:  server.Client(),
		token:   "test-token",
		from:    "noreply@example.com",
		baseURL: server.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := mailer.SendOrgVerification(ctx, "user@example.com", "test-org", "123456", time.Now().Add(15*time.Minute))
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
