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
		Long:  `Login to Rocketship using OIDC authentication`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(cmd.Context())
		},
	}

	return cmd
}

// NewAuthLogoutCmd creates a new logout command
func NewAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout from Rocketship",
		Long:  `Logout from Rocketship and remove stored tokens`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogout(cmd.Context())
		},
	}

	return cmd
}

// NewAuthStatusCmd creates a new status command
func NewAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Long:  `Show current authentication status and user information`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cmd.Context())
		},
	}

	return cmd
}

// runAuthLogin handles the login flow
func runAuthLogin(ctx context.Context) error {
	// Get auth configuration from environment
	config, err := getAuthConfig()
	if err != nil {
		return fmt.Errorf("authentication not configured: %w", err)
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	// Create keyring storage
	storage := auth.NewKeyringStorage()

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
	if userInfo.IsAdmin {
		fmt.Printf("Admin role: %s\n", color.YellowString("Yes"))
	}

	return nil
}

// runAuthLogout handles the logout flow
func runAuthLogout(ctx context.Context) error {
	// Get auth configuration from environment
	config, err := getAuthConfig()
	if err != nil {
		// If not configured, just try to clear local storage
		storage := auth.NewKeyringStorage()
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

	// Create keyring storage
	storage := auth.NewKeyringStorage()

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
func runAuthStatus(ctx context.Context) error {
	// Get auth configuration from environment
	config, err := getAuthConfig()
	if err != nil {
		fmt.Printf("Status: %s (authentication not configured)\n", color.RedString("Not authenticated"))
		return nil
	}

	// Create OIDC client
	client, err := auth.NewOIDCClient(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create OIDC client: %w", err)
	}

	// Create keyring storage
	storage := auth.NewKeyringStorage()

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
	if userInfo.IsAdmin {
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
		Scopes:       []string{"openid", "profile", "email", "groups"},
		AdminGroup:   os.Getenv("ROCKETSHIP_OIDC_ADMIN_GROUP"),
	}, nil
}

// generateState generates a random state parameter for OAuth
func generateState() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}