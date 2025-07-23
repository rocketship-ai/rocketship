package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateRepositoryURL validates a repository URL
func ValidateRepositoryURL(repoURL string) error {
	// Parse URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	// Check if it looks like a GitHub URL
	if !strings.Contains(u.Host, "github") {
		return fmt.Errorf("currently only GitHub repositories are supported")
	}

	// Check path format
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 {
		return fmt.Errorf("URL must include owner and repository name")
	}

	return nil
}