package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
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
		NewTeamListCmd(),
		NewTeamShowCmd(),
		NewTeamAddMemberCmd(),
		NewTeamRemoveMemberCmd(),
		NewTeamAddRepoCmd(),
		NewTeamRemoveRepoCmd(),
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

	// Call the CreateTeam gRPC endpoint
	response, err := client.client.CreateTeam(ctx, &generated.CreateTeamRequest{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	fmt.Printf("%s Team '%s' created successfully\n", color.GreenString("✓"), name)
	fmt.Printf("Team ID: %s\n", response.TeamId)

	return nil
}

// runTeamAddMember handles adding a member to a team
func runTeamAddMember(ctx context.Context, teamName, email, roleStr string, permissionStrs []string) error {
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

	// Validate email format
	if !isValidEmail(email) {
		return fmt.Errorf("invalid email address: %s", email)
	}

	// Set role-appropriate default permissions if using defaults
	finalPermissions := permissionStrs
	if len(permissionStrs) == 1 && permissionStrs[0] == "tests:run" {
		// User didn't specify custom permissions, use role defaults
		switch roleStr {
		case "admin":
			finalPermissions = []string{
				"tests:run",
				"repositories:read",
				"repositories:write", 
				"repositories:manage",
				"team:members:read",
				"team:members:write",
				"team:members:manage",
			}
		case "member":
			finalPermissions = []string{
				"tests:run",
				"team:members:read",
				"repositories:read",  // NEW - can see all repositories
			}
		}
	}

	// Call the AddTeamMember gRPC endpoint
	response, err := client.client.AddTeamMember(ctx, &generated.AddTeamMemberRequest{
		TeamName:    teamName,
		Email:       email,
		Role:        roleStr,
		Permissions: finalPermissions,
	})
	if err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to add team member: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)
	fmt.Printf("Permissions: %v\n", finalPermissions)

	return nil
}

// runTeamAddRepo handles adding a repository to a team
func runTeamAddRepo(ctx context.Context, teamName, repoURL string) error {
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

	// Ask user if they want to enforce CODEOWNERS
	fmt.Printf("Do you want to enforce CODEOWNERS for this repository? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response) // Ignore error - user input handling
	enforceCodeowners := strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"

	// Call the AddTeamRepository gRPC endpoint
	response2, err := client.client.AddTeamRepository(ctx, &generated.AddTeamRepositoryRequest{
		TeamName:          teamName,
		RepositoryUrl:     repoURL,
		EnforceCodeowners: enforceCodeowners,
	})
	if err != nil {
		return fmt.Errorf("failed to add team repository: %w", err)
	}

	if !response2.Success {
		return fmt.Errorf("failed to add team repository: %s", response2.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response2.Message)
	if enforceCodeowners {
		fmt.Printf("  CODEOWNERS enforcement: %s\n", color.GreenString("enabled"))
	}

	return nil
}

// runTeamRemoveRepo handles removing a repository from a team
func runTeamRemoveRepo(ctx context.Context, teamName, repoURL string) error {
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

	// Call the RemoveTeamRepository gRPC endpoint
	response, err := client.client.RemoveTeamRepository(ctx, &generated.RemoveTeamRepositoryRequest{
		TeamName:      teamName,
		RepositoryUrl: repoURL,
	})
	if err != nil {
		return fmt.Errorf("failed to remove team repository: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to remove team repository: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)

	return nil
}

// runTeamList handles listing teams
func runTeamList(ctx context.Context) error {
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

	// Call the ListTeams gRPC endpoint
	response, err := client.client.ListTeams(ctx, &generated.ListTeamsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	if len(response.Teams) == 0 {
		fmt.Println("No teams found")
		return nil
	}

	fmt.Println("\nTeams:")
	fmt.Println(strings.Repeat("-", 60))
	
	for _, team := range response.Teams {
		fmt.Printf("• %s\n", color.CyanString(team.Name))
		fmt.Printf("  ID: %s\n", team.Id)
		fmt.Printf("  Created: %s\n", team.CreatedAt)
		
		// Show user's membership if they're in this team
		if team.UserRole != "" {
			fmt.Printf("  %s %s\n", 
				color.GreenString("Your role:"), 
				color.YellowString(team.UserRole))
		}
		
		// Show member and repository counts
		fmt.Printf("  Members: %d\n", team.MemberCount)
		fmt.Printf("  Repositories: %d\n", team.RepositoryCount)
		
		fmt.Println()
	}

	return nil
}

// NewTeamShowCmd creates a new team show command
func NewTeamShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <team>",
		Short: "Show detailed information about a team",
		Long:  `Show detailed information about a team including members and repositories`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamShow(cmd.Context(), args[0])
		},
	}
	return cmd
}

// NewTeamRemoveMemberCmd creates a new team remove-member command
func NewTeamRemoveMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-member <team> <email>",
		Short: "Remove a member from a team",
		Long:  `Remove a member from a team`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamRemoveMember(cmd.Context(), args[0], args[1])
		},
	}
	return cmd
}

func runTeamShow(ctx context.Context, teamName string) error {
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

	// Get access token
	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the GetTeam gRPC endpoint
	response, err := client.client.GetTeam(ctx, &generated.GetTeamRequest{
		TeamName: teamName,
	})
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}

	// Display team information
	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n%s\n", green("Team Details"))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("• %s: %s\n", blue("Name"), response.Team.Name)
	fmt.Printf("• %s: %s\n", blue("ID"), response.Team.Id)
	fmt.Printf("• %s: %s\n", blue("Created"), response.Team.CreatedAt)
	if response.Team.UserRole != "" {
		fmt.Printf("• %s: %s\n", blue("Your Role"), response.Team.UserRole)
	}

	fmt.Printf("\n%s (%d)\n", green("Members"), len(response.Members))
	fmt.Println(strings.Repeat("-", 60))
	if len(response.Members) == 0 {
		fmt.Println("No members found.")
	} else {
		for _, member := range response.Members {
			fmt.Printf("• %s (%s)\n", member.Email, yellow(member.Role))
			if len(member.Permissions) > 0 {
				fmt.Printf("  Permissions: %s\n", strings.Join(member.Permissions, ", "))
			}
			fmt.Printf("  Joined: %s\n", member.JoinedAt)
			fmt.Println()
		}
	}

	fmt.Printf("\n%s (%d)\n", green("Repositories"), len(response.Repositories))
	fmt.Println(strings.Repeat("-", 60))
	if len(response.Repositories) == 0 {
		fmt.Println("No repositories found.")
	} else {
		for _, repo := range response.Repositories {
			fmt.Printf("• %s\n", repo.RepositoryName)
			fmt.Printf("  URL: %s\n", repo.RepositoryUrl)
			fmt.Printf("  Added: %s\n", repo.AddedAt)
			fmt.Println()
		}
	}

	return nil
}

func runTeamRemoveMember(ctx context.Context, teamName, email string) error {
	// Validate email format
	if !isValidEmail(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}

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

	// Get access token
	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Call the RemoveTeamMember gRPC endpoint
	response, err := client.client.RemoveTeamMember(ctx, &generated.RemoveTeamMemberRequest{
		TeamName: teamName,
		Email:    email,
	})
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}

	if response.Success {
		green := color.New(color.FgGreen).SprintFunc()
		fmt.Printf("%s %s\n", green("✓"), response.Message)
	} else {
		return fmt.Errorf("failed to remove member: %s", response.Message)
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

