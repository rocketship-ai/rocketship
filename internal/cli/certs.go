package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/rocketship-ai/rocketship/internal/certs"
)

// NewCertsCmd creates a new certs command
func NewCertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Certificate management commands",
		Long:  `Manage HTTPS certificates for Rocketship`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewCertsGenerateCmd(),
		NewCertsImportCmd(),
		NewCertsStatusCmd(),
		NewCertsRenewCmd(),
	)

	return cmd
}

// NewCertsGenerateCmd creates a new certificate generation command
func NewCertsGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate HTTPS certificate",
		Long:  `Generate HTTPS certificate using Let's Encrypt or self-signed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, _ := cmd.Flags().GetString("domain")
			email, _ := cmd.Flags().GetString("email")
			selfSigned, _ := cmd.Flags().GetBool("self-signed")
			staging, _ := cmd.Flags().GetBool("staging")
			local, _ := cmd.Flags().GetBool("local")
			dns, _ := cmd.Flags().GetBool("dns")
			dnsProvider, _ := cmd.Flags().GetString("dns-provider")
			
			return runCertsGenerate(cmd.Context(), domain, email, selfSigned, staging, local, dns, dnsProvider)
		},
	}

	cmd.Flags().String("domain", "", "Domain name for certificate (required)")
	cmd.Flags().String("email", "", "Contact email for Let's Encrypt")
	cmd.Flags().Bool("self-signed", false, "Generate self-signed certificate")
	cmd.Flags().Bool("staging", false, "Use Let's Encrypt staging environment")
	cmd.Flags().Bool("local", false, "Use cloudflared tunnel for local HTTPS (HTTP-01)")
	cmd.Flags().Bool("dns", false, "Use DNS-01 challenge instead of HTTP-01")
	cmd.Flags().String("dns-provider", "cloudflare", "DNS provider for DNS-01 challenge (cloudflare, route53)")
	
	_ = cmd.MarkFlagRequired("domain")

	return cmd
}

// NewCertsImportCmd creates a certificate import command for BYOC
func NewCertsImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import existing certificate (BYOC - Bring Your Own Certificate)",
		Long:  `Import an existing SSL certificate and private key for use with Rocketship`,
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, _ := cmd.Flags().GetString("domain")
			certFile, _ := cmd.Flags().GetString("cert-file")
			keyFile, _ := cmd.Flags().GetString("key-file")
			chainFile, _ := cmd.Flags().GetString("chain-file")
			
			return runCertsImport(cmd.Context(), domain, certFile, keyFile, chainFile)
		},
	}

	cmd.Flags().String("domain", "", "Domain name for the certificate (required)")
	cmd.Flags().String("cert-file", "", "Path to certificate file (.crt or .pem) (required)")
	cmd.Flags().String("key-file", "", "Path to private key file (.key or .pem) (required)")
	cmd.Flags().String("chain-file", "", "Path to certificate chain file (optional)")
	
	_ = cmd.MarkFlagRequired("domain")
	_ = cmd.MarkFlagRequired("cert-file")
	_ = cmd.MarkFlagRequired("key-file")

	return cmd
}

// NewCertsStatusCmd creates a certificate status command
func NewCertsStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show certificate status",
		Long:  `Show status of all managed certificates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCertsStatus(cmd.Context())
		},
	}

	return cmd
}

// NewCertsRenewCmd creates a certificate renewal command  
func NewCertsRenewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "renew [domain]",
		Short: "Renew certificate",
		Long:  `Renew certificate for a domain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := ""
			if len(args) > 0 {
				domain = args[0]
			}
			force, _ := cmd.Flags().GetBool("force")
			return runCertsRenew(cmd.Context(), domain, force)
		},
	}

	cmd.Flags().Bool("force", false, "Force renewal even if not expiring")

	return cmd
}

// runCertsGenerate handles certificate generation
func runCertsGenerate(ctx context.Context, domain, email string, selfSigned, staging, local, dns bool, dnsProvider string) error {
	fmt.Printf("🚀 Starting certificate generation for %s\n", color.CyanString(domain))

	// Validate inputs
	if !selfSigned && email == "" {
		return fmt.Errorf("email is required for Let's Encrypt certificates")
	}

	// Validate conflicting options
	if dns && local {
		return fmt.Errorf("cannot use both --dns and --local flags together")
	}

	// Create certificate manager config
	config := &certs.Config{
		Email:      email,
		Domains:    []string{domain},
		UseStaging: staging,
		AutoRenew:  true,
	}

	// Configure challenge type
	if dns && !selfSigned {
		config.ChallengeType = certs.ChallengeTypeDNS01
		config.DNSConfig = &certs.DNSConfig{
			Provider:    dnsProvider,
			Credentials: make(map[string]string),
		}

		// Collect DNS provider credentials from environment
		fmt.Printf("🌐 Using DNS-01 challenge with %s provider\n", color.CyanString(dnsProvider))
		if err := collectDNSCredentials(config.DNSConfig); err != nil {
			return fmt.Errorf("failed to configure DNS provider: %w", err)
		}
	} else if local && !selfSigned {
		config.ChallengeType = certs.ChallengeTypeHTTP01
		config.TunnelConfig = &certs.TunnelConfig{
			Provider: "cloudflared",
			Port:     80,
		}
		fmt.Println("📡 Using HTTP-01 challenge with cloudflared tunnel")
	} else if !selfSigned {
		config.ChallengeType = certs.ChallengeTypeHTTP01
		fmt.Println("🌐 Using HTTP-01 challenge (direct)")
	}

	// Create certificate manager
	manager, err := certs.NewManager(config)
	if err != nil {
		return fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Generate certificate
	if selfSigned {
		fmt.Println("🔐 Generating self-signed certificate...")
		if err := manager.GenerateSelfSigned(domain); err != nil {
			return fmt.Errorf("failed to generate self-signed certificate: %w", err)
		}
		fmt.Printf("\n%s Self-signed certificate generated successfully!\n", color.GreenString("✅"))
		fmt.Printf("⚠️  %s\n", color.YellowString("Warning: Browsers will show security warnings for self-signed certificates"))
	} else {
		fmt.Println("🔐 Requesting certificate from Let's Encrypt...")
		if staging {
			fmt.Printf("📍 Using %s environment\n", color.YellowString("STAGING"))
		}
		
		if err := manager.GenerateCertificate(ctx); err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}
		
		fmt.Printf("\n%s Certificate generated successfully!\n", color.GreenString("✅"))
		fmt.Printf("🔒 Certificate issued by: %s\n", color.BlueString("Let's Encrypt"))
	}

	// Show next steps
	fmt.Printf("\n📋 Next steps:\n")
	fmt.Printf("1. Start the engine with HTTPS: %s\n", color.CyanString("rocketship start server --https --domain "+domain))
	fmt.Printf("2. Access the API at: %s\n", color.CyanString(fmt.Sprintf("https://%s:7700", domain)))

	return nil
}

// runCertsImport handles certificate import (BYOC)
func runCertsImport(ctx context.Context, domain, certFile, keyFile, chainFile string) error {
	fmt.Printf("📥 Importing certificate for %s\n", color.CyanString(domain))

	// Validate files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return fmt.Errorf("private key file not found: %s", keyFile)
	}
	if chainFile != "" {
		if _, err := os.Stat(chainFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate chain file not found: %s", chainFile)
		}
	}

	// Create certificate manager to get cert directory
	config := &certs.Config{}
	manager, err := certs.NewManager(config)
	if err != nil {
		return fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Import certificate using manager
	if err := manager.ImportCertificate(domain, certFile, keyFile, chainFile); err != nil {
		return fmt.Errorf("failed to import certificate: %w", err)
	}

	fmt.Printf("%s Certificate imported successfully!\n", color.GreenString("✅"))
	fmt.Printf("📁 Certificate stored for domain: %s\n", color.CyanString(domain))
	
	// Show next steps
	fmt.Printf("\n📋 Next steps:\n")
	fmt.Printf("1. Verify certificate: %s\n", color.CyanString("rocketship certs status"))
	fmt.Printf("2. Start engine with HTTPS: %s\n", color.CyanString("rocketship start server --https --domain "+domain))
	fmt.Printf("3. Access the API at: %s\n", color.CyanString(fmt.Sprintf("https://%s:7700", domain)))

	return nil
}

// runCertsStatus shows certificate status
func runCertsStatus(ctx context.Context) error {
	// Create certificate manager with default config
	config := &certs.Config{}
	manager, err := certs.NewManager(config)
	if err != nil {
		return fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Get certificate statuses
	statuses, err := manager.Status()
	if err != nil {
		return fmt.Errorf("failed to get certificate status: %w", err)
	}

	if len(statuses) == 0 {
		fmt.Println("No certificates found")
		return nil
	}

	fmt.Println("\nCertificate Status:")
	fmt.Println(strings.Repeat("-", 80))

	for _, status := range statuses {
		fmt.Printf("\n%s %s\n", color.CyanString("●"), status.Domain)
		fmt.Printf("  Valid: %s\n", formatCertBool(status.Valid))
		fmt.Printf("  Issued: %s\n", status.NotBefore.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Expires: %s\n", status.NotAfter.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Days remaining: %d\n", int(time.Until(status.NotAfter).Hours()/24))
		fmt.Printf("  Issuer: %s\n", status.Issuer)
		
		if status.NeedsRenewal {
			fmt.Printf("  %s\n", color.YellowString("⚠ Needs renewal"))
		}
	}

	return nil
}

// runCertsRenew handles certificate renewal
func runCertsRenew(ctx context.Context, domain string, force bool) error {
	// Create certificate manager with default config
	config := &certs.Config{}
	manager, err := certs.NewManager(config)
	if err != nil {
		return fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Get certificate statuses
	statuses, err := manager.Status()
	if err != nil {
		return fmt.Errorf("failed to get certificate status: %w", err)
	}

	// Find certificates to renew
	var toRenew []certs.CertificateStatus
	for _, status := range statuses {
		if domain == "" || status.Domain == domain {
			if force || status.NeedsRenewal {
				toRenew = append(toRenew, status)
			}
		}
	}

	if len(toRenew) == 0 {
		fmt.Println("No certificates need renewal")
		return nil
	}

	// Renew certificates
	for _, cert := range toRenew {
		fmt.Printf("🔄 Renewing certificate for %s...\n", color.CyanString(cert.Domain))
		
		// Configure manager for this domain
		config := &certs.Config{
			Domains: []string{cert.Domain},
		}
		
		manager, err := certs.NewManager(config)
		if err != nil {
			return fmt.Errorf("failed to create certificate manager: %w", err)
		}
		
		if err := manager.GenerateCertificate(ctx); err != nil {
			return fmt.Errorf("failed to renew certificate: %w", err)
		}
		
		fmt.Printf("%s Certificate renewed successfully!\n", color.GreenString("✅"))
	}

	return nil
}

// formatCertBool formats a boolean as colored string for certificates
func formatCertBool(b bool) string {
	if b {
		return color.GreenString("Yes")
	}
	return color.RedString("No")
}

// collectDNSCredentials collects DNS provider credentials from environment variables
func collectDNSCredentials(dnsConfig *certs.DNSConfig) error {
	switch dnsConfig.Provider {
	case "cloudflare":
		// Check for Cloudflare credentials
		if token := os.Getenv("CF_API_TOKEN"); token != "" {
			dnsConfig.Credentials["CF_API_TOKEN"] = token
			fmt.Printf("✅ Found Cloudflare API token\n")
			return nil
		}
		
		if email := os.Getenv("CF_API_EMAIL"); email != "" {
			if key := os.Getenv("CF_API_KEY"); key != "" {
				dnsConfig.Credentials["CF_API_EMAIL"] = email
				dnsConfig.Credentials["CF_API_KEY"] = key
				fmt.Printf("✅ Found Cloudflare API email and key\n")
				return nil
			}
		}
		
		// Show required environment variables
		return fmt.Errorf("cloudflare DNS provider requires environment variables:\n" +
			"  Option 1: CF_API_TOKEN (recommended)\n" +
			"  Option 2: CF_API_EMAIL and CF_API_KEY\n\n" +
			"Get your Cloudflare API token:\n" +
			"  1. Go to https://dash.cloudflare.com/profile/api-tokens\n" +
			"  2. Create token with 'Zone:DNS:Edit' permissions\n" +
			"  3. Export CF_API_TOKEN='your-token-here'")

	case "route53":
		// Check for AWS credentials
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		
		if accessKey == "" || secretKey == "" {
			return fmt.Errorf("Route53 DNS provider requires environment variables:\n" +
				"  AWS_ACCESS_KEY_ID\n" +
				"  AWS_SECRET_ACCESS_KEY\n" +
				"  AWS_REGION (optional, defaults to us-east-1)\n\n" +
				"Set up AWS credentials:\n" +
				"  1. Create IAM user with Route53 permissions\n" +
				"  2. Export AWS_ACCESS_KEY_ID='your-access-key'\n" +
				"  3. Export AWS_SECRET_ACCESS_KEY='your-secret-key'")
		}
		
		dnsConfig.Credentials["AWS_ACCESS_KEY_ID"] = accessKey
		dnsConfig.Credentials["AWS_SECRET_ACCESS_KEY"] = secretKey
		
		if region := os.Getenv("AWS_REGION"); region != "" {
			dnsConfig.Credentials["AWS_REGION"] = region
		}
		
		fmt.Printf("✅ Found AWS credentials\n")
		return nil

	default:
		return fmt.Errorf("unsupported DNS provider: %s", dnsConfig.Provider)
	}
}