package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List recent runs",
	Long:  `List the last 10 runs and their status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := grpc.NewClient("localhost:7700", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("failed to connect to Engine: %w", err)
		}
		defer func() { _ = conn.Close() }()

		client := generated.NewEngineClient(conn)

		ctx := context.Background()
		resp, err := client.ListRuns(ctx, &generated.ListRunsRequest{})
		if err != nil {
			return fmt.Errorf("failed to list runs: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "RUN ID\tSTATUS\tSTARTED\tENDED"); err != nil {
			return err
		}

		for _, run := range resp.Runs {
			startedAt, _ := time.Parse(time.RFC3339, run.StartedAt)
			startedStr := startedAt.Format("2006-01-02 15:04:05")

			endedStr := ""
			if run.EndedAt != "" {
				endedAt, _ := time.Parse(time.RFC3339, run.EndedAt)
				endedStr = endedAt.Format("2006-01-02 15:04:05")
			}

			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", run.RunId, run.Status, startedStr, endedStr); err != nil {
				return err
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
