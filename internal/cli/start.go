package cli

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewStartCmd creates a new start command
func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new rocketship session",
		Long:  `Start a new rocketship session by connecting to a rocketship engine.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			engineAddr, err := cmd.Flags().GetString("engine")
			if err != nil {
				return err
			}

			if engineAddr == "" {
				return fmt.Errorf("engine address is required")
			}

			// TODO: Validate connection to engine

			session := &Session{
				EngineAddress: engineAddr,
				SessionID:     uuid.New().String(),
				CreatedAt:     time.Now(),
			}

			if err := SaveSession(session); err != nil {
				return fmt.Errorf("failed to save session: %w", err)
			}

			fmt.Printf("Session started successfully. Engine address: %s\n", engineAddr)
			return nil
		},
	}

	cmd.Flags().String("engine", "", "Address of the rocketship engine (e.g., localhost:8080)")
	return cmd
}
