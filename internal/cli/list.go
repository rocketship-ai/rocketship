package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/spf13/cobra"
)

// ListFlags holds the flags for the list command
type ListFlags struct {
	Engine       string
	ProjectID    string
	Source       string
	Branch       string
	Status       string
	ScheduleName string
	Limit        int32
	OrderBy      string
	Ascending    bool
	Format       string // table, json, yaml
}

// NewListCmd creates a new list command
func NewListCmd() *cobra.Command {
	flags := &ListFlags{
		Engine:  "",
		Limit:   20,
		OrderBy: "started_at",
		Format:  "table",
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List test runs",
		Long: `List test runs with filtering and sorting options.

Examples:
  # List all recent runs
  rocketship list

  # List failed runs
  rocketship list --status FAILED

  # List runs for a specific project
  rocketship list --project-id my-app

  # List runs from a specific branch
  rocketship list --branch feature/new-api

  # List runs with custom limit and ordering
  rocketship list --limit 50 --order-by duration --ascending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, flags)
		},
	}

	// Engine connection
	cmd.Flags().StringVarP(&flags.Engine, "engine", "e", flags.Engine, "Address of the rocketship engine (defaults to active profile)")

	// Filtering flags
	cmd.Flags().StringVar(&flags.ProjectID, "project-id", "", "Filter by project ID")
	cmd.Flags().StringVar(&flags.Source, "source", "", "Filter by source (cli-local, ci-branch, ci-main, scheduled)")
	cmd.Flags().StringVar(&flags.Branch, "branch", "", "Filter by git branch")
	cmd.Flags().StringVar(&flags.Status, "status", "", "Filter by status (PENDING, RUNNING, PASSED, FAILED, TIMEOUT)")
	cmd.Flags().StringVar(&flags.ScheduleName, "schedule-name", "", "Filter by schedule name")

	// Display options
	cmd.Flags().Int32Var(&flags.Limit, "limit", flags.Limit, "Maximum number of runs to display")
	cmd.Flags().StringVar(&flags.OrderBy, "order-by", flags.OrderBy, "Sort by field (started_at, ended_at, duration)")
	cmd.Flags().BoolVar(&flags.Ascending, "ascending", false, "Sort in ascending order (default: descending)")
	cmd.Flags().StringVar(&flags.Format, "format", flags.Format, "Output format (table, json, yaml)")

	return cmd
}

func runList(cmd *cobra.Command, flags *ListFlags) error {
	engineAddr := ""
	if cmd.Flags().Changed("engine") {
		engineAddr = flags.Engine
	}
	// Connect to engine
	client, err := NewEngineClient(engineAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to engine: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			Logger.Debug("failed to close client", "error", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build request
	req := &generated.ListRunsRequest{
		ProjectId:    flags.ProjectID,
		Source:       flags.Source,
		Branch:       flags.Branch,
		Status:       flags.Status,
		ScheduleName: flags.ScheduleName,
		Limit:        flags.Limit,
		OrderBy:      flags.OrderBy,
		Descending:   !flags.Ascending,
	}

	Logger.Debug("listing test runs",
		"project_id", req.ProjectId,
		"source", req.Source,
		"branch", req.Branch,
		"status", req.Status,
		"limit", req.Limit,
		"order_by", req.OrderBy)

	// Call ListRuns
	resp, err := client.client.ListRuns(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	Logger.Debug("received runs", "count", len(resp.Runs), "total", resp.TotalCount)

	// Display results based on format
	switch flags.Format {
	case "table":
		return displayRunsTable(resp.Runs)
	case "json":
		// TODO: Implement JSON output
		return fmt.Errorf("JSON format not implemented yet")
	case "yaml":
		// TODO: Implement YAML output
		return fmt.Errorf("YAML format not implemented yet")
	default:
		return fmt.Errorf("unknown format: %s", flags.Format)
	}
}

func displayRunsTable(runs []*generated.RunSummary) error {
	if len(runs) == 0 {
		fmt.Println("No test runs found.")
		return nil
	}

	// Create a tabwriter for nice formatting
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {
		if err := w.Flush(); err != nil {
			Logger.Debug("failed to flush writer", "error", err)
		}
	}()

	// Print header
	if _, err := fmt.Fprintf(w, "RUN ID\tSTATUS\tSUITE\tTESTS\tDURATION\tSTARTED\tSOURCE\tPROJECT\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "------\t------\t-----\t-----\t--------\t-------\t------\t-------\n"); err != nil {
		return err
	}

	// Print each run
	for _, run := range runs {
		// Format test counts
		testCounts := fmt.Sprintf("%d/%d/%d", run.PassedTests, run.FailedTests, run.TotalTests)

		// Format duration
		duration := "N/A"
		if run.DurationMs > 0 {
			duration = formatDuration(run.DurationMs)
		}

		// Format started time
		startedAt := "N/A"
		if run.StartedAt != "" {
			if t, err := time.Parse(time.RFC3339, run.StartedAt); err == nil {
				startedAt = formatTime(t)
			}
		}

		// Get status emoji
		statusIcon := getStatusIcon(run.Status)

		// Get context info
		source := "unknown"
		projectID := "unknown"
		if run.Context != nil {
			if run.Context.Source != "" {
				source = run.Context.Source
			}
			if run.Context.ProjectId != "" {
				projectID = run.Context.ProjectId
			}
		}

		if _, err := fmt.Fprintf(w, "%s\t%s %s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			run.RunId[:12], // Truncate run ID for display
			statusIcon,
			run.Status,
			truncate(run.SuiteName, 30),
			testCounts,
			duration,
			startedAt,
			source,
			projectID,
		); err != nil {
			return err
		}
	}

	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "PASSED":
		return "✓"
	case "FAILED":
		return "✗"
	case "RUNNING":
		return "↻"
	case "PENDING":
		return "⏳"
	case "TIMEOUT":
		return "⏱"
	default:
		return "?"
	}
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	} else if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	} else {
		minutes := ms / 60000
		seconds := (ms % 60000) / 1000
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	} else if diff < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	} else {
		return t.Format("Jan 02")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
