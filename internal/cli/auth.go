package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

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
	
	// Get auth configuration from profile
	authConfig := getProfileAuthConfig(profile)
	if authConfig == nil {
		return fmt.Errorf("authentication not configured for profile '%s'\n\nTo set up authentication:\n  1. Configure OIDC settings in your profile\n  2. Or connect to Rocketship Cloud: rocketship connect https://app.rocketship.sh", profile.Name)
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
	
	// Get auth configuration from profile
	config := getProfileAuthConfig(profile)
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

	// Get user info
	userInfo, err := manager.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Display status
	fmt.Printf("Status: %s\n", color.GreenString("Authenticated"))
	fmt.Printf("User: %s (%s)\n", userInfo.Name, userInfo.Email)
	fmt.Printf("Subject: %s\n", userInfo.Subject)
	if userInfo.OrgRole == rbac.OrgRoleAdmin {
		fmt.Printf("Admin role: %s\n", color.YellowString("Yes"))
	} else {
		fmt.Printf("Admin role: No\n")
	}

	// Show groups if available
	if len(userInfo.Groups) > 0 {
		fmt.Printf("Groups: %s\n", strings.Join(userInfo.Groups, ", "))
	}

	return nil
}

// getAuthConfig gets authentication configuration from environment
func getAuthConfig() (*auth.AuthConfig, error) {
	issuerURL := os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	clientSecret := os.Getenv("ROCKETSHIP_OIDC_CLIENT_SECRET")

	if issuerURL == "" {
		return nil, fmt.Errorf("ROCKETSHIP_OIDC_ISSUER environment variable not set")
	}
	if clientID == "" {
		return nil, fmt.Errorf("ROCKETSHIP_OIDC_CLIENT_ID environment variable not set")
	}

	// Validate issuer URL
	if _, err := url.Parse(issuerURL); err != nil {
		return nil, fmt.Errorf("invalid ROCKETSHIP_OIDC_ISSUER: %w", err)
	}

	return &auth.AuthConfig{
		IssuerURL:    issuerURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  "http://localhost:8000/callback",
		Scopes:       []string{"openid", "profile", "email"},
		AdminEmails:  os.Getenv("ROCKETSHIP_ADMIN_EMAILS"),
	}, nil
}

// generateState generates a random state parameter for OAuth
func generateState() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}