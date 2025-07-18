package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/rocketship-ai/rocketship/internal/database"
	"github.com/rocketship-ai/rocketship/internal/rbac"
	"github.com/rocketship-ai/rocketship/internal/tokens"
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
	cmd.MarkFlagRequired("team")

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
	if !isAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Get team
	team, err := repo.GetTeamByName(ctx, teamName)
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return fmt.Errorf("team not found: %s", teamName)
	}

	// Parse permissions
	var permissions []rbac.Permission
	for _, permStr := range permissionStrs {
		switch permStr {
		case "test_runs":
			permissions = append(permissions, rbac.PermissionTestRuns)
		case "repository_mgmt":
			permissions = append(permissions, rbac.PermissionRepositoryMgmt)
		case "team_mgmt":
			permissions = append(permissions, rbac.PermissionTeamMgmt)
		case "user_mgmt":
			permissions = append(permissions, rbac.PermissionUserMgmt)
		default:
			return fmt.Errorf("invalid permission: %s", permStr)
		}
	}

	// Create token manager
	tokenManager := tokens.NewManager(repo)

	// Create token
	req := &tokens.CreateTokenRequest{
		TeamID:      team.ID,
		Name:        name,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		CreatedBy:   "cli-user", // TODO: Get actual user ID
	}

	resp, err := tokenManager.CreateToken(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	// Display token
	fmt.Printf("%s API token created successfully\n", color.GreenString("✓"))
	fmt.Printf("Token ID: %s\n", resp.TokenID)
	fmt.Printf("Token: %s\n", color.YellowString(resp.Token))
	fmt.Printf("Team: %s\n", teamName)
	fmt.Printf("Permissions: %v\n", permissionStrs)
	if resp.ExpiresAt != nil {
		fmt.Printf("Expires: %s\n", resp.ExpiresAt.Format("2006-01-02"))
	} else {
		fmt.Printf("Expires: Never\n")
	}
	fmt.Printf("\n%s Store this token securely. It will not be shown again.\n", color.RedString("⚠"))

	return nil
}

// runTokenList handles listing tokens
func runTokenList(ctx context.Context) error {
	// Check if authenticated
	if !isAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// For now, just print a message
	fmt.Printf("Token listing not implemented yet\n")

	return nil
}

// runTokenRevoke handles token revocation
func runTokenRevoke(ctx context.Context, tokenID string) error {
	// Check if authenticated
	if !isAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Create token manager
	tokenManager := tokens.NewManager(repo)

	// Revoke token
	if err := tokenManager.RevokeToken(ctx, tokenID); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	fmt.Printf("%s Token revoked successfully\n", color.GreenString("✓"))

	return nil
}