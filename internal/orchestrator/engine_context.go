package orchestrator

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
)

func extractRunContext(reqContext *generated.RunContext) *RunContext {
	if reqContext == nil {
		slog.Debug("No context provided, auto-detecting from environment")
		return &RunContext{
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

	return &RunContext{
		ProjectID:    reqContext.ProjectId,
		Source:       reqContext.Source,
		Branch:       reqContext.Branch,
		CommitSHA:    reqContext.CommitSha,
		Trigger:      reqContext.Trigger,
		ScheduleName: reqContext.ScheduleName,
		Metadata:     reqContext.Metadata,
	}
}

func detectProjectID() string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
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
	ciEnvVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "BUILDKITE"}
	for _, envVar := range ciEnvVars {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("echo $%s", envVar))
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) != "" {
			slog.Debug("Detected CI environment", "source", "ci-branch")
			return "ci-branch"
		}
	}

	slog.Debug("Detected local environment", "source", "cli-local")
	return "cli-local"
}

func detectBranch() string {
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
	if detectSource() == "ci-branch" {
		return "webhook"
	}
	return "manual"
}

func determineInitiator(principal *Principal) string {
	if principal == nil {
		return "unknown"
	}
	// Prefer GitHub username (for manual runs) over email
	if username := strings.TrimSpace(principal.Username); username != "" {
		return username
	}
	if email := strings.TrimSpace(principal.Email); email != "" {
		return email
	}
	if subject := strings.TrimSpace(principal.Subject); subject != "" {
		return subject
	}
	return "unknown"
}

func detectConfigSource(ctx *RunContext) string {
	if ctx == nil {
		return "repo_commit"
	}
	if strings.TrimSpace(ctx.CommitSHA) == "" {
		return "uncommitted"
	}
	return "repo_commit"
}

func detectEnvironment(ctx *RunContext) string {
	if ctx == nil {
		return ""
	}
	if ctx.Metadata == nil {
		return ""
	}
	keys := []string{"environment", "env", "rs_environment"}
	for _, key := range keys {
		if val, ok := ctx.Metadata[key]; ok {
			return strings.TrimSpace(val)
		}
	}
	return ""
}
