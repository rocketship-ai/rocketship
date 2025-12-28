package controlplane

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

type stubMailer struct {
	verifications []struct{ email, org, code string }
	invites       []struct{ email, org, code string }
}

func (m *stubMailer) SendOrgVerification(_ context.Context, email, org, code string, _ time.Time) error {
	m.verifications = append(m.verifications, struct{ email, org, code string }{email: email, org: org, code: code})
	return nil
}

func (m *stubMailer) SendOrgInvite(_ context.Context, email, org, code string, _ time.Time, _ string) error {
	m.invites = append(m.invites, struct{ email, org, code string }{email: email, org: org, code: code})
	return nil
}

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

func (f *fakeGitHub) ExchangeAuthorizationCode(_ context.Context, _, _, _ string) (TokenResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenCalls++
	return f.tokenResp, nil
}

func (f *fakeGitHub) FetchUser(_ context.Context, _ string) (GitHubUser, error) {
	return f.user, nil
}

type fakeStore struct {
	mu             sync.Mutex
	user           persistence.User
	summary        persistence.RoleSummary
	refresh        map[string]persistence.RefreshTokenRecord
	projectOrg     map[uuid.UUID]uuid.UUID
	members        map[uuid.UUID][]persistence.ProjectMember
	primaryOrg     uuid.UUID
	primaryProject uuid.UUID
	registrations  map[uuid.UUID]persistence.OrganizationRegistration
	invites        map[uuid.UUID]persistence.OrganizationInvite
	slugMap        map[string]uuid.UUID
}

func newFakeStore() *fakeStore {
	orgID := uuid.New()
	projectID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	user := persistence.User{
		ID:           userID,
		GitHubUserID: 1234,
		Email:        "owner@example.com",
		Name:         "Owner",
		Username:     "owner",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	members := []persistence.ProjectMember{
		{
			UserID:    userID,
			Email:     user.Email,
			Name:      user.Name,
			Username:  user.Username,
			Role:      "write",
			JoinedAt:  now,
			UpdatedAt: now,
		},
	}

	return &fakeStore{
		user: user,
		summary: persistence.RoleSummary{
			Organizations: []persistence.OrganizationMembership{{OrganizationID: orgID, IsAdmin: true}},
		},
		refresh:        make(map[string]persistence.RefreshTokenRecord),
		projectOrg:     map[uuid.UUID]uuid.UUID{projectID: orgID},
		members:        map[uuid.UUID][]persistence.ProjectMember{projectID: members},
		primaryOrg:     orgID,
		primaryProject: projectID,
		registrations:  make(map[uuid.UUID]persistence.OrganizationRegistration),
		invites:        make(map[uuid.UUID]persistence.OrganizationInvite),
		slugMap:        map[string]uuid.UUID{"default": orgID},
	}
}

func (f *fakeStore) UpsertGitHubUser(_ context.Context, input persistence.GitHubUserInput) (persistence.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.user.ID == uuid.Nil {
		f.user = persistence.User{
			ID:           uuid.New(),
			GitHubUserID: input.GitHubUserID,
			Email:        input.Email,
			Name:         input.Name,
			Username:     input.Username,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
	} else {
		f.user.GitHubUserID = input.GitHubUserID
		f.user.Email = input.Email
		f.user.Name = input.Name
		f.user.Username = input.Username
		f.user.UpdatedAt = time.Now()
	}
	return f.user, nil
}

func (f *fakeStore) UpdateUserEmail(_ context.Context, userID uuid.UUID, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.user.ID != userID {
		return sql.ErrNoRows
	}
	f.user.Email = strings.ToLower(strings.TrimSpace(email))
	f.user.UpdatedAt = time.Now()
	return nil
}

func (f *fakeStore) RoleSummary(context.Context, uuid.UUID) (persistence.RoleSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.summary, nil
}

func (f *fakeStore) SaveRefreshToken(_ context.Context, token string, rec persistence.RefreshTokenRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refresh == nil {
		f.refresh = make(map[string]persistence.RefreshTokenRecord)
	}
	recordCopy := rec
	recordCopy.Scopes = append([]string(nil), rec.Scopes...)
	f.refresh[token] = recordCopy
	return nil
}

func (f *fakeStore) GetRefreshToken(_ context.Context, token string) (persistence.RefreshTokenRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.refresh[token]
	if !ok {
		return persistence.RefreshTokenRecord{}, persistence.ErrRefreshTokenNotFound
	}
	return rec, nil
}

func (f *fakeStore) DeleteRefreshToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.refresh[token]; !ok {
		return persistence.ErrRefreshTokenNotFound
	}
	delete(f.refresh, token)
	return nil
}

func (f *fakeStore) DeleteOrgRegistrationsForUser(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, reg := range f.registrations {
		if reg.UserID == userID {
			delete(f.registrations, id)
		}
	}
	return nil
}

func (f *fakeStore) CreateOrgRegistration(_ context.Context, rec persistence.OrganizationRegistration) (persistence.OrganizationRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	rec.CreatedAt = time.Now()
	rec.UpdatedAt = rec.CreatedAt
	f.registrations[rec.ID] = rec
	return rec, nil
}

func (f *fakeStore) GetOrgRegistration(_ context.Context, id uuid.UUID) (persistence.OrganizationRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.registrations[id]
	if !ok {
		return persistence.OrganizationRegistration{}, sql.ErrNoRows
	}
	return rec, nil
}

func (f *fakeStore) LatestOrgRegistrationForUser(_ context.Context, userID uuid.UUID) (persistence.OrganizationRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest persistence.OrganizationRegistration
	found := false
	for _, rec := range f.registrations {
		if rec.UserID != userID {
			continue
		}
		if !found || rec.CreatedAt.After(latest.CreatedAt) {
			latest = rec
			found = true
		}
	}
	if !found {
		return persistence.OrganizationRegistration{}, sql.ErrNoRows
	}
	return latest, nil
}

func (f *fakeStore) UpdateOrgRegistrationForResend(_ context.Context, id uuid.UUID, hash, salt []byte, expiresAt, resendAt time.Time) (persistence.OrganizationRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.registrations[id]
	if !ok {
		return persistence.OrganizationRegistration{}, sql.ErrNoRows
	}
	rec.CodeHash = append([]byte(nil), hash...)
	rec.CodeSalt = append([]byte(nil), salt...)
	rec.ExpiresAt = expiresAt
	rec.ResendAvailableAt = resendAt
	rec.UpdatedAt = time.Now()
	f.registrations[id] = rec
	return rec, nil
}

func (f *fakeStore) IncrementOrgRegistrationAttempts(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.registrations[id]
	if !ok {
		return sql.ErrNoRows
	}
	rec.Attempts++
	rec.UpdatedAt = time.Now()
	f.registrations[id] = rec
	return nil
}

func (f *fakeStore) DeleteOrgRegistration(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.registrations, id)
	return nil
}

func (f *fakeStore) CreateOrganization(_ context.Context, userID uuid.UUID, name, slug string) (persistence.Organization, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	norm := strings.ToLower(strings.TrimSpace(slug))
	if norm == "" {
		norm = "org"
	}
	if _, exists := f.slugMap[norm]; exists {
		return persistence.Organization{}, persistence.ErrOrganizationSlugUsed
	}

	id := uuid.New()
	org := persistence.Organization{
		ID:        id,
		Name:      name,
		Slug:      norm,
		CreatedAt: time.Now(),
	}
	f.slugMap[norm] = id
	f.summary.Organizations = append(f.summary.Organizations, persistence.OrganizationMembership{
		OrganizationID: id,
		IsAdmin:        true,
	})
	f.primaryOrg = id
	return org, nil
}

func (f *fakeStore) OrganizationSlugExists(_ context.Context, slug string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, exists := f.slugMap[strings.ToLower(strings.TrimSpace(slug))]
	return exists, nil
}

func (f *fakeStore) AddOrganizationAdmin(_ context.Context, orgID, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, org := range f.summary.Organizations {
		if org.OrganizationID == orgID {
			return nil
		}
	}
	f.summary.Organizations = append(f.summary.Organizations, persistence.OrganizationMembership{
		OrganizationID: orgID,
		IsAdmin:        true,
	})
	f.primaryOrg = orgID
	return nil
}

func (f *fakeStore) CreateOrgInvite(_ context.Context, invite persistence.OrganizationInvite) (persistence.OrganizationInvite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if invite.ID == uuid.Nil {
		invite.ID = uuid.New()
	}
	if invite.OrganizationName == "" {
		invite.OrganizationName = "invite-org"
	}
	invite.CreatedAt = time.Now()
	invite.UpdatedAt = invite.CreatedAt
	f.invites[invite.ID] = invite
	return invite, nil
}

func (f *fakeStore) FindPendingOrgInvites(_ context.Context, email string) ([]persistence.OrganizationInvite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	email = strings.ToLower(strings.TrimSpace(email))
	var out []persistence.OrganizationInvite
	now := time.Now()
	for _, inv := range f.invites {
		if strings.ToLower(inv.Email) != email {
			continue
		}
		if inv.AcceptedAt.Valid {
			continue
		}
		if inv.ExpiresAt.Before(now) {
			continue
		}
		out = append(out, inv)
	}
	return out, nil
}

func (f *fakeStore) MarkOrgInviteAccepted(_ context.Context, inviteID, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inv, ok := f.invites[inviteID]
	if !ok {
		return sql.ErrNoRows
	}
	inv.AcceptedAt = sql.NullTime{Valid: true, Time: time.Now()}
	inv.AcceptedBy = uuid.NullUUID{Valid: true, UUID: userID}
	inv.UpdatedAt = time.Now()
	f.invites[inviteID] = inv
	return nil
}

func (f *fakeStore) CreateOrganizationWithProject(context.Context, uuid.UUID, persistence.CreateOrgInput) (persistence.Organization, persistence.Project, error) {
	return persistence.Organization{}, persistence.Project{}, nil
}

// GitHub App installation methods
func (f *fakeStore) UpsertGitHubAppInstallation(_ context.Context, _ uuid.UUID, _ int64, _ uuid.UUID, _, _ string) error {
	return nil
}

func (f *fakeStore) GetGitHubAppInstallation(_ context.Context, _ uuid.UUID) (int64, string, string, error) {
	return 0, "", "", persistence.ErrGitHubAppNotInstalled
}

// Project methods
func (f *fakeStore) CreateProject(_ context.Context, project persistence.Project) (persistence.Project, error) {
	return project, nil
}

func (f *fakeStore) GetProject(_ context.Context, _ uuid.UUID) (persistence.Project, error) {
	return persistence.Project{}, sql.ErrNoRows
}

func (f *fakeStore) ListProjects(_ context.Context, _ uuid.UUID) ([]persistence.Project, error) {
	return nil, nil
}

func (f *fakeStore) ProjectNameExists(_ context.Context, _ uuid.UUID, _, _ string) (bool, error) {
	return false, nil
}

// Environment methods
func (f *fakeStore) CreateEnvironment(_ context.Context, env persistence.ProjectEnvironment) (persistence.ProjectEnvironment, error) {
	return env, nil
}

func (f *fakeStore) ListEnvironments(_ context.Context, _ uuid.UUID) ([]persistence.ProjectEnvironment, error) {
	return nil, nil
}

// Suite methods
func (f *fakeStore) UpsertSuite(_ context.Context, suite persistence.Suite) (persistence.Suite, error) {
	return suite, nil
}

func (f *fakeStore) GetSuiteByName(_ context.Context, _ uuid.UUID, _, _ string) (persistence.Suite, bool, error) {
	return persistence.Suite{}, false, nil
}

func (f *fakeStore) ListSuites(_ context.Context, _ uuid.UUID) ([]persistence.Suite, error) {
	return nil, nil
}

func (f *fakeStore) UpsertTest(_ context.Context, test persistence.Test) (persistence.Test, error) {
	return test, nil
}

// Schedule methods
func (f *fakeStore) ListEnabledSchedulesByProject(_ context.Context, _ uuid.UUID) ([]persistence.SuiteSchedule, error) {
	return nil, nil
}

// CI token methods
func (f *fakeStore) ListActiveCITokens(_ context.Context, _ uuid.UUID) ([]persistence.CITokenRecord, error) {
	return nil, nil
}

// Count methods for overview
func (f *fakeStore) CountProjectsForOrg(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeStore) CountSuitesForOrg(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeStore) CountSuitesOnDefaultBranchForOrg(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeStore) CountEnvsWithVarsForOrg(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeStore) CountEnabledSchedulesForOrg(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeStore) CountActiveCITokensForOrg(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeStore) ProjectOrganizationID(_ context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if org, ok := f.projectOrg[projectID]; ok {
		return org, nil
	}
	return uuid.Nil, sql.ErrNoRows
}

func (f *fakeStore) IsOrganizationAdmin(_ context.Context, orgID, userID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return orgID == f.primaryOrg && f.user.ID == userID, nil
}

func (f *fakeStore) ListProjectMembers(_ context.Context, projectID uuid.UUID) ([]persistence.ProjectMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.members[projectID]
	if !ok {
		return nil, nil
	}
	out := make([]persistence.ProjectMember, len(m))
	copy(out, m)
	return out, nil
}

func (f *fakeStore) SetProjectMemberRole(_ context.Context, projectID, userID uuid.UUID, role string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	members, ok := f.members[projectID]
	if !ok {
		return sql.ErrNoRows
	}
	for i := range members {
		if members[i].UserID == userID {
			members[i].Role = role
			members[i].UpdatedAt = time.Now()
			f.members[projectID] = members
			return nil
		}
	}
	return sql.ErrNoRows
}

func (f *fakeStore) RemoveProjectMember(_ context.Context, projectID, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	members, ok := f.members[projectID]
	if !ok {
		return sql.ErrNoRows
	}
	updated := make([]persistence.ProjectMember, 0, len(members))
	found := false
	for _, m := range members {
		if m.UserID == userID {
			found = true
			continue
		}
		updated = append(updated, m)
	}
	if !found {
		return sql.ErrNoRows
	}
	f.members[projectID] = updated
	return nil
}

func (f *fakeStore) InsertWebhookDelivery(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

func (f *fakeStore) ListOrgsByInstallationID(_ context.Context, _ int64) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeStore) InsertScanAttempt(_ context.Context, _ persistence.ScanAttempt) error {
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

	fs := newFakeStore()

	mailer := &stubMailer{}

	srv, err := newServerWithComponents(cfg, signer, fake, nil, fs, mailer)
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

	sub, _ := claims["sub"].(string)
	if !strings.HasPrefix(sub, "user:") {
		t.Fatalf("unexpected subject claim: %v", sub)
	}
	rolesVal, ok := claims["roles"].([]interface{})
	if !ok || len(rolesVal) == 0 {
		t.Fatalf("roles claim missing: %v", claims["roles"])
	}
	firstRole, _ := rolesVal[0].(string)
	if strings.ToLower(firstRole) != "owner" {
		t.Fatalf("expected owner role, got %v", rolesVal)
	}
	if _, ok := claims["org_id"].(string); !ok {
		t.Fatalf("expected org_id claim, got %v", claims["org_id"])
	}

	fs.mu.Lock()
	if _, ok := fs.refresh[tokenResp.RefreshToken]; !ok {
		fs.mu.Unlock()
		t.Fatalf("refresh token not stored in fake store")
	}
	fs.mu.Unlock()

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

	fs.mu.Lock()
	if _, ok := fs.refresh[tokenResp.RefreshToken]; ok {
		fs.mu.Unlock()
		t.Fatalf("old refresh token still persisted")
	}
	if _, ok := fs.refresh[refreshResp.RefreshToken]; !ok {
		fs.mu.Unlock()
		t.Fatalf("new refresh token not persisted")
	}
	fs.mu.Unlock()
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
	srv, err := newServerWithComponents(cfg, signer, fake, nil, newFakeStore(), &stubMailer{})
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
	srv, err := newServerWithComponents(cfg, signer, fake, nil, newFakeStore(), &stubMailer{})
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

func TestOrgRegistrationLifecycle(t *testing.T) {
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
		GitHub:          GitHubConfig{ClientID: "gh", ClientSecret: "secret"},
	}

	store := newFakeStore()
	store.summary = persistence.RoleSummary{Organizations: nil} // start pending
	store.user.Email = "pending@example.com"
	mail := &stubMailer{}

	srv, err := newServerWithComponents(cfg, signer, &fakeGitHub{}, nil, store, mail)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	principal := brokerPrincipal{
		UserID: store.user.ID,
		Email:  store.user.Email,
		Roles:  []string{"pending"},
	}

	body := strings.NewReader(`{"name":"Test Org"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orgs/registration/start", body)
	rec := httptest.NewRecorder()
	srv.handleOrgRegistrationStart(rec, req, principal)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected created, got %d", rec.Code)
	}
	if len(mail.verifications) != 1 {
		t.Fatalf("expected verification email to be sent")
	}

	var startResp struct {
		RegistrationID string `json:"registration_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &startResp); err != nil {
		t.Fatalf("failed to decode start response: %v", err)
	}

	// wrong code should fail
	completeReq := httptest.NewRequest(http.MethodPost, "/api/orgs/registration/complete", strings.NewReader(fmt.Sprintf(`{"registration_id":"%s","code":"000000"}`, startResp.RegistrationID)))
	completeRec := httptest.NewRecorder()
	srv.handleOrgRegistrationComplete(completeRec, completeReq, principal)
	if completeRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for wrong code, got %d", completeRec.Code)
	}

	// correct code
	correct := mail.verifications[0].code
	completeReq = httptest.NewRequest(http.MethodPost, "/api/orgs/registration/complete", strings.NewReader(fmt.Sprintf(`{"registration_id":"%s","code":"%s"}`, startResp.RegistrationID, correct)))
	completeRec = httptest.NewRecorder()
	srv.handleOrgRegistrationComplete(completeRec, completeReq, principal)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected success completing registration, got %d", completeRec.Code)
	}

	var completeResp struct {
		NeedsRefresh bool `json:"needs_claim_refresh"`
	}
	if err := json.Unmarshal(completeRec.Body.Bytes(), &completeResp); err != nil {
		t.Fatalf("failed to decode complete response: %v", err)
	}
	if !completeResp.NeedsRefresh {
		t.Fatalf("expected refresh flag")
	}
}

func TestProjectMembersScopedToOrganization(t *testing.T) {
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
		GitHub:          GitHubConfig{ClientID: "gh", ClientSecret: "secret"},
	}

	store := newFakeStore()

	srv, err := newServerWithComponents(cfg, signer, &fakeGitHub{}, nil, store, &stubMailer{})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	principal := brokerPrincipal{
		UserID: store.user.ID,
		Roles:  []string{"owner"},
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%s/members", store.primaryProject), nil)
	rec := httptest.NewRecorder()
	srv.handleProjectRoutes(rec, req, principal)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected success listing members, got %d", rec.Code)
	}

	store.mu.Lock()
	otherOrg := uuid.New()
	otherProject := uuid.New()
	store.projectOrg[otherProject] = otherOrg
	store.members[otherProject] = nil
	store.mu.Unlock()

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%s/members", otherProject), nil)
	rec = httptest.NewRecorder()
	srv.handleProjectRoutes(rec, req, principal)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for cross-org access, got %d", rec.Code)
	}
}
