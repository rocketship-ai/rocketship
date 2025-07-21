package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"

	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// Config holds GitHub API configuration
type Config struct {
	// GitHub App configuration (preferred for enterprise)
	AppID          int64  `json:"app_id,omitempty"`
	PrivateKey     []byte `json:"private_key,omitempty"`
	InstallationID int64  `json:"installation_id,omitempty"`

	// Personal Access Token (fallback)
	Token string `json:"token,omitempty"`

	// Base URL for GitHub Enterprise Server (optional)
	BaseURL string `json:"base_url,omitempty"`

	// Organization name for team name conversion
	Organization string `json:"organization,omitempty"`
}

// AuthMethod represents the type of GitHub authentication being used
type AuthMethod string

const (
	AuthMethodApp   AuthMethod = "github_app"
	AuthMethodToken AuthMethod = "personal_token"
	AuthMethodNone  AuthMethod = "none"
)

// Client handles GitHub API operations
type Client struct {
	client     *github.Client
	config     *Config
	authMethod AuthMethod
}

// NewClient creates a new GitHub client based on available configuration
func NewClient(ctx context.Context) (*Client, error) {
	config, err := LoadConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load GitHub config: %w", err)
	}

	return NewClientWithConfig(ctx, config)
}

// NewClientWithConfig creates a new GitHub client with the provided configuration
func NewClientWithConfig(ctx context.Context, config *Config) (*Client, error) {
	var httpClient *http.Client
	var authMethod AuthMethod

	// Try GitHub App authentication first (preferred)
	if config.AppID != 0 && len(config.PrivateKey) > 0 && config.InstallationID != 0 {
		transport, err := ghinstallation.New(http.DefaultTransport, config.AppID, config.InstallationID, config.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
		}

		if config.BaseURL != "" {
			transport.BaseURL = config.BaseURL
		}

		httpClient = &http.Client{Transport: transport}
		authMethod = AuthMethodApp
	} else if config.Token != "" {
		// Fallback to Personal Access Token
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Token})
		httpClient = oauth2.NewClient(ctx, ts)
		authMethod = AuthMethodToken
	} else {
		// No authentication configured - use anonymous client (limited rate limits)
		httpClient = http.DefaultClient
		authMethod = AuthMethodNone
	}

	var githubClient *github.Client
	if config.BaseURL != "" {
		// GitHub Enterprise Server
		githubClient, err := github.NewEnterpriseClient(config.BaseURL, config.BaseURL, httpClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub Enterprise client: %w", err)
		}
		return &Client{
			client:     githubClient,
			config:     config,
			authMethod: authMethod,
		}, nil
	} else {
		// GitHub.com
		githubClient = github.NewClient(httpClient)
	}

	return &Client{
		client:     githubClient,
		config:     config,
		authMethod: authMethod,
	}, nil
}

// LoadConfigFromEnv loads GitHub configuration from environment variables
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{
		BaseURL:      os.Getenv("GITHUB_BASE_URL"),
		Organization: os.Getenv("GITHUB_ORGANIZATION"),
	}

	// Set default organization if not specified
	if config.Organization == "" {
		config.Organization = "rocketship-ai" // Default for backwards compatibility
	}

	// Try to load GitHub App configuration
	if appIDStr := os.Getenv("GITHUB_APP_ID"); appIDStr != "" {
		appID, err := strconv.ParseInt(appIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
		}
		config.AppID = appID

		privateKeyStr := os.Getenv("GITHUB_PRIVATE_KEY")
		if privateKeyStr == "" {
			return nil, fmt.Errorf("GITHUB_PRIVATE_KEY is required when GITHUB_APP_ID is set")
		}
		config.PrivateKey = []byte(privateKeyStr)

		installationIDStr := os.Getenv("GITHUB_INSTALLATION_ID")
		if installationIDStr == "" {
			return nil, fmt.Errorf("GITHUB_INSTALLATION_ID is required when GITHUB_APP_ID is set")
		}

		installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid GITHUB_INSTALLATION_ID: %w", err)
		}
		config.InstallationID = installationID
	} else if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		// Fallback to Personal Access Token
		config.Token = token
	}

	return config, nil
}

// GetAuthMethod returns the authentication method being used
func (c *Client) GetAuthMethod() AuthMethod {
	return c.authMethod
}

// NewClientWithAuth creates a new GitHub client with OAuth2 authentication (legacy method)
func NewClientWithAuth(ctx context.Context, accessToken string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	
	return &Client{
		client: github.NewClient(tc),
		config: &Config{
			Token:        accessToken,
			Organization: "rocketship-ai", // Default
		},
		authMethod: AuthMethodToken,
	}
}

// GetRepository retrieves repository information
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repository, nil
}

// GetCodeowners retrieves and parses CODEOWNERS file
func (c *Client) GetCodeowners(ctx context.Context, owner, repo, branch string) (*rbac.CodeownersData, error) {
	// Try different possible locations for CODEOWNERS file
	possiblePaths := []string{
		"CODEOWNERS",
		".github/CODEOWNERS",
		"docs/CODEOWNERS",
	}
	
	var content string
	var found bool
	
	for _, path := range possiblePaths {
		fileContent, _, resp, err := c.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
			Ref: branch,
		})
		
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				continue // Try next path
			}
			return nil, fmt.Errorf("failed to get CODEOWNERS file: %w", err)
		}
		
		if fileContent != nil {
			decodedContent, err := fileContent.GetContent()
			if err != nil {
				return nil, fmt.Errorf("failed to decode CODEOWNERS content: %w", err)
			}
			content = decodedContent
			found = true
			break
		}
	}
	
	if !found {
		return nil, fmt.Errorf("CODEOWNERS file not found in repository %s/%s", owner, repo)
	}
	
	// Parse CODEOWNERS content
	return parseCodeowners(content)
}

// parseCodeowners parses CODEOWNERS file content
func parseCodeowners(content string) (*rbac.CodeownersData, error) {
	lines := strings.Split(content, "\n")
	var rules []rbac.CodeownersRule
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse rule: pattern followed by owners
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		
		pattern := parts[0]
		owners := parts[1:]
		
		// Clean up owners (remove @ prefix if present for consistency)
		var cleanOwners []string
		for _, owner := range owners {
			if strings.HasPrefix(owner, "@") {
				cleanOwners = append(cleanOwners, owner)
			} else {
				cleanOwners = append(cleanOwners, "@"+owner)
			}
		}
		
		rules = append(rules, rbac.CodeownersRule{
			Pattern: pattern,
			Owners:  cleanOwners,
		})
	}
	
	return &rbac.CodeownersData{
		Rules:   rules,
		Updated: time.Now(),
	}, nil
}

// GetRepositoryInfo extracts owner and repo name from repository URL
func GetRepositoryInfo(repoURL string) (owner, repo string, err error) {
	return ParseRepositoryURL(repoURL)
}

// ParseRepositoryURL parses a GitHub repository URL and extracts owner/repo
func ParseRepositoryURL(repoURL string) (owner, repo string, err error) {
	// Handle different URL formats:
	// https://github.com/owner/repo
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git
	// github.com/owner/repo
	// owner/repo
	
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	
	// Remove .git suffix if present
	repoURL = strings.TrimSuffix(repoURL, ".git")
	
	// HTTPS format: https://github.com/owner/repo
	if strings.HasPrefix(repoURL, "https://github.com/") {
		path := strings.TrimPrefix(repoURL, "https://github.com/")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}
	
	// SSH format: git@github.com:owner/repo
	if strings.HasPrefix(repoURL, "git@github.com:") {
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}
	
	// Domain format: github.com/owner/repo
	if strings.HasPrefix(repoURL, "github.com/") {
		path := strings.TrimPrefix(repoURL, "github.com/")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}
	
	// Simple format: owner/repo
	if strings.Contains(repoURL, "/") && !strings.Contains(repoURL, "://") {
		parts := strings.Split(repoURL, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}
	
	return "", "", fmt.Errorf("invalid GitHub repository URL: %s", repoURL)
}

// ValidateAndGetRepository validates a repository URL and returns repository information
func (c *Client) ValidateAndGetRepository(ctx context.Context, repoURL string) (*github.Repository, error) {
	owner, repo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}
	
	repository, err := c.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to validate repository access: %w", err)
	}
	
	return repository, nil
}

// GetRepositoryMetadata returns structured metadata about a repository
func (c *Client) GetRepositoryMetadata(ctx context.Context, repoURL string) (*RepositoryMetadata, error) {
	repository, err := c.ValidateAndGetRepository(ctx, repoURL)
	if err != nil {
		return nil, err
	}
	
	metadata := &RepositoryMetadata{
		Owner:         repository.Owner.GetLogin(),
		Name:          repository.GetName(),
		FullName:      repository.GetFullName(),
		Description:   repository.GetDescription(),
		Private:       repository.GetPrivate(),
		DefaultBranch: repository.GetDefaultBranch(),
		HTMLURL:       repository.GetHTMLURL(),
		CloneURL:      repository.GetCloneURL(),
		Language:      repository.GetLanguage(),
		Topics:        repository.Topics,
		CreatedAt:     repository.GetCreatedAt().Time,
		UpdatedAt:     repository.GetUpdatedAt().Time,
		HasCodeowners: false, // Will be set after checking for CODEOWNERS
	}
	
	// Check if repository has CODEOWNERS file
	_, err = c.GetCodeowners(ctx, metadata.Owner, metadata.Name, metadata.DefaultBranch)
	if err == nil {
		metadata.HasCodeowners = true
	}
	
	return metadata, nil
}

// RepositoryMetadata contains structured metadata about a GitHub repository
type RepositoryMetadata struct {
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"default_branch"`
	HTMLURL       string    `json:"html_url"`
	CloneURL      string    `json:"clone_url"`
	Language      string    `json:"language,omitempty"`
	Topics        []string  `json:"topics,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	HasCodeowners bool      `json:"has_codeowners"`
}

// ValidateRepositoryAccess checks if the client has access to a repository
func (c *Client) ValidateRepositoryAccess(ctx context.Context, owner, repo string) error {
	_, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to access repository: %w", err)
	}
	return nil
}

// CheckPathOwnership checks if a team owns a specific path according to CODEOWNERS
func (c *Client) CheckPathOwnership(codeownersData *rbac.CodeownersData, filePath string, teamNames []string) bool {
	if codeownersData == nil || len(codeownersData.Rules) == 0 {
		// No CODEOWNERS file means no restrictions
		return true
	}
	
	// Convert team names to GitHub team format for comparison
	githubTeams := make(map[string]bool)
	for _, teamName := range teamNames {
		githubTeam := c.convertTeamNameToGitHub(teamName)
		githubTeams[githubTeam] = true
	}
	
	// Find the most specific rule that matches the path
	var matchingRule *rbac.CodeownersRule
	var longestMatch int
	
	for _, rule := range codeownersData.Rules {
		if matchesPattern(rule.Pattern, filePath) {
			// For CODEOWNERS, later rules override earlier ones
			// and more specific patterns take precedence
			patternSpecificity := getPatternSpecificity(rule.Pattern, filePath)
			if patternSpecificity > longestMatch {
				longestMatch = patternSpecificity
				matchingRule = &rule
			}
		}
	}
	
	if matchingRule == nil {
		// No matching rule means no restrictions
		return true
	}
	
	// Check if any of the user's teams own this path
	for _, owner := range matchingRule.Owners {
		if githubTeams[owner] {
			return true
		}
	}
	
	return false
}

// convertTeamNameToGitHub converts internal team name to GitHub team format
func (c *Client) convertTeamNameToGitHub(teamName string) string {
	// Convert "Backend Team" to "@organization/backend-team"
	// Convert "QA Team" to "@organization/qa-team"
	// Convert "Frontend Team" to "@organization/frontend-team"
	
	org := c.config.Organization
	if org == "" {
		org = "rocketship-ai" // Fallback
	}
	
	teamName = strings.ToLower(teamName)
	teamName = strings.ReplaceAll(teamName, " ", "-")
	return fmt.Sprintf("@%s/%s", org, teamName)
}

// matchesPattern checks if a file path matches a CODEOWNERS pattern
func matchesPattern(pattern, filePath string) bool {
	// Normalize paths
	pattern = strings.TrimPrefix(pattern, "/")
	filePath = strings.TrimPrefix(filePath, "/")
	
	// Handle different pattern types
	if pattern == "*" {
		return true
	}
	
	if strings.HasSuffix(pattern, "/") {
		// Directory pattern - matches if path starts with pattern
		return strings.HasPrefix(filePath, pattern) || strings.HasPrefix(filePath+"/", pattern)
	}
	
	if strings.HasPrefix(pattern, "*.") {
		// Extension pattern - check file extension
		extension := pattern[1:] // Remove the *
		return strings.HasSuffix(filePath, extension)
	}
	
	if strings.Contains(pattern, "*") {
		// Glob pattern - use simplified glob matching
		return matchGlob(pattern, filePath)
	}
	
	// Exact match or prefix match
	if pattern == filePath {
		return true
	}
	
	// Check if it's a directory prefix match
	if strings.HasPrefix(filePath, pattern+"/") {
		return true
	}
	
	return false
}

// matchGlob provides simplified glob pattern matching
func matchGlob(pattern, str string) bool {
	// Convert glob pattern to regex-like matching
	// This is a simplified implementation for common CODEOWNERS patterns
	
	// Handle ** patterns (match any directories)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			
			// Remove leading/trailing slashes for comparison
			prefix = strings.Trim(prefix, "/")
			suffix = strings.Trim(suffix, "/")
			
			if prefix != "" && !strings.HasPrefix(str, prefix) {
				return false
			}
			
			if suffix != "" && !strings.HasSuffix(str, suffix) {
				return false
			}
			
			return true
		}
	}
	
	// Handle single * patterns
	if strings.Contains(pattern, "*") {
		// Simple implementation - check prefix and suffix
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			
			return strings.HasPrefix(str, prefix) && strings.HasSuffix(str, suffix)
		}
	}
	
	return false
}

// getPatternSpecificity returns a score for how specific a pattern is
// Higher scores indicate more specific patterns
func getPatternSpecificity(pattern, filePath string) int {
	specificity := 0
	
	// Exact matches are most specific
	if pattern == filePath {
		return 1000
	}
	
	// Extension patterns are less specific than directory patterns
	if strings.HasPrefix(pattern, "*.") {
		specificity = 10
	} else if strings.Contains(pattern, "*") {
		// Glob patterns are less specific
		specificity = 50
	} else {
		// Directory/path patterns
		specificity = 100 + len(strings.Split(pattern, "/"))
	}
	
	return specificity
}