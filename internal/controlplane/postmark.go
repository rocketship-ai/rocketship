package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProjectInviteProject is a lightweight struct for project+role in email context
type ProjectInviteProject struct {
	ProjectName string
	Role        string
}

type mailer interface {
	SendOrgVerification(ctx context.Context, toEmail, orgName, code string, expiresAt time.Time) error
	SendOrgInvite(ctx context.Context, toEmail, orgName, code string, expiresAt time.Time, inviter string) error
	SendProjectInvite(ctx context.Context, toEmail, orgName string, projects []ProjectInviteProject, code string, expiresAt time.Time, inviter, acceptURL string) error
}

type postmarkMailer struct {
	client  *http.Client
	token   string
	from    string
	baseURL string
}

const defaultPostmarkURL = "https://api.postmarkapp.com/email"

func newPostmarkMailer(cfg EmailConfig) (*postmarkMailer, error) {
	if strings.TrimSpace(cfg.PostmarkToken) == "" {
		return nil, fmt.Errorf("postmark token missing")
	}
	if strings.TrimSpace(cfg.FromAddress) == "" {
		return nil, fmt.Errorf("from address missing")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return &postmarkMailer{
		client:  client,
		token:   cfg.PostmarkToken,
		from:    cfg.FromAddress,
		baseURL: defaultPostmarkURL,
	}, nil
}

type postmarkMessage struct {
	From          string `json:"From"`
	To            string `json:"To"`
	Subject       string `json:"Subject"`
	TextBody      string `json:"TextBody"`
	MessageStream string `json:"MessageStream,omitempty"`
}

func (m *postmarkMailer) SendOrgVerification(ctx context.Context, toEmail, orgName, code string, expiresAt time.Time) error {
	duration := time.Until(expiresAt).Round(time.Minute)
	if duration < 0 {
		duration = 0
	}

	subject := "Confirm your Rocketship organization"
	text := fmt.Sprintf("Use this code to confirm \"%s\": %s. It expires in %s.", strings.TrimSpace(orgName), strings.TrimSpace(code), duration)
	return m.send(ctx, toEmail, subject, text)
}

func (m *postmarkMailer) SendOrgInvite(ctx context.Context, toEmail, orgName, code string, expiresAt time.Time, inviter string) error {
	duration := time.Until(expiresAt).Round(time.Minute)
	if duration < 0 {
		duration = 0
	}

	subject := fmt.Sprintf("Join %s on Rocketship", strings.TrimSpace(orgName))
	var builder strings.Builder
	if inviter = strings.TrimSpace(inviter); inviter != "" {
		builder.WriteString(inviter)
		builder.WriteString(" invited you to join ")
	} else {
		builder.WriteString("You were invited to join ")
	}
	builder.WriteString(strings.TrimSpace(orgName))
	builder.WriteString(" on Rocketship. Enter code ")
	builder.WriteString(strings.TrimSpace(code))
	builder.WriteString(" within ")
	builder.WriteString(duration.String())
	builder.WriteString(" to accept.")

	return m.send(ctx, toEmail, subject, builder.String())
}

func (m *postmarkMailer) SendProjectInvite(ctx context.Context, toEmail, orgName string, projects []ProjectInviteProject, code string, expiresAt time.Time, inviter, acceptURL string) error {
	duration := time.Until(expiresAt).Round(time.Minute)
	if duration < 0 {
		duration = 0
	}

	subject := "You've been invited to Rocketship projects"
	var builder strings.Builder

	if inviter = strings.TrimSpace(inviter); inviter != "" {
		builder.WriteString(inviter)
		builder.WriteString(" invited you to access projects in ")
	} else {
		builder.WriteString("You've been invited to access projects in ")
	}
	builder.WriteString(strings.TrimSpace(orgName))
	builder.WriteString(" on Rocketship.\n\n")

	builder.WriteString("Projects:\n")
	for _, p := range projects {
		builder.WriteString("  - ")
		builder.WriteString(p.ProjectName)
		builder.WriteString(" (")
		builder.WriteString(p.Role)
		builder.WriteString(")\n")
	}
	builder.WriteString("\n")

	// Include the clickable accept link
	if acceptURL = strings.TrimSpace(acceptURL); acceptURL != "" {
		builder.WriteString("Click here to accept: ")
		builder.WriteString(acceptURL)
		builder.WriteString("\n\n")
		builder.WriteString("Or enter code ")
	} else {
		builder.WriteString("Enter code ")
	}
	builder.WriteString(strings.TrimSpace(code))
	builder.WriteString(" within ")
	builder.WriteString(duration.String())
	builder.WriteString(" to accept.\n\n")

	builder.WriteString("If you don't have an account, sign in with GitHub first, then accept the invite.")

	return m.send(ctx, toEmail, subject, builder.String())
}

func (m *postmarkMailer) send(ctx context.Context, toEmail, subject, body string) error {
	msg := postmarkMessage{
		From:          m.from,
		To:            strings.TrimSpace(toEmail),
		Subject:       strings.TrimSpace(subject),
		TextBody:      strings.TrimSpace(body),
		MessageStream: "outbound",
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal postmark payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create postmark request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", m.token)

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("postmark request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		snippet := strings.TrimSpace(string(bodyBytes))
		if snippet == "" {
			snippet = resp.Status
		}
		return fmt.Errorf("postmark error: %s", snippet)
	}

	return nil
}
