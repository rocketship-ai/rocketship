package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var logsCmd = &cobra.Command{
	Use:   "logs [run-id]",
	Short: "Tail logs for a run",
	Long:  `Tail logs for a specific run.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runID := args[0]

		conn, err := grpc.NewClient("localhost:7700", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("failed to connect to Engine: %w", err)
		}
		defer func() { _ = conn.Close() }()

		client := generated.NewEngineClient(conn)

		ctx := context.Background()
		stream, err := client.StreamLogs(ctx, &generated.LogStreamRequest{
			RunId: runID,
		})
		if err != nil {
			return fmt.Errorf("failed to stream logs: %w", err)
		}

		for {
			logLine, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("error streaming logs: %w", err)
			}

			fmt.Printf("%s %s\n", logLine.Ts, logLine.Msg)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(logsCmd)
}
