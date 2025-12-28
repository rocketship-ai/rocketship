package controlplane

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

type overviewSetupStep struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Complete bool   `json:"complete"`
}

type overviewSetupResponse struct {
	Steps          []overviewSetupStep `json:"steps"`
	GitHubAppSlug  string              `json:"github_app_slug,omitempty"`
	GitHubInstallURL string            `json:"github_install_url,omitempty"`
}

func (s *Server) handleOverviewSetup(w http.ResponseWriter, r *http.Request, p brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	steps := []overviewSetupStep{
		{ID: "create_account", Title: "Create account", Complete: true}, // Always complete if authenticated
		{ID: "create_org", Title: "Create organization", Complete: p.OrgID != uuid.Nil},
		{ID: "install_github_app", Title: "Install GitHub App", Complete: false},
		{ID: "connect_repo", Title: "Connect repository", Complete: false},
	}

	var gitHubAppSlug, gitHubInstallURL string

	if p.OrgID != uuid.Nil {
		// Check GitHub App installation status
		installationID, _, _, err := s.store.GetGitHubAppInstallation(r.Context(), p.OrgID)
		if err == nil && installationID > 0 {
			steps[2].Complete = true

			// Check if they have at least one project
			projects, err := s.store.ListProjects(r.Context(), p.OrgID)
			if err == nil && len(projects) > 0 {
				steps[3].Complete = true
			}
		}
	}

	if s.githubApp.Configured() {
		gitHubAppSlug = s.githubApp.Slug()
		gitHubInstallURL = fmt.Sprintf("https://github.com/apps/%s/installations/new", gitHubAppSlug)
	}

	writeJSON(w, http.StatusOK, overviewSetupResponse{
		Steps:            steps,
		GitHubAppSlug:    gitHubAppSlug,
		GitHubInstallURL: gitHubInstallURL,
	})
}

type gitHubAppStatusResponse struct {
	Installed    bool   `json:"installed"`
	InstallURL   string `json:"install_url"`
	AccountLogin string `json:"account_login,omitempty"`
	AccountType  string `json:"account_type,omitempty"`
}

type gitHubAppRepoResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
}

type gitHubAppConnectRequest struct {
	RepoFullName string `json:"repo_full_name"`
}

// bootstrapScanResult contains the result of a single bootstrap scan
type bootstrapScanResult struct {
	SourceRef   string `json:"source_ref"`
	Status      string `json:"status"`
	SuitesFound int    `json:"suites_found"`
	TestsFound  int    `json:"tests_found"`
	Error       string `json:"error,omitempty"`
}

// gitHubAppConnectResponse is the response for the connect endpoint
type gitHubAppConnectResponse struct {
	RepoFullName  string                `json:"repo_full_name"`
	DefaultBranch string                `json:"default_branch"`
	Scans         []bootstrapScanResult `json:"scans"`
}

func (s *Server) handleGitHubAppStatus(w http.ResponseWriter, r *http.Request, p brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if p.OrgID == uuid.Nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	if !s.githubApp.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GitHub App not configured")
		return
	}

	installationID, accountLogin, accountType, err := s.store.GetGitHubAppInstallation(r.Context(), p.OrgID)
	if err != nil && err != persistence.ErrGitHubAppNotInstalled {
		writeError(w, http.StatusInternalServerError, "failed to check GitHub App installation")
		return
	}

	installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new", s.githubApp.Slug())

	if installationID == 0 {
		writeJSON(w, http.StatusOK, gitHubAppStatusResponse{
			Installed:  false,
			InstallURL: installURL,
		})
		return
	}

	writeJSON(w, http.StatusOK, gitHubAppStatusResponse{
		Installed:    true,
		InstallURL:   installURL,
		AccountLogin: accountLogin,
		AccountType:  accountType,
	})
}

func (s *Server) handleGitHubAppRepos(w http.ResponseWriter, r *http.Request, p brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if p.OrgID == uuid.Nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	if !s.githubApp.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GitHub App not configured")
		return
	}

	installationID, _, _, err := s.store.GetGitHubAppInstallation(r.Context(), p.OrgID)
	if err != nil {
		if err == persistence.ErrGitHubAppNotInstalled {
			writeError(w, http.StatusPreconditionFailed, "GitHub App not installed")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get GitHub App installation")
		return
	}

	repos, err := s.githubApp.ListRepositories(r.Context(), installationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to list repositories: %v", err))
		return
	}

	response := make([]gitHubAppRepoResponse, 0, len(repos))
	for _, repo := range repos {
		response = append(response, gitHubAppRepoResponse(repo))
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGitHubAppConnect(w http.ResponseWriter, r *http.Request, p brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if p.OrgID == uuid.Nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	if !p.HasAnyRole("admin", "owner") {
		writeError(w, http.StatusForbidden, "admin access required to connect repositories")
		return
	}

	if !s.githubApp.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GitHub App not configured")
		return
	}

	var req gitHubAppConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.RepoFullName == "" {
		writeError(w, http.StatusBadRequest, "repo_full_name is required")
		return
	}

	parts := strings.SplitN(req.RepoFullName, "/", 2)
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid repo format, expected owner/repo")
		return
	}
	owner, repoName := parts[0], parts[1]

	// Get installation ID
	installationID, _, _, err := s.store.GetGitHubAppInstallation(r.Context(), p.OrgID)
	if err != nil {
		if err == persistence.ErrGitHubAppNotInstalled {
			writeError(w, http.StatusPreconditionFailed, "GitHub App not installed")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get GitHub App installation")
		return
	}

	// Fetch repos to find the one being connected and get its default branch
	repos, err := s.githubApp.ListRepositories(r.Context(), installationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to list repositories: %v", err))
		return
	}

	var defaultBranch string
	for _, repo := range repos {
		if repo.FullName == req.RepoFullName {
			defaultBranch = repo.DefaultBranch
			break
		}
	}

	if defaultBranch == "" {
		writeError(w, http.StatusNotFound, "repository not found or not accessible")
		return
	}

	// Bootstrap scan: scan both default branch AND all open PRs
	// This ensures .rocketship dirs on existing PRs are discovered immediately
	scanner := NewScanner(s.store, s.githubApp)
	var scanResults []bootstrapScanResult

	// Create a bootstrap delivery record so we can track scan attempts
	bootstrapDeliveryID := fmt.Sprintf("bootstrap-%s-%s", p.OrgID.String()[:8], repoName)
	if err := s.store.InsertWebhookDelivery(r.Context(), bootstrapDeliveryID, "bootstrap", req.RepoFullName, "", ""); err != nil {
		slog.Warn("github app connect: failed to insert bootstrap delivery",
			"delivery_id", bootstrapDeliveryID,
			"repo", req.RepoFullName,
			"error", err,
		)
	}

	// A) Scan default branch
	defaultBranchSHA, _ := s.githubApp.GetBranchHeadSHA(r.Context(), installationID, owner, repoName, defaultBranch)
	defaultBranchRef := NormalizeSourceRef("refs/heads/" + defaultBranch)
	defaultInput := ScanInput{
		OrgID:          p.OrgID,
		InstallationID: installationID,
		RepoFullName:   req.RepoFullName,
		SourceRef:      defaultBranchRef,
		HeadSHA:        defaultBranchSHA,
		DeliveryID:     bootstrapDeliveryID,
	}
	defaultResult, defaultErr := scanner.Scan(r.Context(), defaultInput)
	if defaultErr != nil {
		scanResults = append(scanResults, bootstrapScanResult{
			SourceRef: defaultBranch,
			Status:    "error",
			Error:     defaultErr.Error(),
		})
	} else {
		scanResults = append(scanResults, bootstrapScanResult{
			SourceRef:   defaultBranch,
			Status:      "success",
			SuitesFound: defaultResult.SuitesFound,
			TestsFound:  defaultResult.TestsFound,
		})
	}
	// Record scan attempt for default branch
	scanner.recordScanAttempt(r.Context(), defaultInput, defaultResult, defaultErr)

	// B) Scan all open PRs
	openPRs, err := s.githubApp.ListOpenPullRequests(r.Context(), installationID, owner, repoName)
	if err != nil {
		slog.Warn("github app connect: failed to list open pull requests",
			"repo", req.RepoFullName,
			"error", err,
		)
	} else {
		for _, pr := range openPRs {
			prRef := NormalizedRef{
				Ref:  fmt.Sprintf("pr/%d", pr.Number),
				Kind: RefKindPR,
				Raw:  fmt.Sprintf("refs/pull/%d/head", pr.Number),
			}
			prInput := ScanInput{
				OrgID:          p.OrgID,
				InstallationID: installationID,
				RepoFullName:   req.RepoFullName,
				SourceRef:      prRef,
				HeadSHA:        pr.HeadSHA,
				DeliveryID:     bootstrapDeliveryID,
			}
			prResult, prErr := scanner.Scan(r.Context(), prInput)
			if prErr != nil {
				scanResults = append(scanResults, bootstrapScanResult{
					SourceRef: prRef.Ref,
					Status:    "error",
					Error:     prErr.Error(),
				})
			} else {
				scanResults = append(scanResults, bootstrapScanResult{
					SourceRef:   prRef.Ref,
					Status:      "success",
					SuitesFound: prResult.SuitesFound,
					TestsFound:  prResult.TestsFound,
				})
			}
			// Record scan attempt for PR
			scanner.recordScanAttempt(r.Context(), prInput, prResult, prErr)
		}
	}

	writeJSON(w, http.StatusOK, gitHubAppConnectResponse{
		RepoFullName:  req.RepoFullName,
		DefaultBranch: defaultBranch,
		Scans:         scanResults,
	})
}

func (s *Server) handleGitHubAppCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	installationIDStr := r.URL.Query().Get("installation_id")
	setupAction := r.URL.Query().Get("setup_action")

	if installationIDStr == "" {
		writeError(w, http.StatusBadRequest, "installation_id parameter required")
		return
	}

	var installationID int64
	if _, err := fmt.Sscanf(installationIDStr, "%d", &installationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid installation_id")
		return
	}

	if !s.githubApp.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GitHub App not configured")
		return
	}

	// Handle uninstall
	if setupAction == "install" || setupAction == "" {
		// Fetch installation info from GitHub to get the account details
		info, err := s.githubApp.GetInstallationInfo(r.Context(), installationID)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to get installation info: %v", err))
			return
		}

		if info == nil {
			writeError(w, http.StatusNotFound, "installation not found")
			return
		}

		// Get user from cookie/token to associate installation with their org
		var token string
		if cookie, err := r.Cookie("access_token"); err == nil {
			token = cookie.Value
		}

		if token == "" {
			// No token - redirect to login with return URL
			loginURL := fmt.Sprintf("%s/authorize?redirect_uri=%s&installation_id=%d",
				s.cfg.Issuer,
				"/github-app/callback",
				installationID,
			)
			http.Redirect(w, r, loginURL, http.StatusFound)
			return
		}

		claims, err := s.parseToken(token)
		if err != nil {
			// Invalid token - redirect to login
			loginURL := fmt.Sprintf("%s/authorize?redirect_uri=%s&installation_id=%d",
				s.cfg.Issuer,
				"/github-app/callback",
				installationID,
			)
			http.Redirect(w, r, loginURL, http.StatusFound)
			return
		}

		principal, err := principalFromClaims(claims)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}

		if principal.OrgID == uuid.Nil {
			// User is not part of an org yet - redirect to onboarding
			http.Redirect(w, r, s.cfg.Issuer+"/onboarding?github_app=pending_org", http.StatusFound)
			return
		}

		// Save the installation for this org
		if err := s.store.UpsertGitHubAppInstallation(
			r.Context(),
			principal.OrgID,
			installationID,
			principal.UserID,
			info.Account.Login,
			info.Account.Type,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save installation")
			return
		}
	}

	// Redirect back to overview page
	http.Redirect(w, r, s.cfg.Issuer+"/overview", http.StatusFound)
}
