package controlplane

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// handleGitHubAppWebhook handles incoming GitHub App webhook events.
// It verifies the signature, logs the event, persists an audit record,
// and triggers repository scans for relevant events.
func (s *Server) handleGitHubAppWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if webhook secret is configured
	if s.cfg.GitHubWebhookSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "webhook secret not configured")
		return
	}

	// Require X-GitHub-Event header
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		writeError(w, http.StatusBadRequest, "missing X-GitHub-Event header")
		return
	}

	// Require X-GitHub-Delivery header
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	if deliveryID == "" {
		writeError(w, http.StatusBadRequest, "missing X-GitHub-Delivery header")
		return
	}

	// Read the raw body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !verifyWebhookSignature(body, signature, s.cfg.GitHubWebhookSecret) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	// Parse the payload to extract relevant fields
	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("webhook: failed to parse payload", "delivery", deliveryID, "event", event, "error", err)
		// Still acknowledge the webhook even if we can't parse the payload
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	// Extract fields based on event type
	var repoFullName, ref, action string
	if payload.Repository != nil {
		repoFullName = payload.Repository.FullName
	}
	if event == "push" {
		ref = payload.Ref
	}
	if event == "pull_request" {
		action = payload.Action
	}

	// Log the webhook receipt
	slog.Info("webhook received",
		"event", event,
		"delivery", deliveryID,
		"repo", repoFullName,
		"ref", ref,
		"action", action,
		"installation_id", payload.Installation.ID,
	)

	// Persist the delivery for audit
	if err := s.store.InsertWebhookDelivery(r.Context(), deliveryID, event, repoFullName, ref, action); err != nil {
		slog.Error("webhook: failed to persist delivery", "delivery", deliveryID, "error", err)
		// Don't fail the webhook - GitHub would retry
	}

	// Process scannable events
	s.processWebhookForScanning(r.Context(), event, deliveryID, &payload)

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// processWebhookForScanning checks if a webhook event should trigger a repository scan
func (s *Server) processWebhookForScanning(ctx context.Context, event, deliveryID string, payload *webhookPayload) {
	// Check if GitHub App is configured
	if s.githubApp == nil || !s.githubApp.Configured() {
		slog.Debug("webhook: github app not configured, skipping scan")
		return
	}

	// Check installation ID
	if payload.Installation.ID == 0 {
		slog.Debug("webhook: no installation ID in payload, skipping scan")
		return
	}

	// Check if we have a repository
	if payload.Repository == nil || payload.Repository.FullName == "" {
		slog.Debug("webhook: no repository in payload, skipping scan")
		return
	}

	// Determine if this event should trigger a scan
	var shouldScan bool
	var sourceRef NormalizedRef
	var headSHA string

	switch event {
	case "push":
		shouldScan, sourceRef, headSHA = s.shouldScanPushEvent(payload)
	case "pull_request":
		shouldScan, sourceRef, headSHA = s.shouldScanPullRequestEvent(payload)
	default:
		// Other events don't trigger scans
		return
	}

	if !shouldScan {
		slog.Debug("webhook: event does not require scan",
			"event", event,
			"delivery", deliveryID,
		)
		return
	}

	// Look up organizations for this installation
	orgIDs, err := s.store.ListOrgsByInstallationID(ctx, payload.Installation.ID)
	if err != nil {
		slog.Error("webhook: failed to list orgs for installation",
			"installation_id", payload.Installation.ID,
			"error", err,
		)
		return
	}

	if len(orgIDs) == 0 {
		slog.Debug("webhook: no organizations found for installation",
			"installation_id", payload.Installation.ID,
		)
		return
	}

	// Create scanner
	scanner := NewScanner(s.store, s.githubApp)

	// Trigger scan for each organization (multi-tenant safety)
	for _, orgID := range orgIDs {
		slog.Info("webhook: triggering scan",
			"org_id", orgID,
			"repo", payload.Repository.FullName,
			"source_ref", sourceRef.Ref,
			"head_sha", headSHA,
		)

		input := ScanInput{
			OrgID:          orgID,
			InstallationID: payload.Installation.ID,
			RepoFullName:   payload.Repository.FullName,
			SourceRef:      sourceRef,
			HeadSHA:        headSHA,
			DeliveryID:     deliveryID,
		}

		// Scan synchronously for now (MVP)
		// TODO: Consider moving to async/queue for production
		scanner.ScanForWebhook(ctx, input)
	}
}

// shouldScanPushEvent determines if a push event should trigger a scan
func (s *Server) shouldScanPushEvent(payload *webhookPayload) (bool, NormalizedRef, string) {
	// Normalize the ref
	sourceRef := NormalizeSourceRef(payload.Ref)

	// Get the head SHA
	headSHA := payload.After
	if headSHA == "" && payload.HeadCommit != nil {
		headSHA = payload.HeadCommit.ID
	}

	// Check if any commits touch .rocketship files
	touchesRocketship := false

	// Check head_commit
	if payload.HeadCommit != nil {
		if s.commitTouchesRocketship(payload.HeadCommit) {
			touchesRocketship = true
		}
	}

	// Check all commits
	for _, commit := range payload.Commits {
		if s.commitTouchesRocketship(&commit) {
			touchesRocketship = true
			break
		}
	}

	if !touchesRocketship {
		slog.Debug("webhook: push does not touch .rocketship",
			"ref", payload.Ref,
			"commits", len(payload.Commits),
		)
		return false, sourceRef, headSHA
	}

	return true, sourceRef, headSHA
}

// shouldScanPullRequestEvent determines if a PR event should trigger a scan
func (s *Server) shouldScanPullRequestEvent(payload *webhookPayload) (bool, NormalizedRef, string) {
	// Only scan on opened, reopened, and synchronize actions
	switch payload.Action {
	case "opened", "reopened", "synchronize":
		// Continue
	default:
		return false, NormalizedRef{}, ""
	}

	if payload.PullRequest == nil {
		return false, NormalizedRef{}, ""
	}

	// Use the PR number for source_ref
	prNum := payload.PullRequest.Number
	sourceRef := NormalizedRef{
		Ref:  "pr/" + itoa(prNum),
		Kind: RefKindPR,
		Raw:  "refs/pull/" + itoa(prNum) + "/head",
	}

	// Get head SHA and branch
	headSHA := ""
	if payload.PullRequest.Head != nil {
		headSHA = payload.PullRequest.Head.SHA
	}

	// For PRs, always scan (the PR is already scoped to specific changes)
	return true, sourceRef, headSHA
}

// commitTouchesRocketship checks if a commit touches any .rocketship files
func (s *Server) commitTouchesRocketship(commit *webhookCommit) bool {
	if commit == nil {
		return false
	}

	for _, path := range commit.Added {
		if strings.Contains(path, ".rocketship") {
			return true
		}
	}
	for _, path := range commit.Modified {
		if strings.Contains(path, ".rocketship") {
			return true
		}
	}
	for _, path := range commit.Removed {
		if strings.Contains(path, ".rocketship") {
			return true
		}
	}

	return false
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// webhookPayload represents the GitHub webhook payload fields we need
type webhookPayload struct {
	Action       string              `json:"action,omitempty"`
	Ref          string              `json:"ref,omitempty"`
	Before       string              `json:"before,omitempty"`
	After        string              `json:"after,omitempty"`
	Repository   *webhookRepository  `json:"repository,omitempty"`
	Installation webhookInstallation `json:"installation,omitempty"`
	HeadCommit   *webhookCommit      `json:"head_commit,omitempty"`
	Commits      []webhookCommit     `json:"commits,omitempty"`
	PullRequest  *webhookPullRequest `json:"pull_request,omitempty"`
}

type webhookRepository struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type webhookInstallation struct {
	ID int64 `json:"id"`
}

type webhookCommit struct {
	ID       string   `json:"id"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

type webhookPullRequest struct {
	Number int                    `json:"number"`
	Head   *webhookPullRequestRef `json:"head,omitempty"`
	Base   *webhookPullRequestRef `json:"base,omitempty"`
}

type webhookPullRequestRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// verifyWebhookSignature verifies the GitHub webhook signature using HMAC-SHA256.
// The signature header format is: sha256=<hex_digest>
func verifyWebhookSignature(body []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Parse the signature header (format: sha256=<hex>)
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	providedHex := strings.TrimPrefix(signature, "sha256=")

	providedSig, err := hex.DecodeString(providedHex)
	if err != nil {
		return false
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := mac.Sum(nil)

	// Constant-time comparison
	return hmac.Equal(providedSig, expectedSig)
}
