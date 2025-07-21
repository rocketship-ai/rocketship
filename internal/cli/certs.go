package cli

import (
	"context"
	"fmt"
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
			
			return runCertsGenerate(cmd.Context(), domain, email, selfSigned, staging, local)
		},
	}

	cmd.Flags().String("domain", "", "Domain name for certificate (required)")
	cmd.Flags().String("email", "", "Contact email for Let's Encrypt")
	cmd.Flags().Bool("self-signed", false, "Generate self-signed certificate")
	cmd.Flags().Bool("staging", false, "Use Let's Encrypt staging environment")
	cmd.Flags().Bool("local", false, "Use cloudflared tunnel for local HTTPS")
	
	_ = cmd.MarkFlagRequired("domain")

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
func runCertsGenerate(ctx context.Context, domain, email string, selfSigned, staging, local bool) error {
	fmt.Printf("🚀 Starting certificate generation for %s\n", color.CyanString(domain))

	// Validate inputs
	if !selfSigned && email == "" {
		return fmt.Errorf("email is required for Let's Encrypt certificates")
	}

	// Create certificate manager config
	config := &certs.Config{
		Email:      email,
		Domains:    []string{domain},
		UseStaging: staging,
		AutoRenew:  true,
	}

	// Add tunnel config for local HTTPS
	if local && !selfSigned {
		config.TunnelConfig = &certs.TunnelConfig{
			Provider: "cloudflared",
			Port:     80,
		}
		fmt.Println("📡 Local mode: Will create tunnel for HTTPS validation")
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
	fmt.Printf("1. Start the engine with HTTPS: %s\n", color.CyanString("rocketship start server --https"))
	fmt.Printf("2. Access the API at: %s\n", color.CyanString(fmt.Sprintf("https://%s:8443", domain)))

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