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
			slog.Debug("Detected CI environment", "source", "github-actions")
			return "github-actions"
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
	if detectSource() == "github-actions" {
		return "ci"
	}
	return "manual"
}

func determineInitiator(principal *Principal) string {
	if principal == nil {
		return "unknown"
	}

	// CI token principals: ci_token:<uuid>
	if principal.IsCIToken {
		return "ci_token:" + principal.CITokenID.String()
	}

	// User principals: user:<github_username>
	// Prefer GitHub username over email for consistency
	if username := strings.TrimSpace(principal.Username); username != "" {
		return "user:" + username
	}
	if email := strings.TrimSpace(principal.Email); email != "" {
		return "user:" + email
	}
	if subject := strings.TrimSpace(principal.Subject); subject != "" {
		return "user:" + subject
	}
	return "unknown"
}

func detectConfigSource(ctx *RunContext) string {
	if ctx == nil {
		return "repo_commit"
	}
	// Honor explicit per-file config_source from CLI metadata
	if ctx.Metadata != nil {
		if source := strings.TrimSpace(ctx.Metadata["rs_config_source"]); source != "" {
			if source == "repo_commit" || source == "uncommitted" {
				slog.Debug("Using explicit config_source from CLI", "config_source", source)
				return source
			}
		}
	}
	// Fallback: infer from commit_sha presence (for backwards compatibility)
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
