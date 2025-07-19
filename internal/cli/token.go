package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// NewTokenCmd creates a new token command
func NewTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "API token management commands",
		Long:  `Manage API tokens for CI/CD usage`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewTokenCreateCmd(),
		NewTokenListCmd(),
		NewTokenRevokeCmd(),
	)

	return cmd
}

// NewTokenCreateCmd creates a new token create command
func NewTokenCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new API token",
		Long:  `Create a new API token for CI/CD usage`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamName, _ := cmd.Flags().GetString("team")
			permissions, _ := cmd.Flags().GetStringSlice("permissions")
			expiresStr, _ := cmd.Flags().GetString("expires")
			
			var expiresAt *time.Time
			if expiresStr != "" {
				expires, err := time.Parse("2006-01-02", expiresStr)
				if err != nil {
					return fmt.Errorf("invalid expires date format (use YYYY-MM-DD): %w", err)
				}
				expiresAt = &expires
			}
			
			return runTokenCreate(cmd.Context(), args[0], teamName, permissions, expiresAt)
		},
	}

	cmd.Flags().String("team", "", "Team name for the token (required)")
	cmd.Flags().StringSlice("permissions", []string{"test_runs"}, "Permissions for the token")
	cmd.Flags().String("expires", "", "Expiration date (YYYY-MM-DD format, optional)")
	_ = cmd.MarkFlagRequired("team")

	return cmd
}

// NewTokenListCmd creates a new token list command
func NewTokenListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API tokens",
		Long:  `List all API tokens`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTokenList(cmd.Context())
		},
	}

	return cmd
}

// NewTokenRevokeCmd creates a new token revoke command
func NewTokenRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <token-id>",
		Short: "Revoke an API token",
		Long:  `Revoke an API token`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTokenRevoke(cmd.Context(), args[0])
		},
	}

	return cmd
}

// runTokenCreate handles token creation
func runTokenCreate(ctx context.Context, name, teamName string, permissionStrs []string, expiresAt *time.Time) error {
	// Check if authenticated
	if !isAuthenticatedToken(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	_, err := connectToDatabaseToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create repository (placeholder)
	var repo *rbac.Repository // TODO: Implement when database is available

	// Get team (placeholder)
	if repo == nil {
		return fmt.Errorf("authentication system not implemented")
	}
	// TODO: Implement team lookup when database is available

	// Parse permissions (placeholder - not used in current implementation)
	_ = permissionStrs // Avoid unused variable warning

	// TODO: Implement token creation when authentication system is available
	fmt.Printf("%s API token creation not implemented yet\n", color.YellowString("⚠"))
	fmt.Printf("Team: %s\n", teamName)
	fmt.Printf("Permissions: %v\n", permissionStrs)

	return nil
}

// runTokenList handles listing tokens
func runTokenList(ctx context.Context) error {
	// Check if authenticated
	if !isAuthenticatedToken(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	_, err := connectToDatabaseToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// TODO: Implement token listing when authentication system is available
	fmt.Printf("%s Token listing not implemented yet\n", color.YellowString("⚠"))

	return nil
}

// Helper functions (copied from team.go to avoid circular dependencies)

// isAuthenticated checks if user is authenticated
func isAuthenticatedToken(ctx context.Context) bool {
	// For now, just check if auth config exists
	_, err := getAuthConfig()
	return err == nil
}

// connectToDatabase connects to the database
func connectToDatabaseToken(ctx context.Context) (interface{}, error) {
	// Placeholder - authentication system not fully implemented
	return nil, fmt.Errorf("database connection not implemented")
}

// runTokenRevoke handles token revocation
func runTokenRevoke(ctx context.Context, tokenID string) error {
	// Check if authenticated
	if !isAuthenticatedToken(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	_, err := connectToDatabaseToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create repository (placeholder)
	var repo *rbac.Repository // TODO: Implement when database is available

	// TODO: Implement token revocation when authentication system is available
	if repo == nil {
		return fmt.Errorf("authentication system not implemented")
	}

	fmt.Printf("%s Token revocation not implemented yet\n", color.YellowString("⚠"))

	return nil
}