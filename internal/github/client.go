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