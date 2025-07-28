package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
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
	// Load CLI config to get active profile
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config", "error", err)
		return fmt.Errorf("failed to load CLI config: %w", err)
	}
	
	profile := cliConfig.GetActiveProfile()
	
	// Create authenticated gRPC client to engine
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get access token for authentication
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured")
	}

	// Create OIDC client and auth manager
	oidcClient, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
	manager := auth.NewManager(oidcClient, storage)

	if !manager.IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the AddRepository gRPC endpoint
	response, err := client.client.AddRepository(ctx, &generated.AddRepositoryRequest{
		RepositoryUrl:     repoURL,
		EnforceCodeowners: enforceCodeowners,
	})
	if err != nil {
		return fmt.Errorf("failed to add repository: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to add repository: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)
	fmt.Printf("Repository ID: %s\n", response.RepositoryId)
	if enforceCodeowners {
		fmt.Printf("CODEOWNERS enforcement: %s\n", color.YellowString("enabled"))
	}

	return nil
}

// runRepoList handles listing repositories
func runRepoList(ctx context.Context) error {
	// Load CLI config to get active profile
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config", "error", err)
		return fmt.Errorf("failed to load CLI config: %w", err)
	}
	
	profile := cliConfig.GetActiveProfile()
	
	// Create authenticated gRPC client to engine
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get access token for authentication
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured")
	}

	// Create OIDC client and auth manager
	oidcClient, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
	manager := auth.NewManager(oidcClient, storage)

	if !manager.IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the ListRepositories gRPC endpoint
	response, err := client.client.ListRepositories(ctx, &generated.ListRepositoriesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(response.Repositories) == 0 {
		fmt.Println("No repositories found")
		return nil
	}

	fmt.Printf("\nRepositories (%d):\n", len(response.Repositories))
	fmt.Println(strings.Repeat("-", 60))
	
	for _, repo := range response.Repositories {
		fmt.Printf("\n• %s\n", color.CyanString(repo.Url))
		fmt.Printf("  ID: %s\n", repo.Id)
		fmt.Printf("  CODEOWNERS: %s\n", formatBool(repo.EnforceCodeowners))
		fmt.Printf("  Created: %s\n", repo.CreatedAt)

		if repo.TeamCount == 0 {
			fmt.Printf("  Teams: %s\n", color.YellowString("none"))
		} else {
			fmt.Printf("  Teams (%d): %s\n", repo.TeamCount, strings.Join(repo.TeamNames, ", "))
		}
	}

	return nil
}

// runRepoAssign handles assigning a team to a repository
func runRepoAssign(ctx context.Context, repoURL, teamName string) error {
	// Load CLI config to get active profile
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config", "error", err)
		return fmt.Errorf("failed to load CLI config: %w", err)
	}
	
	profile := cliConfig.GetActiveProfile()
	
	// Create authenticated gRPC client to engine
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get access token for authentication
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured")
	}

	// Create OIDC client and auth manager
	oidcClient, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
	manager := auth.NewManager(oidcClient, storage)

	if !manager.IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the AssignTeamToRepository gRPC endpoint
	response, err := client.client.AssignTeamToRepository(ctx, &generated.AssignTeamToRepositoryRequest{
		RepositoryUrl: repoURL,
		TeamName:      teamName,
	})
	if err != nil {
		return fmt.Errorf("failed to assign team to repository: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to assign team: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)

	return nil
}

// runRepoUnassign handles removing a team from a repository
func runRepoUnassign(ctx context.Context, repoURL, teamName string) error {
	// Load CLI config to get active profile
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config", "error", err)
		return fmt.Errorf("failed to load CLI config: %w", err)
	}
	
	profile := cliConfig.GetActiveProfile()
	
	// Create authenticated gRPC client to engine
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get access token for authentication
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured")
	}

	// Create OIDC client and auth manager
	oidcClient, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
	manager := auth.NewManager(oidcClient, storage)

	if !manager.IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the UnassignTeamFromRepository gRPC endpoint
	response, err := client.client.UnassignTeamFromRepository(ctx, &generated.UnassignTeamFromRepositoryRequest{
		RepositoryUrl: repoURL,
		TeamName:      teamName,
	})
	if err != nil {
		return fmt.Errorf("failed to unassign team from repository: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to unassign team: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)

	return nil
}

// runRepoRemove handles removing a repository
func runRepoRemove(ctx context.Context, repoURL string) error {
	// Load CLI config to get active profile
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config", "error", err)
		return fmt.Errorf("failed to load CLI config: %w", err)
	}
	
	profile := cliConfig.GetActiveProfile()
	
	// Create authenticated gRPC client to engine
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get access token for authentication
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured")
	}

	// Create OIDC client and auth manager
	oidcClient, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
	manager := auth.NewManager(oidcClient, storage)

	if !manager.IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the RemoveRepository gRPC endpoint
	response, err := client.client.RemoveRepository(ctx, &generated.RemoveRepositoryRequest{
		RepositoryUrl: repoURL,
	})
	if err != nil {
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to remove repository: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)

	return nil
}

// runRepoShow handles showing repository details
func runRepoShow(ctx context.Context, repoURL string) error {
	// Load CLI config to get active profile
	cliConfig, err := LoadConfig()
	if err != nil {
		Logger.Debug("failed to load CLI config", "error", err)
		return fmt.Errorf("failed to load CLI config: %w", err)
	}
	
	profile := cliConfig.GetActiveProfile()
	
	// Create authenticated gRPC client to engine
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get access token for authentication
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured")
	}

	// Create OIDC client and auth manager
	oidcClient, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
	manager := auth.NewManager(oidcClient, storage)

	if !manager.IsAuthenticated(ctx) {
		return fmt.Errorf("authentication required. Run 'rocketship auth login'")
	}

	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the GetRepository gRPC endpoint
	response, err := client.client.GetRepository(ctx, &generated.GetRepositoryRequest{
		RepositoryUrl: repoURL,
	})
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Display repository details
	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()
	
	fmt.Printf("\n%s\n", green("Repository Details"))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("• %s: %s\n", blue("URL"), response.Repository.Url)
	fmt.Printf("• %s: %s\n", blue("ID"), response.Repository.Id)
	fmt.Printf("• %s: %s\n", blue("CODEOWNERS"), formatBool(response.Repository.EnforceCodeowners))
	fmt.Printf("• %s: %s\n", blue("Created"), response.Repository.CreatedAt)

	fmt.Printf("\n%s (%d)\n", green("Assigned Teams"), len(response.Teams))
	fmt.Println(strings.Repeat("-", 60))
	if len(response.Teams) == 0 {
		fmt.Printf("No teams assigned.\n")
	} else {
		for _, team := range response.Teams {
			fmt.Printf("• %s (ID: %s)\n", team.Name, team.Id)
		}
	}

	return nil
}

// Helper functions

// formatBool formats boolean for display
func formatBool(b bool) string {
	if b {
		return color.GreenString("enabled")
	}
	return color.RedString("disabled")
}