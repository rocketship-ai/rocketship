package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"

	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// Client handles GitHub API operations
type Client struct {
	client *github.Client
}

// NewClient creates a new GitHub client
func NewClient(accessToken string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	
	return &Client{
		client: github.NewClient(tc),
	}
}

// NewClientWithAuth creates a new GitHub client with OAuth2 authentication
func NewClientWithAuth(ctx context.Context, accessToken string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	
	return &Client{
		client: github.NewClient(tc),
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
		return &rbac.CodeownersData{
			Rules:   []rbac.CodeownersRule{},
			Updated: time.Now(),
		}, nil
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
	// Handle different URL formats
	// https://github.com/owner/repo
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git
	
	repoURL = strings.TrimSpace(repoURL)
	
	// Remove .git suffix if present
	repoURL = strings.TrimSuffix(repoURL, ".git")
	
	if strings.HasPrefix(repoURL, "https://github.com/") {
		// HTTPS URL
		path := strings.TrimPrefix(repoURL, "https://github.com/")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	} else if strings.HasPrefix(repoURL, "git@github.com:") {
		// SSH URL
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}
	
	return "", "", fmt.Errorf("invalid GitHub repository URL: %s", repoURL)
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
func CheckPathOwnership(codeownersData *rbac.CodeownersData, filePath string, teamNames []string) bool {
	if codeownersData == nil || len(codeownersData.Rules) == 0 {
		// No CODEOWNERS file means no restrictions
		return true
	}
	
	// Convert team names to GitHub team format for comparison
	githubTeams := make(map[string]bool)
	for _, teamName := range teamNames {
		// Convert "Backend Team" to "@rocketship-ai/backend-team"
		githubTeam := convertTeamNameToGitHub(teamName)
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
func convertTeamNameToGitHub(teamName string) string {
	// Convert "Backend Team" to "@rocketship-ai/backend-team"
	// Convert "QA Team" to "@rocketship-ai/qa-team"
	// Convert "Frontend Team" to "@rocketship-ai/frontend-team"
	
	teamName = strings.ToLower(teamName)
	teamName = strings.ReplaceAll(teamName, " ", "-")
	return fmt.Sprintf("@rocketship-ai/%s", teamName)
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