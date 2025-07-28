package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// NewAuthCmd creates a new auth command
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long:  `Manage authentication for Rocketship`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewAuthLoginCmd(),
		NewAuthLogoutCmd(),
		NewAuthStatusCmd(),
	)

	return cmd
}

// NewAuthLoginCmd creates a new login command
func NewAuthLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Rocketship",
		Long: `Login to Rocketship using OIDC authentication.

If no connection profile is configured, this will automatically connect to the
Rocketship cloud service at https://app.rocketship.sh.

Examples:
  rocketship auth login                    # Login to cloud (app.rocketship.sh)
  rocketship auth login --profile enterprise # Login to specific profile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			return runAuthLogin(cmd.Context(), profileName)
		},
	}

	cmd.Flags().String("profile", "", "Use specific connection profile")
	return cmd
}

// NewAuthLogoutCmd creates a new logout command
func NewAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout from Rocketship",
		Long:  `Logout from Rocketship and remove stored tokens`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			return runAuthLogout(cmd.Context(), profileName)
		},
	}

	cmd.Flags().String("profile", "", "Use specific connection profile")
	return cmd
}

// NewAuthStatusCmd creates a new status command
func NewAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Long:  `Show current authentication status and user information`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			return runAuthStatus(cmd.Context(), profileName)
		},
	}

	cmd.Flags().String("profile", "", "Use specific connection profile")
	return cmd
}

// runAuthLogin handles the login flow
func runAuthLogin(ctx context.Context, profileName string) error {
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
			return fmt.Errorf("profile '%s' not found: %w", profileName, err)
		}
	} else {
		// Check if we have any configured profiles besides local
		if len(cliConfig.Profiles) == 1 {
			if _, hasLocal := cliConfig.Profiles["local"]; hasLocal {
				// Only have local profile, auto-create and use cloud profile
				fmt.Println("🌐 No connection configured. Connecting to Rocketship Cloud...")
				cliConfig.CreateCloudProfile()
				cliConfig.DefaultProfile = "cloud"
				if err := cliConfig.SaveConfig(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}
				fmt.Println("✅ Connected to https://app.rocketship.sh")
			}
		}
		
		// Use active profile
		profile = cliConfig.GetActiveProfile()
	}
	
	// First try to get auth configuration from server
	authConfig, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		// Fall back to profile configuration
		authConfig = getProfileAuthConfig(profile)
	}
	
	if authConfig == nil {
		return fmt.Errorf("authentication not configured for profile '%s'\n\nTo set up authentication:\n  1. Configure OIDC settings in your profile\n  2. Or connect to Rocketship Cloud: rocketship connect https://app.rocketship.sh", profile.Name)
	}

	// If auth config came from environment variables and profile doesn't have valid auth configured,
	// save the configuration to the profile for persistence across sessions
	if profile.Auth.Issuer == "" || profile.Auth.ClientID == "" {
		Logger.Debug("saving auth configuration from environment to profile", "profile", profile.Name, "issuer", authConfig.IssuerURL, "clientID", authConfig.ClientID)
		
		// Update the profile in the config map directly (since profile is a copy)
		profileInConfig := cliConfig.Profiles[profile.Name]
		profileInConfig.Auth.Issuer = authConfig.IssuerURL
		profileInConfig.Auth.ClientID = authConfig.ClientID
		profileInConfig.Auth.ClientSecret = authConfig.ClientSecret
		profileInConfig.Auth.AdminEmails = authConfig.AdminEmails
		cliConfig.Profiles[profile.Name] = profileInConfig
		
		// Save the updated profile configuration
		if err := cliConfig.SaveConfig(); err != nil {
			Logger.Warn("failed to save auth configuration to profile", "error", err)
			// Continue anyway - authentication will still work for this session
		} else {
			Logger.Debug("successfully saved auth configuration to profile", "profile", profile.Name)
		}
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	// Create keyring storage for this profile
	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)

	// Create auth manager
	manager := auth.NewManager(client, storage)

	// Check if already authenticated
	if manager.IsAuthenticated(ctx) {
		userInfo, err := manager.GetCurrentUser(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		fmt.Printf("Already authenticated as %s (%s)\n", userInfo.Name, userInfo.Email)
		return nil
	}

	// Start callback server
	callbackServer := auth.NewCallbackServer(8000)
	if err := callbackServer.Start(); err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer func() { _ = callbackServer.Shutdown() }()

	// Generate auth URL
	state := generateState()
	authURL, pkceChallenge, err := manager.GetAuthURL(state)
	if err != nil {
		return fmt.Errorf("failed to generate auth URL: %w", err)
	}

	// Print instructions
	fmt.Printf("Please visit the following URL to authenticate:\n\n")
	fmt.Printf("  %s\n\n", color.BlueString(authURL))
	fmt.Printf("Waiting for authentication... (timeout: 5 minutes)\n")

	// Wait for callback
	result := callbackServer.WaitForCallback(5 * time.Minute)
	if result.Error != nil {
		return fmt.Errorf("authentication failed: %w", result.Error)
	}

	// Verify state
	if result.State != state {
		return fmt.Errorf("invalid state parameter")
	}

	// Handle callback
	userInfo, err := manager.HandleCallback(ctx, result.Code, pkceChallenge.CodeVerifier)
	if err != nil {
		return fmt.Errorf("failed to handle callback: %w", err)
	}

	// Success
	fmt.Printf("\n%s Authentication successful!\n", color.GreenString("✓"))
	fmt.Printf("Welcome, %s (%s)\n", userInfo.Name, userInfo.Email)
	if userInfo.OrgRole == rbac.OrgRoleAdmin {
		fmt.Printf("Admin role: %s\n", color.YellowString("Yes"))
	}

	return nil
}

// runAuthLogout handles the logout flow
func runAuthLogout(ctx context.Context, profileName string) error {
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
			return fmt.Errorf("profile '%s' not found: %w", profileName, err)
		}
	} else {
		// Use active profile
		profile = cliConfig.GetActiveProfile()
	}
	
	// Get auth configuration from profile
	config := getProfileAuthConfig(profile)
	if config == nil {
		// If not configured, just try to clear local storage for this profile
		keyringKey := GetProfileKeyringKey(profile.Name)
		storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)
		if err := storage.DeleteToken(ctx); err != nil {
			return fmt.Errorf("failed to clear local tokens: %w", err)
		}
		fmt.Printf("Logged out (local tokens cleared)\n")
		return nil
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	// Create keyring storage for this profile
	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)

	// Create auth manager
	manager := auth.NewManager(client, storage)

	// Logout
	if err := manager.Logout(ctx); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	fmt.Printf("%s Logged out successfully\n", color.GreenString("✓"))
	return nil
}

// runAuthStatus shows the current authentication status
func runAuthStatus(ctx context.Context, profileName string) error {
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
			return fmt.Errorf("profile '%s' not found: %w", profileName, err)
		}
	} else {
		// Use active profile
		profile = cliConfig.GetActiveProfile()
	}
	
	// First try to get auth configuration from server
	config, err := getAuthConfigFromServer(profile)
	if err != nil {
		Logger.Debug("failed to fetch auth config from server, falling back to profile config", "error", err)
		// Fall back to profile configuration
		config = getProfileAuthConfig(profile)
	}
	
	if config == nil {
		fmt.Printf("Status: %s (authentication not configured)\n", color.RedString("Not authenticated"))
		return nil
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	// Create keyring storage for this profile
	keyringKey := GetProfileKeyringKey(profile.Name)
	storage := auth.NewKeyringStorageWithKey("rocketship", keyringKey)

	// Create auth manager
	manager := auth.NewManager(client, storage)

	// Check authentication status
	if !manager.IsAuthenticated(ctx) {
		fmt.Printf("Status: %s\n", color.RedString("Not authenticated"))
		fmt.Printf("Run 'rocketship auth login' to authenticate\n")
		return nil
	}

	// Get user info from local token (for display)
	userInfo, err := manager.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Get SERVER-DETERMINED user info (for accurate role)
	serverUserInfo, err := getServerUserInfo(profile, manager)
	if err != nil {
		Logger.Debug("failed to get server user info, using local token data", "error", err)
		// Fall back to local token data
		serverUserInfo = nil
	}

	// Display status
	fmt.Printf("Status: %s\n", color.GreenString("Authenticated"))
	fmt.Printf("User: %s (%s)\n", userInfo.Name, userInfo.Email)
	fmt.Printf("Subject: %s\n", userInfo.Subject)
	
	// Use server-determined role if available, otherwise fall back to local
	var isAdmin bool
	if serverUserInfo != nil {
		isAdmin = serverUserInfo.OrgRole == string(rbac.OrgRoleAdmin)
		Logger.Debug("using server-determined role", "role", serverUserInfo.OrgRole)
	} else {
		isAdmin = userInfo.OrgRole == rbac.OrgRoleAdmin
		Logger.Debug("using local token role", "role", userInfo.OrgRole)
	}
	
	if isAdmin {
		fmt.Printf("Admin role: %s\n", color.YellowString("Yes"))
	} else {
		fmt.Printf("Admin role: No\n")
	}

	// Show groups if available
	if serverUserInfo != nil && len(serverUserInfo.Groups) > 0 {
		fmt.Printf("Groups: %s\n", strings.Join(serverUserInfo.Groups, ", "))
	} else if len(userInfo.Groups) > 0 {
		fmt.Printf("Groups: %s\n", strings.Join(userInfo.Groups, ", "))
	}

	return nil
}

// generateState generates a random state parameter for OAuth
func generateState() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// getAuthConfigFromServer fetches authentication configuration from the server
func getAuthConfigFromServer(profile *Profile) (*auth.AuthConfig, error) {
	Logger.Debug("fetching auth config from server", "profile", profile.Name, "engine", profile.EngineAddress)
	
	// Create gRPC client to the engine using the profile
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()
	
	// Call the auth discovery endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	response, err := client.client.GetAuthConfig(ctx, &generated.GetAuthConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get auth config from server: %w", err)
	}
	
	Logger.Debug("received auth config from server", "auth_enabled", response.AuthEnabled)
	
	// If authentication is not enabled on the server, return nil
	if !response.AuthEnabled {
		Logger.Debug("authentication not enabled on server")
		return nil, nil
	}
	
	// If no OIDC config provided, return error
	if response.Oidc == nil {
		return nil, fmt.Errorf("server has auth enabled but no OIDC configuration provided")
	}
	
	Logger.Debug("server provided OIDC config", "issuer", response.Oidc.Issuer, "client_id", response.Oidc.ClientId)
	
	// Convert server response to AuthConfig
	return &auth.AuthConfig{
		IssuerURL:    response.Oidc.Issuer,
		ClientID:     response.Oidc.ClientId,
		ClientSecret: "", // Server doesn't provide client secret for security
		RedirectURL:  "http://localhost:8000/callback",
		Scopes:       response.Oidc.Scopes,
		AdminEmails:  "", // Server doesn't provide admin emails to client
	}, nil
}

// getServerUserInfo fetches user information with server-determined role
func getServerUserInfo(profile *Profile, manager *auth.Manager) (*generated.GetCurrentUserResponse, error) {
	Logger.Debug("fetching server user info", "profile", profile.Name)
	
	// Create authenticated gRPC client 
	client, err := NewEngineClientWithProfile(profile.EngineAddress, profile.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() { _ = client.Close() }()
	
	// Get access token for authentication
	ctx := context.Background()
	token, err := manager.GetValidToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}
	
	// Add authorization header as gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)
	
	// Call the authenticated endpoint
	response, err := client.client.GetCurrentUser(ctx, &generated.GetCurrentUserRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get current user from server: %w", err)
	}
	
	Logger.Debug("received server user info", "user_id", response.UserId, "email", response.Email, "org_role", response.OrgRole)
	
	return response, nil
}