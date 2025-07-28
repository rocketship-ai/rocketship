package runs

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/types"
)

// extractRunContext extracts run context from request or detects from environment
func extractRunContext(reqContext *generated.RunContext) *types.RunContext {
	if reqContext == nil {
		// Auto-detect context from environment
		slog.Debug("No context provided, auto-detecting from environment")
		return &types.RunContext{
			ProjectID:    detectProjectID(),
			Source:       detectSource(),
			Branch:       detectBranch(),
			CommitSHA:    detectCommitSHA(),
			Trigger:      detectTrigger(),
			ScheduleName: "",
			Metadata:     make(map[string]string),
		}
	}

	slog.Debug("Using provided context",
		"project_id", reqContext.ProjectId,
		"source", reqContext.Source,
		"branch", reqContext.Branch)

	return &types.RunContext{
		ProjectID:    reqContext.ProjectId,
		Source:       reqContext.Source,
		Branch:       reqContext.Branch,
		CommitSHA:    reqContext.CommitSha,
		Trigger:      reqContext.Trigger,
		ScheduleName: reqContext.ScheduleName,
		Metadata:     reqContext.Metadata,
	}
}

// Auto-detection helper functions
func detectProjectID() string {
	// Try to get from git remote origin
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		// Extract repo name from URL
		if strings.Contains(url, "/") {
			parts := strings.Split(url, "/")
			if len(parts) > 0 {
				repo := parts[len(parts)-1]
				repo = strings.TrimSuffix(repo, ".git")
				if repo != "" {
					slog.Debug("Detected project ID from git remote", "project_id", repo)
					return repo
				}
			}
		}
	}

	slog.Debug("Using default project ID")
	return "default"
}

func detectSource() string {
	// Check for common CI environment variables
	ciEnvVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "BUILDKITE"}
	for _, envVar := range ciEnvVars {
		if cmd := exec.Command("sh", "-c", fmt.Sprintf("echo $%s", envVar)); cmd != nil {
			if output, err := cmd.Output(); err == nil && strings.TrimSpace(string(output)) != "" {
				slog.Debug("Detected CI environment", "source", "ci-branch")
				return "ci-branch"
			}
		}
	}

	slog.Debug("Detected local environment", "source", "cli-local")
	return "cli-local"
}

func detectBranch() string {
	// Try git branch --show-current
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" {
			slog.Debug("Detected git branch", "branch", branch)
			return branch
		}
	}

	slog.Debug("Could not detect git branch")
	return ""
}

func detectCommitSHA() string {
	// Try git rev-parse HEAD
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err == nil {
		commit := strings.TrimSpace(string(output))
		if commit != "" {
			slog.Debug("Detected git commit", "commit", commit[:8])
			return commit
		}
	}

	slog.Debug("Could not detect git commit")
	return ""
}

func detectTrigger() string {
	// If it's CI, it's likely webhook triggered, otherwise manual
	if detectSource() == "ci-branch" {
		return "webhook"
	}
	return "manual"
}