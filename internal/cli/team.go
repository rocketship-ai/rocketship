package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/database"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// NewTeamCmd creates a new team command
func NewTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Team management commands",
		Long:  `Manage teams and team members`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewTeamCreateCmd(),
		NewTeamAddMemberCmd(),
		NewTeamAddRepoCmd(),
		NewTeamListCmd(),
	)

	return cmd
}

// NewTeamCreateCmd creates a new team create command
func NewTeamCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new team",
		Long:  `Create a new team`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamCreate(cmd.Context(), args[0])
		},
	}

	return cmd
}

// NewTeamAddMemberCmd creates a new team add-member command
func NewTeamAddMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-member <team> <email>",
		Short: "Add a member to a team",
		Long:  `Add a member to a team`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			role, _ := cmd.Flags().GetString("role")
			permissions, _ := cmd.Flags().GetStringSlice("permissions")
			return runTeamAddMember(cmd.Context(), args[0], args[1], role, permissions)
		},
	}

	cmd.Flags().String("role", "member", "Role for the member (admin or member)")
	cmd.Flags().StringSlice("permissions", []string{"test_runs"}, "Permissions for the member")

	return cmd
}

// NewTeamAddRepoCmd creates a new team add-repo command
func NewTeamAddRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-repo <team> <repo-url>",
		Short: "Add a repository to a team",
		Long:  `Add a repository to a team`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamAddRepo(cmd.Context(), args[0], args[1])
		},
	}

	return cmd
}

// NewTeamListCmd creates a new team list command
func NewTeamListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List teams",
		Long:  `List all teams`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamList(cmd.Context())
		},
	}

	return cmd
}

// runTeamCreate handles team creation
func runTeamCreate(ctx context.Context, name string) error {
	// Check if authenticated and admin
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

	// Create team
	team := &rbac.Team{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
	}

	if err := repo.CreateTeam(ctx, team); err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	fmt.Printf("%s Team '%s' created successfully\n", color.GreenString("✓"), name)
	fmt.Printf("Team ID: %s\n", team.ID)

	return nil
}

// runTeamAddMember handles adding a member to a team
func runTeamAddMember(ctx context.Context, teamName, email, roleStr string, permissionStrs []string) error {
	// Check if authenticated and admin
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

	// Validate role
	var role rbac.Role
	switch roleStr {
	case "admin":
		role = rbac.RoleAdmin
	case "member":
		role = rbac.RoleMember
	default:
		return fmt.Errorf("invalid role: %s (must be 'admin' or 'member')", roleStr)
	}

	// Parse permissions (Buildkite-style)
	var permissions []rbac.Permission
	for _, permStr := range permissionStrs {
		switch permStr {
		// Test permissions
		case "tests:read":
			permissions = append(permissions, rbac.PermissionTestsRead)
		case "tests:write":
			permissions = append(permissions, rbac.PermissionTestsWrite)
		case "tests:manage":
			permissions = append(permissions, rbac.PermissionTestsManage)
		// Workflow permissions
		case "workflows:read":
			permissions = append(permissions, rbac.PermissionWorkflowsRead)
		case "workflows:write":
			permissions = append(permissions, rbac.PermissionWorkflowsWrite)
		case "workflows:manage":
			permissions = append(permissions, rbac.PermissionWorkflowsManage)
		// Repository permissions
		case "repositories:read":
			permissions = append(permissions, rbac.PermissionRepositoriesRead)
		case "repositories:write":
			permissions = append(permissions, rbac.PermissionRepositoriesWrite)
		case "repositories:manage":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		// Team member permissions (for team admins)
		case "team:members:read":
			permissions = append(permissions, rbac.PermissionTeamMembersRead)
		case "team:members:write":
			permissions = append(permissions, rbac.PermissionTeamMembersWrite)
		case "team:members:manage":
			permissions = append(permissions, rbac.PermissionTeamMembersManage)
		// Legacy mappings for backwards compatibility
		case "test_runs":
			permissions = append(permissions, rbac.PermissionTestsWrite)
		case "repository_mgmt":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		default:
			return fmt.Errorf("invalid permission: %s. Valid permissions: tests:read, tests:write, tests:manage, workflows:read, workflows:write, workflows:manage, repositories:read, repositories:write, repositories:manage, team:members:read, team:members:write, team:members:manage", permStr)
		}
	}

	// Get or create user by email
	user, err := repo.GetOrCreateUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get or create user: %w", err)
	}

	// Add team member
	if err := repo.AddTeamMember(ctx, team.ID, user.ID, role, permissions); err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	fmt.Printf("%s Added %s to team '%s' as %s\n", color.GreenString("✓"), email, teamName, role)
	fmt.Printf("Permissions: %v\n", permissionStrs)

	return nil
}

// runTeamAddRepo handles adding a repository to a team
func runTeamAddRepo(ctx context.Context, teamName, repoURL string) error {
	// Check if authenticated and admin
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

	// Get or create repository
	repository, err := repo.GetRepository(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		// Create new repository
		repository = &rbac.RepositoryEntity{
			ID:                   uuid.New().String(),
			URL:                  repoURL,
			EnforceCodeowners:    false,
			CreatedAt:            time.Now(),
		}
		if err := repo.CreateRepository(ctx, repository); err != nil {
			return fmt.Errorf("failed to create repository: %w", err)
		}
	}

	// Add team repository
	if err := repo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return fmt.Errorf("failed to add team repository: %w", err)
	}

	fmt.Printf("%s Added repository '%s' to team '%s'\n", color.GreenString("✓"), repoURL, teamName)

	return nil
}

// runTeamList handles listing teams
func runTeamList(ctx context.Context) error {
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
	fmt.Printf("Team listing not implemented yet\n")

	return nil
}

// Helper functions

// isAuthenticated checks if user is authenticated
func isAuthenticated(ctx context.Context) bool {
	// Create auth manager same way as auth commands
	config, err := getAuthConfig()
	if err != nil {
		return false
	}
	
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return false
	}
	
	storage := auth.NewKeyringStorage()
	manager := auth.NewManager(client, storage)
	
	// Use the manager's built-in authentication check
	return manager.IsAuthenticated(ctx)
}

// connectToDatabase connects to the database
func connectToDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	config := database.DefaultConfig()
	return database.Connect(ctx, config)
}