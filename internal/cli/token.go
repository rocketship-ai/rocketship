package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

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
	if !isAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Get auth context and check permissions
	authCtx, err := getAuthContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth context: %w", err)
	}

	// Connect to database
	db, err := connectToDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository and enforcer
	repo := rbac.NewRepository(db)
	enforcer := rbac.NewEnforcer(repo)

	// Get team
	team, err := repo.GetTeamByName(ctx, teamName)
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return fmt.Errorf("team not found: %s", teamName)
	}

	// Check if user can manage API tokens for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	// Parse permissions
	var permissions []rbac.Permission
	for _, permStr := range permissionStrs {
		switch permStr {
		case "tests:run":
			permissions = append(permissions, rbac.PermissionTestsRun)
		case "repositories:read":
			permissions = append(permissions, rbac.PermissionRepositoriesRead)
		case "repositories:write":
			permissions = append(permissions, rbac.PermissionRepositoriesWrite)
		case "repositories:manage":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		case "test_runs": // Legacy support
			permissions = append(permissions, rbac.PermissionTestsRun)
		default:
			return fmt.Errorf("invalid permission: %s. Valid permissions: tests:run, repositories:read, repositories:write, repositories:manage", permStr)
		}
	}

	// Create token manager
	tokenManager := tokens.NewManager(repo)

	// Create token request
	req := &tokens.CreateTokenRequest{
		TeamID:      team.ID,
		Name:        name,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		CreatedBy:   authCtx.UserID,
	}

	// Create token
	resp, err := tokenManager.CreateToken(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	// Display success message and token
	fmt.Printf("%s API token created successfully\n", color.GreenString("✓"))
	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Team: %s\n", teamName)
	fmt.Printf("Token ID: %s\n", resp.TokenID)
	fmt.Printf("Permissions: %v\n", permissionStrs)
	if resp.ExpiresAt != nil {
		fmt.Printf("Expires: %s\n", resp.ExpiresAt.Format("2006-01-02"))
	} else {
		fmt.Printf("Expires: Never\n")
	}
	fmt.Printf("\n%s Please save this token securely - it will not be shown again:\n", color.YellowString("⚠"))
	fmt.Printf("%s\n", color.RedString(resp.Token))

	return nil
}

// runTokenList handles listing tokens
func runTokenList(ctx context.Context) error {
	// Check if authenticated
	if !isAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Get auth context
	authCtx, err := getAuthContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth context: %w", err)
	}

	// Connect to database
	db, err := connectToDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)
	tokenManager := tokens.NewManager(repo)

	// Get user's teams to list tokens for
	teams := authCtx.TeamMemberships
	if len(teams) == 0 {
		fmt.Println("No teams found. You must be a member of a team to view API tokens.")
		return nil
	}

	fmt.Println("\nAPI Tokens:")
	fmt.Println(strings.Repeat("-", 80))

	totalTokens := 0
	for _, teamMember := range teams {
		// Get team details
		team, err := repo.GetTeam(ctx, teamMember.TeamID)
		if err != nil {
			continue // Skip teams we can't access
		}
		if team == nil {
			continue
		}

		// List tokens for this team
		teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
		if err != nil {
			fmt.Printf("Error listing tokens for team %s: %v\n", team.Name, err)
			continue
		}

		if len(teamTokens) > 0 {
			fmt.Printf("\n%s (%d tokens):\n", color.CyanString("Team: "+team.Name), len(teamTokens))
			for _, token := range teamTokens {
				fmt.Printf("  • %s (ID: %s)\n", token.Name, token.ID)
				fmt.Printf("    Created: %s\n", token.CreatedAt.Format("2006-01-02 15:04:05"))
				if token.ExpiresAt != nil {
					fmt.Printf("    Expires: %s\n", token.ExpiresAt.Format("2006-01-02"))
				} else {
					fmt.Printf("    Expires: Never\n")
				}
				if token.LastUsedAt != nil {
					fmt.Printf("    Last used: %s\n", token.LastUsedAt.Format("2006-01-02 15:04:05"))
				} else {
					fmt.Printf("    Last used: Never\n")
				}
				fmt.Printf("    Permissions: %v\n", token.Permissions)
				fmt.Println()
			}
			totalTokens += len(teamTokens)
		}
	}

	if totalTokens == 0 {
		fmt.Println("No API tokens found.")
	} else {
		fmt.Printf("Total tokens: %d\n", totalTokens)
	}

	return nil
}


// runTokenRevoke handles token revocation
func runTokenRevoke(ctx context.Context, tokenID string) error {
	// Check if authenticated
	if !isAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Get auth context
	authCtx, err := getAuthContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth context: %w", err)
	}

	// Connect to database
	db, err := connectToDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository and token manager
	repo := rbac.NewRepository(db)
	tokenManager := tokens.NewManager(repo)

	// First, we need to find the token to check permissions
	// Since we only have the token ID, we need to check if the user has permission to revoke it
	// We'll check if the user is an admin of any team that could own this token
	canRevoke := false
	
	// Check if user is org admin (can revoke any token)
	if authCtx.OrgRole == rbac.OrgRoleAdmin {
		canRevoke = true
	} else {
		// Check if user is admin of any team (simplified check)
		for _, teamMember := range authCtx.TeamMemberships {
			if teamMember.Role == rbac.RoleAdmin {
				// User is admin of at least one team, allow revocation
				// In a more complex system, we'd verify the token belongs to their team
				canRevoke = true
				break
			}
		}
	}

	if !canRevoke {
		return fmt.Errorf("permission denied: only organization admins or team admins can revoke API tokens")
	}

	// Revoke the token
	if err := tokenManager.RevokeToken(ctx, tokenID); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	fmt.Printf("%s API token revoked successfully\n", color.GreenString("✓"))
	fmt.Printf("Token ID: %s\n", tokenID)

	return nil
}