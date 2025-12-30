package cli

import (
	"encoding/json"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// GitInfo contains local git repository information
type GitInfo struct {
	Branch        string
	CommitSHA     string
	RepoURL       string // Normalized https://github.com/<owner>/<repo> format
	CommitMessage string // First line (subject) of the commit message
}

// GetGitInfo retrieves git information from the local repository
func GetGitInfo() (*GitInfo, error) {
	info := &GitInfo{}

	// Get current branch
	branch, err := runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	info.Branch = strings.TrimSpace(branch)

	// Get HEAD commit SHA
	sha, err := runGitCommand("rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	info.CommitSHA = strings.TrimSpace(sha)

	// Get commit message subject (first line)
	msg, err := runGitCommand("log", "-1", "--pretty=%s", info.CommitSHA)
	if err == nil {
		info.CommitMessage = strings.TrimSpace(msg)
	}
	// Ignore error - commit message is optional

	// Get remote URL
	remoteURL, err := runGitCommand("config", "--get", "remote.origin.url")
	if err != nil {
		// No remote origin configured, that's okay
		return info, nil
	}
	info.RepoURL = normalizeGitURL(strings.TrimSpace(remoteURL))

	return info, nil
}

// GetRepoRoot returns the root directory of the git repository
func GetRepoRoot() (string, error) {
	root, err := runGitCommand("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(root), nil
}

// DerivePathScope determines the path scope from a YAML file path relative to the repo root
// For a file at <repoRoot>/api/.rocketship/foo.yaml, returns ["api/.rocketship/**"]
// For a file at <repoRoot>/.rocketship/foo.yaml, returns [".rocketship/**"]
func DerivePathScope(yamlFilePath string) ([]string, error) {
	repoRoot, err := GetRepoRoot()
	if err != nil {
		return nil, err
	}

	// Get absolute path of the yaml file
	absPath, err := filepath.Abs(yamlFilePath)
	if err != nil {
		return nil, err
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return nil, err
	}

	// Find the .rocketship directory in the path
	parts := strings.Split(relPath, string(filepath.Separator))
	rocketshipIdx := -1
	for i, part := range parts {
		if part == ".rocketship" {
			rocketshipIdx = i
			break
		}
	}

	if rocketshipIdx < 0 {
		// Not in a .rocketship directory, return empty
		return []string{}, nil
	}

	// Build path scope: everything up to and including .rocketship/**
	scopeParts := parts[:rocketshipIdx+1]
	scopePath := strings.Join(scopeParts, "/") + "/**"

	return []string{scopePath}, nil
}

// PathScopeToJSON converts a path scope to JSON string for metadata
func PathScopeToJSON(pathScope []string) string {
	if pathScope == nil {
		pathScope = []string{}
	}
	data, err := json.Marshal(pathScope)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// normalizeGitURL converts various git remote URL formats to https://github.com/<owner>/<repo>
func normalizeGitURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	// Handle SSH URLs: git@github.com:owner/repo.git
	sshRegex := regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(rawURL); matches != nil {
		host := matches[1]
		path := matches[2]
		return "https://" + host + "/" + path
	}

	// Handle various URL formats
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Strip .git suffix
	path := strings.TrimSuffix(u.Path, ".git")

	// Ensure leading slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Normalize to https
	return "https://" + u.Host + path
}
