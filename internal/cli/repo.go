package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/database"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// NewRepoCmd creates a new repository command
func NewRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository management commands",
		Long:  `Manage repositories and their team assignments`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewRepoAddCmd(),
		NewRepoListCmd(),
		NewRepoAssignCmd(),
		NewRepoUnassignCmd(),
		NewRepoRemoveCmd(),
		NewRepoShowCmd(),
	)

	return cmd
}

// NewRepoAddCmd creates a new repo add command
func NewRepoAddCmd() *cobra.Command {
	var enforceCodeowners bool

	cmd := &cobra.Command{
		Use:   "add <repository-url>",
		Short: "Add a repository to the system",
		Long:  `Add a repository to the system for team management and test execution`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoAdd(cmd.Context(), args[0], enforceCodeowners)
		},
	}

	cmd.Flags().BoolVar(&enforceCodeowners, "enforce-codeowners", false, "Enable CODEOWNERS file enforcement")

	return cmd
}

// NewRepoListCmd creates a new repo list command
func NewRepoListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all repositories",
		Long:  `List all repositories in the system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoList(cmd.Context())
		},
	}

	return cmd
}

// NewRepoAssignCmd creates a new repo assign command
func NewRepoAssignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <repository-url> <team-name>",
		Short: "Assign a team to a repository",
		Long:  `Assign a team to manage a repository`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoAssign(cmd.Context(), args[0], args[1])
		},
	}

	return cmd
}

// NewRepoUnassignCmd creates a new repo unassign command
func NewRepoUnassignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unassign <repository-url> <team-name>",
		Short: "Remove a team from a repository",
		Long:  `Remove a team's access to a repository`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoUnassign(cmd.Context(), args[0], args[1])
		},
	}

	return cmd
}

// NewRepoRemoveCmd creates a new repo remove command
func NewRepoRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <repository-url>",
		Short: "Remove a repository from the system",
		Long:  `Remove a repository and all its team assignments`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoRemove(cmd.Context(), args[0])
		},
	}

	return cmd
}

// NewRepoShowCmd creates a new repo show command
func NewRepoShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <repository-url>",
		Short: "Show repository details",
		Long:  `Show repository details including assigned teams`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoShow(cmd.Context(), args[0])
		},
	}

	return cmd
}

// runRepoAdd handles adding a repository
func runRepoAdd(ctx context.Context, repoURL string, enforceCodeowners bool) error {
	// Check if authenticated and admin
	if !isRepoAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Validate repository URL
	if err := validateRepositoryURL(repoURL); err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	// Connect to database
	db, err := connectToRepoDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Check if repository already exists
	existing, err := repo.GetRepository(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to check existing repository: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("repository already exists: %s", repoURL)
	}

	// Create new repository entity
	repoEntity := &rbac.RepositoryEntity{
		ID:                uuid.New().String(),
		URL:               repoURL,
		EnforceCodeowners: enforceCodeowners,
		CreatedAt:         time.Now(),
	}

	// Create repository
	if err := repo.CreateRepository(ctx, repoEntity); err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	fmt.Printf("%s Repository '%s' added successfully\n", color.GreenString("✓"), repoURL)
	fmt.Printf("Repository ID: %s\n", repoEntity.ID)
	if enforceCodeowners {
		fmt.Printf("CODEOWNERS enforcement: %s\n", color.YellowString("enabled"))
	}

	return nil
}

// runRepoList handles listing repositories
func runRepoList(ctx context.Context) error {
	// Check if authenticated
	if !isRepoAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToRepoDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// List repositories
	repositories, err := repo.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(repositories) == 0 {
		fmt.Println("No repositories found")
		return nil
	}

	fmt.Printf("\nRepositories (%d):\n", len(repositories))
	for _, r := range repositories {
		fmt.Printf("\n• %s\n", color.CyanString(r.URL))
		fmt.Printf("  ID: %s\n", r.ID)
		fmt.Printf("  CODEOWNERS: %s\n", formatBool(r.EnforceCodeowners))
		fmt.Printf("  Created: %s\n", r.CreatedAt.Format(time.RFC3339))

		// Get assigned teams
		teams, err := repo.GetRepositoryTeamsDetailed(ctx, r.ID)
		if err != nil {
			fmt.Printf("  Teams: %s\n", color.RedString("error fetching teams"))
		} else if len(teams) == 0 {
			fmt.Printf("  Teams: %s\n", color.YellowString("none"))
		} else {
			teamNames := make([]string, len(teams))
			for i, team := range teams {
				teamNames[i] = team.Name
			}
			fmt.Printf("  Teams: %s\n", strings.Join(teamNames, ", "))
		}
	}

	return nil
}

// runRepoAssign handles assigning a team to a repository
func runRepoAssign(ctx context.Context, repoURL, teamName string) error {
	// Check if authenticated and admin
	if !isRepoAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToRepoDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Get repository
	repository, err := repo.GetRepository(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return fmt.Errorf("repository not found: %s", repoURL)
	}

	// Get team
	team, err := repo.GetTeamByName(ctx, teamName)
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return fmt.Errorf("team not found: %s", teamName)
	}

	// Assign team to repository
	if err := repo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return fmt.Errorf("failed to assign team to repository: %w", err)
	}

	fmt.Printf("%s Team '%s' assigned to repository '%s'\n", 
		color.GreenString("✓"), teamName, repoURL)

	return nil
}

// runRepoUnassign handles removing a team from a repository
func runRepoUnassign(ctx context.Context, repoURL, teamName string) error {
	// Check if authenticated and admin
	if !isRepoAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToRepoDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Get repository
	repository, err := repo.GetRepository(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return fmt.Errorf("repository not found: %s", repoURL)
	}

	// Get team
	team, err := repo.GetTeamByName(ctx, teamName)
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return fmt.Errorf("team not found: %s", teamName)
	}

	// Remove team from repository
	if err := repo.RemoveTeamFromRepository(ctx, team.ID, repository.ID); err != nil {
		return fmt.Errorf("failed to remove team from repository: %w", err)
	}

	fmt.Printf("%s Team '%s' removed from repository '%s'\n", 
		color.GreenString("✓"), teamName, repoURL)

	return nil
}

// runRepoRemove handles removing a repository
func runRepoRemove(ctx context.Context, repoURL string) error {
	// Check if authenticated and admin
	if !isRepoAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToRepoDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Get repository
	repository, err := repo.GetRepository(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return fmt.Errorf("repository not found: %s", repoURL)
	}

	// Delete repository (cascade will remove team assignments)
	if err := repo.DeleteRepository(ctx, repository.ID); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	fmt.Printf("%s Repository '%s' removed successfully\n", 
		color.GreenString("✓"), repoURL)

	return nil
}

// runRepoShow handles showing repository details
func runRepoShow(ctx context.Context, repoURL string) error {
	// Check if authenticated
	if !isRepoAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	// Connect to database
	db, err := connectToRepoDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository
	repo := rbac.NewRepository(db)

	// Get repository
	repository, err := repo.GetRepository(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return fmt.Errorf("repository not found: %s", repoURL)
	}

	// Get assigned teams
	teams, err := repo.GetRepositoryTeamsDetailed(ctx, repository.ID)
	if err != nil {
		return fmt.Errorf("failed to get repository teams: %w", err)
	}

	// Display repository details
	fmt.Printf("\n%s\n", color.CyanString("Repository Details"))
	fmt.Printf("URL: %s\n", repository.URL)
	fmt.Printf("ID: %s\n", repository.ID)
	fmt.Printf("CODEOWNERS Enforcement: %s\n", formatBool(repository.EnforceCodeowners))
	fmt.Printf("Created: %s\n", repository.CreatedAt.Format(time.RFC3339))

	if repository.CodeownersCachedAt != nil {
		fmt.Printf("CODEOWNERS Cache: %s\n", repository.CodeownersCachedAt.Format(time.RFC3339))
	}

	fmt.Printf("\n%s (%d):\n", color.CyanString("Assigned Teams"), len(teams))
	if len(teams) == 0 {
		fmt.Printf("  %s\n", color.YellowString("No teams assigned"))
	} else {
		for _, team := range teams {
			fmt.Printf("• %s (ID: %s)\n", team.Name, team.ID)
		}
	}

	return nil
}

// Helper functions

// validateRepositoryURL validates a repository URL
func validateRepositoryURL(repoURL string) error {
	// Parse URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	// Check if it looks like a GitHub URL
	if !strings.Contains(u.Host, "github") {
		return fmt.Errorf("currently only GitHub repositories are supported")
	}

	// Check path format
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 {
		return fmt.Errorf("URL must include owner and repository name")
	}

	return nil
}

// isRepoAuthenticated checks if user is authenticated (reusing team auth logic)
func isRepoAuthenticated(ctx context.Context) bool {
	return isRepoAuthenticatedWithProfile(ctx, "")
}

func isRepoAuthenticatedWithProfile(ctx context.Context, profileName string) bool {
	// Load CLI config
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config, creating default", "error", err)
		cliConfig = DefaultConfig()
	}
	
	// Determine which profile to use
	var profile *Profile
	if profileName != "" {
		// Use specified profile
		profile, err = cliConfig.GetProfile(profileName)
		if err != nil {
			return false
		}
	} else {
		// Use active profile
		profile = cliConfig.GetActiveProfile()
	}
	
	// Get auth configuration from profile
	config := getProfileAuthConfig(profile)
	if config == nil {
		return false
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return false
	}

	// Create keyring storage for this profile
	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)

	// Create auth manager
	manager := auth.NewManager(client, storage)
	
	// Use the manager's built-in authentication check
	return manager.IsAuthenticated(ctx)
}

// connectToRepoDatabase connects to the database (reusing team database logic)
func connectToRepoDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	config := database.DefaultConfig()
	return database.Connect(ctx, config)
}

// formatBool formats boolean for display
func formatBool(b bool) string {
	if b {
		return color.GreenString("enabled")
	}
	return color.RedString("disabled")
}