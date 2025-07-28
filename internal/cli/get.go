package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/spf13/cobra"
)

// GetFlags holds the flags for the get command
type GetFlags struct {
	Engine  string
	Profile string
	Format  string // table, json, yaml
	Logs    bool   // Show logs for the run
}

// NewGetCmd creates a new get command
func NewGetCmd() *cobra.Command {
	flags := &GetFlags{
		Engine: "localhost:7700",
		Format: "table",
	}

	cmd := &cobra.Command{
		Use:   "get <run-id>",
		Short: "Get details of a specific test run",
		Long: `Get detailed information about a specific test run.

Examples:
  # Get run details
  rocketship get abc123def456

  # Get run details with logs
  rocketship get abc123def456 --logs

  # Get run details in JSON format
  rocketship get abc123def456 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile from global flag if not set locally
			if flags.Profile == "" {
				flags.Profile = cmd.Root().PersistentFlags().Lookup("profile").Value.String()
			}
			runID := args[0]
			return runGet(runID, flags)
		},
	}

	// Engine connection
	cmd.Flags().StringVarP(&flags.Engine, "engine", "e", flags.Engine, "Address of the rocketship engine")
	cmd.Flags().StringVar(&flags.Profile, "profile", "", "Use specific connection profile")

	// Display options
	cmd.Flags().StringVar(&flags.Format, "format", flags.Format, "Output format (table, json, yaml)")
	cmd.Flags().BoolVar(&flags.Logs, "logs", false, "Include logs from the test run")

	return cmd
}

func runGet(runID string, flags *GetFlags) error {
	// Connect to engine using profiles if available, fallback to engine address
	var client *EngineClient
	var err error
	
	engineFlagSet := flags.Engine != "localhost:7700" // Check if engine was explicitly set
	if !engineFlagSet && flags.Profile != "" {
		// Use profile-aware connection
		client, err = NewEngineClientWithProfile("", flags.Profile)
	} else if !engineFlagSet {
		// Use default profile-aware connection
		client, err = NewEngineClientWithProfile("", "")
	} else {
		// Use explicit engine address
		client, err = NewEngineClient(flags.Engine)
	}
	
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

	Logger.Debug("getting test run details", "run_id", runID)

	// Call GetRun
	resp, err := client.client.GetRun(ctx, &generated.GetRunRequest{
		RunId: runID,
	})
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	// Display results based on format
	switch flags.Format {
	case "table":
		return displayRunDetails(resp.Run, flags.Logs)
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

func displayRunDetails(run *generated.RunDetails, showLogs bool) error {
	if run == nil {
		return fmt.Errorf("run details not found")
	}

	// Print header
	fmt.Printf("Test Run Details\n")
	fmt.Printf("================\n\n")

	// Basic info
	fmt.Printf("Run ID:      %s\n", run.RunId)
	fmt.Printf("Suite Name:  %s\n", run.SuiteName)
	fmt.Printf("Status:      %s %s\n", getStatusIcon(run.Status), run.Status)

	// Timing info
	if run.StartedAt != "" {
		fmt.Printf("Started:     %s\n", run.StartedAt)
	}
	if run.EndedAt != "" {
		fmt.Printf("Ended:       %s\n", run.EndedAt)
	}
	if run.DurationMs > 0 {
		fmt.Printf("Duration:    %s\n", formatDuration(run.DurationMs))
	}

	// Context info
	if run.Context != nil {
		fmt.Printf("\nContext:\n")
		fmt.Printf("  Project ID:    %s\n", run.Context.ProjectId)
		fmt.Printf("  Source:        %s\n", run.Context.Source)
		if run.Context.Branch != "" {
			fmt.Printf("  Branch:        %s\n", run.Context.Branch)
		}
		if run.Context.CommitSha != "" {
			fmt.Printf("  Commit:        %s\n", run.Context.CommitSha)
		}
		if run.Context.Trigger != "" {
			fmt.Printf("  Trigger:       %s\n", run.Context.Trigger)
		}
		if run.Context.ScheduleName != "" {
			fmt.Printf("  Schedule:      %s\n", run.Context.ScheduleName)
		}
		if len(run.Context.Metadata) > 0 {
			fmt.Printf("  Metadata:\n")
			for k, v := range run.Context.Metadata {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	}

	// Test details
	if len(run.Tests) > 0 {
		fmt.Printf("\nTests (%d):\n", len(run.Tests))
		if err := displayTestsTable(run.Tests); err != nil {
			return err
		}
	}

	// TODO: Show logs if requested
	if showLogs {
		fmt.Printf("\n⚠️  Log streaming not implemented yet\n")
	}

	return nil
}

func displayTestsTable(tests []*generated.TestDetails) error {
	// Create a tabwriter for nice formatting
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {
		if err := w.Flush(); err != nil {
			Logger.Debug("failed to flush writer", "error", err)
		}
	}()

	// Print header
	if _, err := fmt.Fprintf(w, "  #\tNAME\tSTATUS\tDURATION\tERROR\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  -\t----\t------\t--------\t-----\n"); err != nil {
		return err
	}

	// Print each test
	for i, test := range tests {
		// Format duration
		duration := "N/A"
		if test.DurationMs > 0 {
			duration = formatDuration(test.DurationMs)
		}

		// Format error (truncate if too long)
		errorMsg := ""
		if test.ErrorMessage != "" {
			errorMsg = strings.ReplaceAll(test.ErrorMessage, "\n", " ")
			errorMsg = truncate(errorMsg, 40)
		}

		// Get status emoji
		statusIcon := getStatusIcon(test.Status)

		if _, err := fmt.Fprintf(w, "  %d\t%s\t%s %s\t%s\t%s\n",
			i+1,
			truncate(test.Name, 40),
			statusIcon,
			test.Status,
			duration,
			errorMsg,
		); err != nil {
			return err
		}
	}

	// Print summary
	var passed, failed, timeout int
	for _, test := range tests {
		switch test.Status {
		case "PASSED":
			passed++
		case "FAILED":
			failed++
		case "TIMEOUT":
			timeout++
		}
	}

	fmt.Printf("\nSummary: %d passed, %d failed, %d timeout out of %d total\n",
		passed, failed, timeout, len(tests))

	return nil
}