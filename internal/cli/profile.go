package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// profileCmd represents the profile command
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage connection profiles",
	Long: `Manage connection profiles for different Rocketship environments.

Profiles allow you to easily switch between different Rocketship deployments
(local, self-hosted, cloud) without having to specify connection details
every time you run a command.

Examples:
  rocketship profile list
  rocketship profile create enterprise https://rocketship.company.com
  rocketship profile use enterprise
  rocketship profile show enterprise`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all connection profiles",
	Long:  "List all available connection profiles and show which one is currently active.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProfileList()
	},
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <name> <url>",
	Short: "Create a new connection profile",
	Long: `Create a new connection profile for a Rocketship deployment.

The URL should be the base URL of your Rocketship deployment, e.g.:
  https://rocketship.company.com
  https://globalbank.rocketship.sh

TLS settings will be automatically detected from the URL scheme.

Examples:
  rocketship profile create enterprise https://rocketship.company.com
  rocketship profile create staging https://staging.company.com:8443`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		urlStr := args[1]
		
		team, _ := cmd.Flags().GetString("team")
		port, _ := cmd.Flags().GetInt("port")
		
		return runProfileCreate(name, urlStr, team, port)
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a connection profile",
	Long:  "Delete a connection profile. The local profile cannot be deleted.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		return runProfileDelete(name)
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the default connection profile",
	Long:  "Set the default connection profile to use for all commands.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		return runProfileUse(name)
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show connection profile details",
	Long:  "Show detailed information about a connection profile. If no name is provided, shows the current active profile.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		}
		return runProfileShow(name)
	},
}

var connectCmd = &cobra.Command{
	Use:   "connect <url>",
	Short: "Connect to a Rocketship deployment",
	Long: `Connect to a Rocketship deployment by creating or updating a connection profile.

If no profile name is specified, this will create or update a profile based on
the URL hostname. For example:
  rocketship connect https://rocketship.company.com  # Creates/updates "rocketship-company-com" profile
  rocketship connect https://app.rocketship.sh       # Creates/updates "cloud" profile

Examples:
  rocketship connect https://globalbank.rocketship.sh
  rocketship connect https://rocketship.company.com --name enterprise
  rocketship connect https://app.rocketship.sh`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		urlStr := args[0]
		
		name, _ := cmd.Flags().GetString("name")
		team, _ := cmd.Flags().GetString("team")
		port, _ := cmd.Flags().GetInt("port")
		setDefault, _ := cmd.Flags().GetBool("set-default")
		
		return runConnect(urlStr, name, team, port, setDefault)
	},
}

func init() {
	// Add profile subcommands
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileShowCmd)
	
	// Add flags
	profileCreateCmd.Flags().String("team", "", "Associate profile with a specific team")
	profileCreateCmd.Flags().Int("port", 0, "Override the port number")
	
	connectCmd.Flags().String("name", "", "Name for the profile (auto-generated if not provided)")
	connectCmd.Flags().String("team", "", "Associate profile with a specific team")
	connectCmd.Flags().Int("port", 0, "Override the port number")
	connectCmd.Flags().Bool("set-default", true, "Set this profile as the default")
}

// NewProfileCmd returns the profile command
func NewProfileCmd() *cobra.Command {
	return profileCmd
}

// NewConnectCmd returns the connect command  
func NewConnectCmd() *cobra.Command {
	return connectCmd
}

func runProfileList() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	if len(config.Profiles) == 0 {
		fmt.Println("No profiles found.")
		return nil
	}
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "PROFILE\tSTATUS\tENGINE ADDRESS\tTLS\tTEAM"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	
	for name, profile := range config.Profiles {
		status := ""
		if name == config.DefaultProfile {
			status = "*"
		}
		
		tlsStatus := "disabled"
		if profile.TLS.Enabled {
			tlsStatus = "enabled"
			if profile.TLS.Domain != "" {
				tlsStatus += fmt.Sprintf(" (%s)", profile.TLS.Domain)
			}
		}
		
		team := ""
		if profile.TeamContext != nil {
			team = profile.TeamContext.TeamName
		}
		
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, status, profile.EngineAddress, tlsStatus, team); err != nil {
			return fmt.Errorf("failed to write profile row: %w", err)
		}
	}
	
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}
	return nil
}

func runProfileCreate(name, urlStr, team string, port int) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	profile, err := createProfileFromURL(name, urlStr, port)
	if err != nil {
		return err
	}
	
	// Add team context if specified
	if team != "" {
		profile.TeamContext = &TeamContext{
			TeamName: team,
		}
	}
	
	config.AddProfile(profile)
	
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	fmt.Printf("✅ Created profile '%s'\n", name)
	fmt.Printf("   Engine: %s\n", profile.EngineAddress)
	if profile.TLS.Enabled {
		fmt.Printf("   TLS: enabled (%s)\n", profile.TLS.Domain)
	}
	if team != "" {
		fmt.Printf("   Team: %s\n", team)
	}
	
	return nil
}

func runProfileDelete(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	if err := config.DeleteProfile(name); err != nil {
		return err
	}
	
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	fmt.Printf("✅ Deleted profile '%s'\n", name)
	return nil
}

func runProfileUse(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	if err := config.SetDefaultProfile(name); err != nil {
		return err
	}
	
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	profile, _ := config.GetProfile(name)
	fmt.Printf("✅ Now using profile '%s'\n", name)
	fmt.Printf("   Engine: %s\n", profile.EngineAddress)
	if profile.TLS.Enabled {
		fmt.Printf("   TLS: enabled (%s)\n", profile.TLS.Domain)
	}
	
	return nil
}

func runProfileShow(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	profile, err := config.GetProfile(name)
	if err != nil {
		return err
	}
	
	fmt.Printf("Profile: %s", profile.Name)
	if profile.Name == config.DefaultProfile {
		fmt.Printf(" (default)")
	}
	fmt.Println()
	fmt.Printf("Engine Address: %s\n", profile.EngineAddress)
	
	if profile.TLS.Enabled {
		fmt.Printf("TLS: enabled\n")
		if profile.TLS.Domain != "" {
			fmt.Printf("TLS Domain: %s\n", profile.TLS.Domain)
		}
	} else {
		fmt.Printf("TLS: disabled\n")
	}
	
	if profile.Auth.Issuer != "" {
		fmt.Printf("Auth Issuer: %s\n", profile.Auth.Issuer)
		fmt.Printf("Auth Client ID: %s\n", profile.Auth.ClientID)
	}
	
	if profile.TeamContext != nil {
		fmt.Printf("Team: %s", profile.TeamContext.TeamName)
		if profile.TeamContext.TeamID != "" {
			fmt.Printf(" (%s)", profile.TeamContext.TeamID)
		}
		fmt.Println()
	}
	
	if len(profile.Environment) > 0 {
		fmt.Println("Environment Variables:")
		for key, value := range profile.Environment {
			fmt.Printf("  %s=%s\n", key, value)
		}
	}
	
	return nil
}

func runConnect(urlStr, name, team string, port int, setDefault bool) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Generate profile name if not provided
	if name == "" {
		name = generateProfileName(urlStr)
	}
	
	profile, err := createProfileFromURL(name, urlStr, port)
	if err != nil {
		return err
	}
	
	// Add team context if specified
	if team != "" {
		profile.TeamContext = &TeamContext{
			TeamName: team,
		}
	}
	
	config.AddProfile(profile)
	
	// Set as default if requested
	if setDefault {
		config.DefaultProfile = profile.Name
	}
	
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	fmt.Printf("✅ Connected to %s\n", urlStr)
	fmt.Printf("   Profile: %s", profile.Name)
	if setDefault {
		fmt.Printf(" (default)")
	}
	fmt.Println()
	fmt.Printf("   Engine: %s\n", profile.EngineAddress)
	if profile.TLS.Enabled {
		fmt.Printf("   TLS: enabled (%s)\n", profile.TLS.Domain)
	}
	
	return nil
}

func createProfileFromURL(name, urlStr string, portOverride int) (Profile, error) {
	// Handle raw host:port format (e.g., localhost:12100)
	if !strings.Contains(urlStr, "://") {
		// Add http:// scheme for parsing
		urlStr = "http://" + urlStr
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return Profile{}, fmt.Errorf("invalid URL: %w", err)
	}
	
	// Determine TLS settings from scheme
	tlsEnabled := parsedURL.Scheme == "https"
	
	// Build engine address
	host := parsedURL.Hostname()
	port := parsedURL.Port()
	
	// Use port override if provided
	if portOverride > 0 {
		port = fmt.Sprintf("%d", portOverride)
	}
	
	// Default ports
	if port == "" {
		if tlsEnabled {
			port = "443"
		} else {
			port = "7700"
		}
	}
	
	engineAddress := fmt.Sprintf("%s:%s", host, port)
	
	profile := Profile{
		Name:          name,
		EngineAddress: engineAddress,
		TLS: TLSConfig{
			Enabled: tlsEnabled,
			Domain:  host,
		},
		Environment: make(map[string]string),
	}
	
	// Set auth config for known cloud domains
	if host == "app.rocketship.sh" {
		profile.Auth = AuthProfile{
			Issuer:   "https://auth.rocketship.sh",
			ClientID: "rocketship-cloud-cli",
		}
	}
	
	// Note: Authentication configuration will be auto-detected on first use
	// The auth config is queried from the server when needed, not during profile creation
	
	return profile, nil
}

func generateProfileName(urlStr string) string {
	// Handle raw host:port format (e.g., localhost:12100)
	if !strings.Contains(urlStr, "://") {
		urlStr = "http://" + urlStr
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "custom"
	}
	
	host := parsedURL.Hostname()
	port := parsedURL.Port()
	
	// Special cases for known domains
	if host == "app.rocketship.sh" {
		return "cloud"
	}
	
	// Special case for localhost
	if host == "localhost" || host == "127.0.0.1" {
		if port != "" && port != "7700" {
			return fmt.Sprintf("local-%s", port)
		}
		return "local"
	}
	
	if strings.Contains(host, "rocketship.sh") {
		// For subdomains of rocketship.sh, use the subdomain
		parts := strings.Split(host, ".")
		if len(parts) > 2 {
			return parts[0]
		}
	}
	
	// Generate name from hostname
	name := strings.ReplaceAll(host, ".", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

