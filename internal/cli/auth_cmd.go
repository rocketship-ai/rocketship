package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rocketship-ai/rocketship/internal/cli/auth"
	"github.com/rocketship-ai/rocketship/internal/cli/oidc"
	"github.com/spf13/cobra"
)

func NewLoginCmd() *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate the CLI via OIDC device flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd.Context(), profileName)
		},
	}
	cmd.Flags().StringVarP(&profileName, "profile", "p", "", "Profile to authenticate (defaults to active profile)")
	return cmd
}

func NewLogoutCmd() *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored authentication tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(profileName)
		},
	}
	cmd.Flags().StringVarP(&profileName, "profile", "p", "", "Profile to clear (defaults to active profile)")
	return cmd
}

func NewAuthStatusCmd() *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(profileName)
		},
	}
	cmd.Flags().StringVarP(&profileName, "profile", "p", "", "Profile to inspect (defaults to active profile)")
	return cmd
}

func runLogin(ctx context.Context, profileFlag string) error {
	cfg, profile, name, err := resolveProfile(profileFlag)
	if err != nil {
		return err
	}

	client, err := newProfileClient(profile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			Logger.Debug("failed to close login client", "error", cerr)
		}
	}()

	info, err := client.GetServerInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch server info: %w", err)
	}
	if !strings.EqualFold(info.AuthType, "oidc") {
		return fmt.Errorf("profile %s is not configured for OIDC auth (reported type: %s)", name, info.AuthType)
	}

	flowCfg, err := buildFlowConfig(profile, info)
	if err != nil {
		return err
	}

	loginCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	device, err := oidc.StartDeviceAuthorization(loginCtx, flowCfg)
	if err != nil {
		return err
	}

	fmt.Println("Follow the instructions below to approve access:")
	if device.VerificationURIComplete != "" {
		fmt.Printf("  1. Open %s\n", device.VerificationURIComplete)
	} else {
		fmt.Printf("  1. Open %s\n", device.VerificationURI)
		fmt.Printf("  2. Enter code: %s\n", device.UserCode)
	}
	fmt.Printf("This code expires in %s.\n", device.ExpiresIn.Round(time.Second))
	fmt.Println("Waiting for approval...")

	tokenData, err := oidc.PollToken(loginCtx, flowCfg, device)
	if err != nil {
		return err
	}

	manager, err := auth.NewManager()
	if err != nil {
		return err
	}
	if err := manager.Save(name, tokenData); err != nil {
		return err
	}

	// Persist issuer/client metadata for the profile so status and future logins have context
	profile.Auth.Issuer = flowCfg.Issuer
	profile.Auth.ClientID = flowCfg.ClientID
	cfg.AddProfile(profile)
	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to persist profile metadata: %w", err)
	}

	fmt.Println("✅ Login complete")
	if err := maybeRunOnboarding(ctx, name, tokenData, manager); err != nil {
		return err
	}
	if updated, err := manager.Load(name); err == nil && updated.IDToken != "" {
		if user, derr := decodeIDToken(updated.IDToken); derr == nil {
			fmt.Printf("Authenticated as %s\n", user)
		}
	}
	return nil
}

func runLogout(profileFlag string) error {
	_, _, name, err := resolveProfile(profileFlag)
	if err != nil {
		return err
	}

	manager, err := auth.NewManager()
	if err != nil {
		return err
	}
	if err := manager.Delete(name); err != nil {
		return err
	}
	fmt.Printf("Logged out of profile %s\n", name)
	return nil
}

func runAuthStatus(profileFlag string) error {
	_, profile, name, err := resolveProfile(profileFlag)
	if err != nil {
		return err
	}

	manager, err := auth.NewManager()
	if err != nil {
		return err
	}
	tokenData, err := manager.Load(name)
	if err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			fmt.Printf("Profile %s: not logged in\n", name)
			return nil
		}
		return err
	}

	fmt.Printf("Profile: %s\n", name)
	if profile.Auth.Issuer != "" {
		fmt.Printf("Issuer: %s\n", profile.Auth.Issuer)
	}
	if tokenData.Audience != "" {
		fmt.Printf("Audience: %s\n", tokenData.Audience)
	}
	if !tokenData.Expiry.IsZero() {
		remaining := time.Until(tokenData.Expiry)
		fmt.Printf("Access token expires in: %s\n", remaining.Round(time.Second))
	} else {
		fmt.Println("Access token expiry: not provided")
	}
	if tokenData.HasRefresh() {
		fmt.Println("Refresh token: present")
	} else {
		fmt.Println("Refresh token: not stored")
	}
	if len(tokenData.Scopes) > 0 {
		fmt.Printf("Scopes: %s\n", strings.Join(tokenData.Scopes, " "))
	}
	if tokenData.IDToken != "" {
		if user, err := decodeIDToken(tokenData.IDToken); err == nil {
			fmt.Printf("Identity: %s\n", user)
		}
	}

	return nil
}

func resolveProfile(profileFlag string) (*Config, Profile, string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, Profile{}, "", err
	}

	name := profileFlag
	if name == "" {
		name = cfg.DefaultProfile
		if name == "" {
			name = "default"
		}
	}

	profile, exists := cfg.GetProfile(name)
	if !exists {
		return nil, Profile{}, "", fmt.Errorf("profile %s not found", name)
	}
	profile.Name = name
	return cfg, profile, name, nil
}

func newProfileClient(profile Profile) (*EngineClient, error) {
	scheme := "grpc"
	if profile.TLS.Enabled {
		scheme = "grpcs"
	}
	return NewEngineClient(fmt.Sprintf("%s://%s", scheme, profile.EngineAddress))
}

func buildFlowConfig(profile Profile, info *ServerInfo) (oidc.FlowConfig, error) {
	cfg := oidc.FlowConfig{
		ClientID:       info.ClientID,
		Audience:       info.Audience,
		Scopes:         info.Scopes,
		DeviceEndpoint: info.DeviceEndpoint,
		TokenEndpoint:  info.TokenEndpoint,
		Issuer:         info.Issuer,
	}
	if cfg.ClientID == "" {
		cfg.ClientID = profile.Auth.ClientID
	}
	if cfg.ClientID == "" {
		return oidc.FlowConfig{}, errors.New("server did not advertise a client id")
	}
	if cfg.DeviceEndpoint == "" {
		cfg.DeviceEndpoint = info.AuthEndpoint
	}
	if cfg.DeviceEndpoint == "" {
		return oidc.FlowConfig{}, errors.New("device authorization endpoint missing from discovery")
	}
	if cfg.TokenEndpoint == "" {
		return oidc.FlowConfig{}, errors.New("token endpoint missing from discovery")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email", "offline_access"}
	}
	return cfg, nil
}

func decodeIDToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid id token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Email   string `json:"email"`
		Subject string `json:"sub"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	switch {
	case claims.Email != "":
		return claims.Email, nil
	case claims.Name != "":
		return claims.Name, nil
	case claims.Subject != "":
		return claims.Subject, nil
	default:
		return "(unknown user)", nil
	}
}
