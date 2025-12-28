package controlplane

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	gitHubAppTokenURL          = "https://api.github.com/app/installations/%d/access_tokens"
	gitHubAppInstallationsURL  = "https://api.github.com/app/installations"
	gitHubInstallationReposURL = "https://api.github.com/installation/repositories"
)

type GitHubAppClient struct {
	appID      int64
	slug       string
	privateKey *rsa.PrivateKey
	client     *http.Client

	// Token cache
	mu                sync.RWMutex
	installationToken map[int64]cachedToken
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

type InstallationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type GitHubAppInstallationInfo struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

type GitHubAppRepo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
}

type installationReposResponse struct {
	TotalCount   int             `json:"total_count"`
	Repositories []GitHubAppRepo `json:"repositories"`
}

func NewGitHubAppClient(cfg GitHubAppConfig, httpClient *http.Client) (*GitHubAppClient, error) {
	if cfg.AppID == 0 || cfg.PrivateKeyPEM == "" {
		return nil, nil // GitHub App not configured
	}

	block, _ := pem.Decode([]byte(cfg.PrivateKeyPEM))
	if block == nil {
		return nil, errors.New("failed to decode GitHub App private key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		k, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse GitHub App private key: %w (PKCS8: %v)", err, err2)
		}
		var ok bool
		key, ok = k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("GitHub App private key must be RSA")
		}
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &GitHubAppClient{
		appID:             cfg.AppID,
		slug:              cfg.Slug,
		privateKey:        key,
		client:            httpClient,
		installationToken: make(map[int64]cachedToken),
	}, nil
}

func (g *GitHubAppClient) Configured() bool {
	return g != nil && g.privateKey != nil
}

func (g *GitHubAppClient) Slug() string {
	if g == nil {
		return ""
	}
	return g.slug
}

func (g *GitHubAppClient) generateAppJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(), // Issued 60 seconds ago to account for clock drift
		"exp": now.Add(10 * time.Minute).Unix(),   // GitHub App JWTs expire after 10 minutes max
		"iss": g.appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.privateKey)
}

func (g *GitHubAppClient) GetInstallationToken(ctx context.Context, installationID int64) (string, error) {
	// Check cache first
	g.mu.RLock()
	if cached, ok := g.installationToken[installationID]; ok {
		if time.Now().Add(5 * time.Minute).Before(cached.expiresAt) {
			g.mu.RUnlock()
			return cached.token, nil
		}
	}
	g.mu.RUnlock()

	// Generate new token
	appJWT, err := g.generateAppJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate app JWT: %w", err)
	}

	url := fmt.Sprintf(gitHubAppTokenURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("User-Agent", "rocketship-controlplane")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get installation token (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp InstallationTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode installation token response: %w", err)
	}

	// Cache the token
	g.mu.Lock()
	g.installationToken[installationID] = cachedToken{
		token:     tokenResp.Token,
		expiresAt: tokenResp.ExpiresAt,
	}
	g.mu.Unlock()

	return tokenResp.Token, nil
}

func (g *GitHubAppClient) GetInstallationInfo(ctx context.Context, installationID int64) (*GitHubAppInstallationInfo, error) {
	appJWT, err := g.generateAppJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate app JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d", installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("User-Agent", "rocketship-controlplane")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Installation not found
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get installation info (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info GitHubAppInstallationInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode installation info: %w", err)
	}

	return &info, nil
}

// GitHubRepoInfo contains repository metadata
type GitHubRepoInfo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// GitHubTreeEntry represents an entry in a git tree
type GitHubTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob", "tree"
	SHA  string `json:"sha"`
	Size int64  `json:"size,omitempty"`
}

// GitHubTree represents a git tree response
type GitHubTree struct {
	SHA       string            `json:"sha"`
	URL       string            `json:"url"`
	Tree      []GitHubTreeEntry `json:"tree"`
	Truncated bool              `json:"truncated"`
}

// GitHubContent represents file content from GitHub API
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int64  `json:"size"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	DownloadURL string `json:"download_url"`
}

// GetRepository fetches repository metadata
func (g *GitHubAppClient) GetRepository(ctx context.Context, installationID int64, owner, repo string) (*GitHubRepoInfo, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "rocketship-controlplane")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get repository (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info GitHubRepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode repository info: %w", err)
	}

	return &info, nil
}

// GetTree fetches the git tree for a repo at a specific ref
func (g *GitHubAppClient) GetTree(ctx context.Context, installationID int64, owner, repo, ref string, recursive bool) (*GitHubTree, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s", owner, repo, ref)
	if recursive {
		url += "?recursive=1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "rocketship-controlplane")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tree not found for ref %q", ref)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get tree (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tree GitHubTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree: %w", err)
	}

	return &tree, nil
}

// GetFileContent fetches file content from GitHub
func (g *GitHubAppClient) GetFileContent(ctx context.Context, installationID int64, owner, repo, path, ref string) ([]byte, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)
	if ref != "" {
		url += "?ref=" + ref
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Request raw content directly
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "rocketship-controlplane")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get file content (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return io.ReadAll(resp.Body)
}

func (g *GitHubAppClient) ListRepositories(ctx context.Context, installationID int64) ([]GitHubAppRepo, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	var allRepos []GitHubAppRepo
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s?per_page=%d&page=%d", gitHubInstallationReposURL, perPage, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "rocketship-controlplane")

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to list repositories (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var reposResp installationReposResponse
		if err := json.NewDecoder(resp.Body).Decode(&reposResp); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode repositories response: %w", err)
		}
		_ = resp.Body.Close()

		allRepos = append(allRepos, reposResp.Repositories...)

		if len(reposResp.Repositories) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

// PullRequestInfo contains basic info about a pull request
type PullRequestInfo struct {
	Number  int    `json:"number"`
	HeadRef string `json:"head_ref"`
	HeadSHA string `json:"head_sha"`
}

type pullRequestResponse struct {
	Number int `json:"number"`
	Head   struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

// ListOpenPullRequests fetches all open PRs for a repository
func (g *GitHubAppClient) ListOpenPullRequests(ctx context.Context, installationID int64, owner, repo string) ([]PullRequestInfo, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	var allPRs []PullRequestInfo
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&per_page=%d&page=%d", owner, repo, perPage, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "rocketship-controlplane")

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to list pull requests (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var prs []pullRequestResponse
		if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode pull requests response: %w", err)
		}
		_ = resp.Body.Close()

		for _, pr := range prs {
			allPRs = append(allPRs, PullRequestInfo{
				Number:  pr.Number,
				HeadRef: pr.Head.Ref,
				HeadSHA: pr.Head.SHA,
			})
		}

		if len(prs) < perPage {
			break
		}
		page++
	}

	return allPRs, nil
}

// GetBranchHeadSHA fetches the HEAD SHA of a branch
func (g *GitHubAppClient) GetBranchHeadSHA(ctx context.Context, installationID int64, owner, repo, branch string) (string, error) {
	token, err := g.GetInstallationToken(ctx, installationID)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s", owner, repo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "rocketship-controlplane")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("branch %q not found", branch)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get branch ref (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var refResp struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&refResp); err != nil {
		return "", fmt.Errorf("failed to decode branch ref response: %w", err)
	}

	return refResp.Object.SHA, nil
}
