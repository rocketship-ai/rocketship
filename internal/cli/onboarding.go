package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/cli/auth"
	"github.com/rocketship-ai/rocketship/internal/cli/oidc"
)

type brokerClient struct {
	baseURL    string
	profile    string
	manager    *auth.Manager
	httpClient *http.Client
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("broker request failed (%d): %s", e.Status, e.Message)
}

type registrationInfo struct {
	ID                string
	OrgName           string
	Email             string
	ExpiresAt         time.Time
	ResendAvailableAt time.Time
	Attempts          int
	MaxAttempts       int
}

type inviteInfo struct {
	InviteID         string
	OrganizationID   string
	OrganizationName string
	Role             string
	ExpiresAt        time.Time
}

type onboardingState struct {
	Status              string
	Roles               []string
	PendingRegistration *registrationInfo
	PendingInvites      []inviteInfo
}

type completionResult struct {
	NeedsRefresh bool
	Message      string
}

func maybeRunOnboarding(ctx context.Context, profileName string, token auth.TokenData, manager *auth.Manager) error {
	if manager == nil {
		return errors.New("token manager required")
	}

	if !isInteractive() {
		return nil
	}

	base := strings.TrimSpace(token.Issuer)
	if base == "" {
		return nil
	}

	client := &brokerClient{
		baseURL:    strings.TrimRight(base, "/"),
		profile:    profileName,
		manager:    manager,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}

	flow := onboardingFlow{
		client: client,
		reader: bufio.NewReader(os.Stdin),
		stdout: os.Stdout,
	}
	return flow.run(ctx)
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func (c *brokerClient) currentUser(ctx context.Context) (onboardingState, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/users/me", nil)
	if err != nil {
		return onboardingState{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return onboardingState{}, c.decodeError(resp)
	}

	var payload struct {
		Status              string   `json:"status"`
		Roles               []string `json:"roles"`
		PendingRegistration *struct {
			RegistrationID    string `json:"registration_id"`
			OrgName           string `json:"org_name"`
			Email             string `json:"email"`
			ExpiresAt         string `json:"expires_at"`
			ResendAvailableAt string `json:"resend_available_at"`
			Attempts          int    `json:"attempts"`
			MaxAttempts       int    `json:"max_attempts"`
		} `json:"pending_registration"`
		PendingInvites []struct {
			InviteID         string `json:"invite_id"`
			OrganizationID   string `json:"organization_id"`
			OrganizationName string `json:"organization_name"`
			Role             string `json:"role"`
			ExpiresAt        string `json:"expires_at"`
		} `json:"pending_invites"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return onboardingState{}, fmt.Errorf("failed to decode broker response: %w", err)
	}

	state := onboardingState{
		Status: payload.Status,
		Roles:  append([]string(nil), payload.Roles...),
	}

	if payload.PendingRegistration != nil {
		expiry, _ := time.Parse(time.RFC3339, payload.PendingRegistration.ExpiresAt)
		resend, _ := time.Parse(time.RFC3339, payload.PendingRegistration.ResendAvailableAt)
		state.PendingRegistration = &registrationInfo{
			ID:                payload.PendingRegistration.RegistrationID,
			OrgName:           payload.PendingRegistration.OrgName,
			Email:             payload.PendingRegistration.Email,
			ExpiresAt:         expiry,
			ResendAvailableAt: resend,
			Attempts:          payload.PendingRegistration.Attempts,
			MaxAttempts:       payload.PendingRegistration.MaxAttempts,
		}
	}

	for _, raw := range payload.PendingInvites {
		expiry, _ := time.Parse(time.RFC3339, raw.ExpiresAt)
		state.PendingInvites = append(state.PendingInvites, inviteInfo{
			InviteID:         raw.InviteID,
			OrganizationID:   raw.OrganizationID,
			OrganizationName: raw.OrganizationName,
			Role:             raw.Role,
			ExpiresAt:        expiry,
		})
	}

	return state, nil
}

func (c *brokerClient) startRegistration(ctx context.Context, orgName, email string) (registrationInfo, error) {
	body := map[string]string{"name": orgName, "email": email}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/orgs/registration/start", body)
	if err != nil {
		return registrationInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return registrationInfo{}, c.decodeError(resp)
	}

	var payload struct {
		RegistrationID    string `json:"registration_id"`
		OrgName           string `json:"org_name"`
		Email             string `json:"email"`
		ExpiresAt         string `json:"expires_at"`
		ResendAvailableAt string `json:"resend_available_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return registrationInfo{}, fmt.Errorf("failed to decode registration response: %w", err)
	}

	expiry, _ := time.Parse(time.RFC3339, payload.ExpiresAt)
	resend, _ := time.Parse(time.RFC3339, payload.ResendAvailableAt)

	return registrationInfo{
		ID:                payload.RegistrationID,
		OrgName:           payload.OrgName,
		Email:             payload.Email,
		ExpiresAt:         expiry,
		ResendAvailableAt: resend,
		Attempts:          0,
		MaxAttempts:       0,
	}, nil
}

func (c *brokerClient) resendRegistration(ctx context.Context, registrationID string) (registrationInfo, error) {
	body := map[string]string{"registration_id": registrationID}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/orgs/registration/resend", body)
	if err != nil {
		return registrationInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return registrationInfo{}, c.decodeError(resp)
	}

	var payload struct {
		RegistrationID    string `json:"registration_id"`
		OrgName           string `json:"org_name"`
		Email             string `json:"email"`
		ExpiresAt         string `json:"expires_at"`
		ResendAvailableAt string `json:"resend_available_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return registrationInfo{}, fmt.Errorf("failed to decode resend response: %w", err)
	}

	expiry, _ := time.Parse(time.RFC3339, payload.ExpiresAt)
	resend, _ := time.Parse(time.RFC3339, payload.ResendAvailableAt)

	return registrationInfo{
		ID:                payload.RegistrationID,
		OrgName:           payload.OrgName,
		Email:             payload.Email,
		ExpiresAt:         expiry,
		ResendAvailableAt: resend,
	}, nil
}

func (c *brokerClient) completeRegistration(ctx context.Context, registrationID, code string) (completionResult, error) {
	body := map[string]string{
		"registration_id": registrationID,
		"code":            code,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/orgs/registration/complete", body)
	if err != nil {
		return completionResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return completionResult{}, c.decodeError(resp)
	}

	var payload struct {
		NeedsRefresh bool `json:"needs_claim_refresh"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return completionResult{}, fmt.Errorf("failed to decode completion response: %w", err)
	}

	return completionResult{NeedsRefresh: payload.NeedsRefresh}, nil
}

func (c *brokerClient) acceptInvite(ctx context.Context, code string) (completionResult, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/orgs/invites/accept", map[string]string{"code": code})
	if err != nil {
		return completionResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return completionResult{}, c.decodeError(resp)
	}

	var payload struct {
		NeedsRefresh bool `json:"needs_claim_refresh"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return completionResult{}, fmt.Errorf("failed to decode invite response: %w", err)
	}

	return completionResult{NeedsRefresh: payload.NeedsRefresh}, nil
}

func (c *brokerClient) refreshTokens(ctx context.Context) error {
	data, err := c.manager.Load(c.profile)
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	refreshed, err := oidc.RefreshAccessToken(refreshCtx, data)
	if err != nil {
		return fmt.Errorf("failed to refresh tokens: %w", err)
	}

	if err := c.manager.Save(c.profile, refreshed); err != nil {
		return fmt.Errorf("failed to persist refreshed tokens: %w", err)
	}
	return nil
}

func (c *brokerClient) doRequest(ctx context.Context, method, path string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	token, err := c.manager.AccessToken(c.profile, func(data auth.TokenData) (auth.TokenData, error) {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return oidc.RefreshAccessToken(refreshCtx, data)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("broker request failed: %w", err)
	}
	return resp, nil
}

func (c *brokerClient) decodeError(resp *http.Response) error {
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()

	var payload map[string]string
	if err := json.Unmarshal(buf, &payload); err == nil {
		if msg := strings.TrimSpace(payload["error"]); msg != "" {
			return &apiError{Status: resp.StatusCode, Message: msg}
		}
	}

	message := strings.TrimSpace(string(buf))
	if message == "" {
		message = resp.Status
	}
	return &apiError{Status: resp.StatusCode, Message: message}
}

type onboardingFlow struct {
	client *brokerClient
	reader *bufio.Reader
	stdout io.Writer
}

func (f *onboardingFlow) run(ctx context.Context) error {
	state, err := f.client.currentUser(ctx)
	if err != nil {
		return err
	}

	if state.Status != "pending" && state.PendingRegistration == nil && len(state.PendingInvites) == 0 {
		return nil
	}

	_, _ = fmt.Fprintln(f.stdout, "\nYou are not part of a Rocketship organization yet.")

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if state.PendingRegistration != nil {
			if err := f.resumeRegistration(ctx, state.PendingRegistration); err != nil {
				return err
			}
			state.PendingRegistration = nil
			refreshed, err := f.client.currentUser(ctx)
			if err != nil {
				return err
			}
			state = refreshed
			if state.Status != "pending" {
				_, _ = fmt.Fprintln(f.stdout, "Onboarding complete.")
				return nil
			}
			continue
		}

		_, _ = fmt.Fprintln(f.stdout, "\nSelect an option:")
		_, _ = fmt.Fprintln(f.stdout, "  1) Create a new organization")
		_, _ = fmt.Fprintln(f.stdout, "  2) Join with an invite code")
		_, _ = fmt.Fprintln(f.stdout, "  3) Finish later")

		choice, err := f.prompt("Enter choice [1-3]: ")
		if err != nil {
			return err
		}

		switch strings.TrimSpace(choice) {
		case "1":
			if err := f.createOrganizationFlow(ctx); err != nil {
				return err
			}
		case "2":
			if err := f.joinWithInviteFlow(ctx); err != nil {
				return err
			}
		case "3":
			_, _ = fmt.Fprintln(f.stdout, "Skipping onboarding for now. Run `rocketship login` again when ready.")
			return nil
		default:
			_, _ = fmt.Fprintln(f.stdout, "Please enter 1, 2, or 3.")
			continue
		}

		refreshed, err := f.client.currentUser(ctx)
		if err != nil {
			return err
		}
		state = refreshed
		if state.Status != "pending" && state.PendingRegistration == nil {
			_, _ = fmt.Fprintln(f.stdout, "Onboarding complete.")
			return nil
		}
	}
}

func (f *onboardingFlow) resumeRegistration(ctx context.Context, reg *registrationInfo) error {
	_, _ = fmt.Fprintf(f.stdout, "\nA pending organization registration exists for %q.\n", reg.OrgName)
	_, _ = fmt.Fprintf(f.stdout, "Check %s for the verification code.\n", reg.Email)
	_, _ = fmt.Fprintln(f.stdout, "Enter the code below (or type 'resend' to request a new one).")

	for {
		input, err := f.prompt("Verification code: ")
		if err != nil {
			return err
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.EqualFold(input, "resend") {
			updated, err := f.client.resendRegistration(ctx, reg.ID)
			if err != nil {
				f.reportError(err)
				continue
			}
			*reg = updated
			_, _ = fmt.Fprintf(f.stdout, "Sent a new code. It expires at %s.\n", updated.ExpiresAt.Format(time.RFC1123))
			continue
		}

		result, err := f.client.completeRegistration(ctx, reg.ID, input)
		if err != nil {
			var apiErr *apiError
			if errors.As(err, &apiErr) {
				f.reportError(apiErr)
				continue
			}
			return err
		}

		if result.NeedsRefresh {
			if err := f.client.refreshTokens(ctx); err != nil {
				return err
			}
		}

		_, _ = fmt.Fprintf(f.stdout, "Organization %q created successfully.\n", reg.OrgName)
		return nil
	}
}

func (f *onboardingFlow) createOrganizationFlow(ctx context.Context) error {
	name, err := f.prompt("Organization name: ")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		_, _ = fmt.Fprintln(f.stdout, "Organization name cannot be empty.")
		return nil
	}

	email, err := f.prompt("Email address for verification: ")
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" || !strings.Contains(email, "@") {
		_, _ = fmt.Fprintln(f.stdout, "Valid email address required.")
		return nil
	}

	reg, err := f.client.startRegistration(ctx, name, email)
	if err != nil {
		f.reportError(err)
		return nil
	}

	_, _ = fmt.Fprintf(f.stdout, "We emailed a verification code to %s. It expires at %s.\n", reg.Email, reg.ExpiresAt.Format(time.RFC1123))
	return f.resumeRegistration(ctx, &reg)
}

func (f *onboardingFlow) joinWithInviteFlow(ctx context.Context) error {
	_, _ = fmt.Fprintln(f.stdout, "Enter the invite code from your email. Type 'back' to return to the menu.")
	for {
		code, err := f.prompt("Invite code: ")
		if err != nil {
			return err
		}
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if strings.EqualFold(code, "back") {
			return nil
		}

		result, err := f.client.acceptInvite(ctx, code)
		if err != nil {
			f.reportError(err)
			continue
		}

		if result.NeedsRefresh {
			if err := f.client.refreshTokens(ctx); err != nil {
				return err
			}
		}

		_, _ = fmt.Fprintln(f.stdout, "Invite accepted. Welcome aboard!")
		return nil
	}
}

func (f *onboardingFlow) prompt(prompt string) (string, error) {
	_, _ = fmt.Fprint(f.stdout, prompt)
	text, err := f.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func (f *onboardingFlow) reportError(err error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		_, _ = fmt.Fprintf(f.stdout, "Error: %s (status %d)\n", apiErr.Message, apiErr.Status)
	} else {
		_, _ = fmt.Fprintf(f.stdout, "Error: %v\n", err)
	}
}
