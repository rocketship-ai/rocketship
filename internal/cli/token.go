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
			
			return runTokenCreate(cmd.Context(), args[0], teamName, permissions, expiresStr)
		},
	}

	cmd.Flags().String("team", "", "Team name for the token (required)")
	cmd.Flags().StringSlice("permissions", []string{"tests:run"}, "Permissions for the token")
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
			teamName, _ := cmd.Flags().GetString("team")
			return runTokenList(cmd.Context(), teamName)
		},
	}

	cmd.Flags().String("team", "", "Filter by team name (optional)")

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
func runTokenCreate(ctx context.Context, name, teamName string, permissions []string, expiresStr string) error {
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

	// Call the CreateToken gRPC endpoint
	response, err := client.client.CreateToken(ctx, &generated.CreateTokenRequest{
		TeamName:    teamName,
		Name:        name,
		Permissions: permissions,
		ExpiresAt:   expiresStr,
	})
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to create token: %s", response.Message)
	}

	// Display success message and token
	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)
	fmt.Printf("Token ID: %s\n", response.TokenId)
	fmt.Printf("Permissions: %v\n", response.Permissions)
	if response.ExpiresAt != "" {
		fmt.Printf("Expires: %s\n", response.ExpiresAt)
	} else {
		fmt.Printf("Expires: Never\n")
	}
	fmt.Printf("\n%s Please save this token securely - it will not be shown again:\n", color.YellowString("⚠"))
	fmt.Printf("%s\n", color.RedString(response.Token))

	return nil
}

// runTokenList handles listing tokens
func runTokenList(ctx context.Context, teamName string) error {
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

	// Call the ListTokens gRPC endpoint
	response, err := client.client.ListTokens(ctx, &generated.ListTokensRequest{
		TeamName: teamName,
	})
	if err != nil {
		return fmt.Errorf("failed to list tokens: %w", err)
	}

	if len(response.Tokens) == 0 {
		if teamName != "" {
			fmt.Printf("No API tokens found for team '%s'\n", teamName)
		} else {
			fmt.Println("No API tokens found")
		}
		return nil
	}

	fmt.Println("\nAPI Tokens:")
	fmt.Println(strings.Repeat("-", 80))

	// Group tokens by team for better display
	tokensByTeam := make(map[string][]*generated.ApiToken)
	for _, token := range response.Tokens {
		tokensByTeam[token.TeamName] = append(tokensByTeam[token.TeamName], token)
	}

	totalTokens := 0
	for teamDisplayName, teamTokens := range tokensByTeam {
		fmt.Printf("\n%s (%d tokens):\n", color.CyanString("Team: "+teamDisplayName), len(teamTokens))
		for _, token := range teamTokens {
			fmt.Printf("  • %s (ID: %s)\n", token.Name, token.Id)
			fmt.Printf("    Created: %s\n", token.CreatedAt)
			if token.ExpiresAt != "" {
				fmt.Printf("    Expires: %s\n", token.ExpiresAt)
			} else {
				fmt.Printf("    Expires: Never\n")
			}
			if token.LastUsedAt != "" {
				fmt.Printf("    Last used: %s\n", token.LastUsedAt)
			} else {
				fmt.Printf("    Last used: Never\n")
			}
			fmt.Printf("    Permissions: %v\n", token.Permissions)
			fmt.Println()
		}
		totalTokens += len(teamTokens)
	}

	fmt.Printf("Total tokens: %d\n", totalTokens)

	return nil
}

// runTokenRevoke handles token revocation
func runTokenRevoke(ctx context.Context, tokenID string) error {
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

	// Call the RevokeToken gRPC endpoint
	response, err := client.client.RevokeToken(ctx, &generated.RevokeTokenRequest{
		TokenId: tokenID,
	})
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to revoke token: %s", response.Message)
	}

	fmt.Printf("%s %s\n", color.GreenString("✓"), response.Message)

	return nil
}