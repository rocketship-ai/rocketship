package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/database"
	"github.com/rocketship-ai/rocketship/internal/github"
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
		NewTeamRemoveRepoCmd(),
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
	cmd.Flags().StringSlice("permissions", []string{"tests:run"}, "Permissions for the member")

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

// NewTeamRemoveRepoCmd creates a new team remove-repo command
func NewTeamRemoveRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-repo <team> <repo-url>",
		Short: "Remove a repository from a team",
		Long:  `Remove a repository from a team`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamRemoveRepo(cmd.Context(), args[0], args[1])
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

	// Check if user can create teams (only org admins)
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionTeamsWrite); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

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

	// Check if user can manage team members
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersWrite); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	// Validate email format
	if !isValidEmail(email) {
		return fmt.Errorf("invalid email address: %s", email)
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

	// Parse permissions (simplified model)
	var permissions []rbac.Permission
	for _, permStr := range permissionStrs {
		switch permStr {
		// Test permissions (simplified)
		case "tests:run":
			permissions = append(permissions, rbac.PermissionTestsRun)
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
		// Test schedules
		case "test:schedules:manage":
			permissions = append(permissions, rbac.PermissionTestSchedulesManage)
		// Legacy mappings for backwards compatibility
		case "test_runs":
			permissions = append(permissions, rbac.PermissionTestsRun)
		case "repository_mgmt":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		default:
			return fmt.Errorf("invalid permission: %s. Valid permissions: tests:run, repositories:read, repositories:write, repositories:manage, team:members:read, team:members:write, team:members:manage, test:schedules:manage", permStr)
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

	// Check if user can manage repositories for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	// Create GitHub client and validation service
	githubClient, err := github.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	validationService := github.NewValidationService(githubClient, repo)

	// Validate repository and get metadata
	fmt.Printf("Validating repository: %s\n", repoURL)
	metadata, err := validationService.GetRepositoryMetadata(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to validate repository: %w", err)
	}

	fmt.Printf("Repository validated: %s (%s)\n", metadata.FullName, metadata.Description)
	if metadata.Private {
		fmt.Printf("  %s Private repository\n", color.YellowString("⚠"))
	}
	if metadata.HasCodeowners {
		fmt.Printf("  %s CODEOWNERS file detected\n", color.BlueString("ℹ"))
	}

	// Ask user if they want to enforce CODEOWNERS (if present)
	enforceCodeowners := false
	if metadata.HasCodeowners {
		fmt.Printf("\nDo you want to enforce CODEOWNERS for this repository? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response) // Ignore error - user input handling
		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			enforceCodeowners = true
			fmt.Printf("CODEOWNERS enforcement will be enabled\n")
		}
	}

	// Validate and create/update repository in database
	repository, err := validationService.ValidateAndCreateRepository(ctx, repoURL, enforceCodeowners)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Add team repository
	if err := repo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return fmt.Errorf("failed to add team repository: %w", err)
	}

	fmt.Printf("%s Added repository '%s' to team '%s'\n", color.GreenString("✓"), metadata.FullName, teamName)
	if enforceCodeowners {
		fmt.Printf("  CODEOWNERS enforcement: %s\n", color.GreenString("enabled"))
	}

	return nil
}

// runTeamRemoveRepo handles removing a repository from a team
func runTeamRemoveRepo(ctx context.Context, teamName, repoURL string) error {
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

	// Check if user can manage repositories for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	// Parse repository URL to get standard format
	owner, repoName, err := github.ParseRepositoryURL(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}
	
	// Construct the standard GitHub URL format
	standardURL := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)

	// Get repository from database
	repository, err := repo.GetRepository(ctx, standardURL)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return fmt.Errorf("repository not found in system: %s", standardURL)
	}

	// Remove team repository association
	if err := repo.RemoveTeamFromRepository(ctx, team.ID, repository.ID); err != nil {
		return fmt.Errorf("failed to remove repository from team: %w", err)
	}

	fmt.Printf("%s Removed repository '%s' from team '%s'\n", color.GreenString("✓"), standardURL, teamName)

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

	// Create repository
	repo := rbac.NewRepository(db)

	// Get current user to check their teams
	authConfig, _ := getAuthConfig()
	authClient, _ := auth.NewOIDCClient(ctx, authConfig)
	storage := auth.NewKeyringStorage()
	
	token, err := storage.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	
	userInfo, err := authClient.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	// Get all teams
	teams, err := repo.ListTeams(ctx)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	if len(teams) == 0 {
		fmt.Println("No teams found")
		return nil
	}

	// Get user's team memberships
	userTeams, err := repo.GetUserTeams(ctx, userInfo.Subject)
	if err != nil {
		// Non-fatal, just won't show membership info
		userTeams = []rbac.TeamMember{}
	}

	// Create a map for quick lookup
	userTeamMap := make(map[string]rbac.TeamMember)
	for _, tm := range userTeams {
		userTeamMap[tm.TeamID] = tm
	}

	fmt.Println("\nTeams:")
	fmt.Println(strings.Repeat("-", 60))
	
	for _, team := range teams {
		fmt.Printf("• %s\n", color.CyanString(team.Name))
		fmt.Printf("  ID: %s\n", team.ID)
		fmt.Printf("  Created: %s\n", team.CreatedAt.Format("2006-01-02 15:04:05"))
		
		// Show user's membership if they're in this team
		if membership, ok := userTeamMap[team.ID]; ok {
			fmt.Printf("  %s %s\n", 
				color.GreenString("Your role:"), 
				color.YellowString(string(membership.Role)))
		}
		
		// Get team member count
		members, err := repo.GetTeamMembers(ctx, team.ID)
		if err == nil {
			fmt.Printf("  Members: %d\n", len(members))
		}
		
		// Get assigned repositories count
		repos, err := repo.GetTeamRepositories(ctx, team.ID)
		if err == nil {
			fmt.Printf("  Repositories: %d\n", len(repos))
		}
		
		fmt.Println()
	}

	return nil
}

// Helper functions

// emailRegex is a regex pattern for basic email validation
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// isValidEmail validates an email address using regex
func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

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

// getAuthContext creates an AuthContext from the current authenticated user
func getAuthContext(ctx context.Context) (*rbac.AuthContext, error) {
	// Get authentication config and client
	config, err := getAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth config: %w", err)
	}
	
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC client: %w", err)
	}
	
	storage := auth.NewKeyringStorage()
	
	// Get token
	token, err := storage.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	
	// Get user info
	userInfo, err := client.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	
	// Connect to database to get user details
	db, err := connectToDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()
	
	repo := rbac.NewRepository(db)
	
	// Get or create user
	user, err := repo.GetOrCreateUserByEmail(ctx, userInfo.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	// Get user's team memberships
	teamMemberships, err := repo.GetUserTeams(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team memberships: %w", err)
	}
	
	return &rbac.AuthContext{
		UserID:          user.ID,
		Email:           user.Email,
		Name:            user.Name,
		OrgRole:         user.OrgRole,
		TeamMemberships: teamMemberships,
	}, nil
}