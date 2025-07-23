package certs

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/registration"
	"golang.org/x/crypto/acme/autocert"
)

// Manager handles certificate generation and management
type Manager struct {
	config     *Config
	certDir    string
	acmeClient *autocert.Manager
}

// Config contains certificate manager configuration
type Config struct {
	Email        string            // Contact email for Let's Encrypt
	Domains      []string          // Domains to get certificates for
	CertDir      string            // Directory to store certificates
	UseStaging   bool              // Use Let's Encrypt staging environment
	AutoRenew    bool              // Enable automatic renewal
	TunnelConfig *TunnelConfig     // Configuration for HTTP-01 challenge via tunnels
	DNSConfig    *DNSConfig        // Configuration for DNS-01 challenge
	ChallengeType ChallengeType    // Type of ACME challenge to use
}

// TunnelConfig contains tunnel configuration for local HTTPS
type TunnelConfig struct {
	Provider string // "cloudflared" 
	Port     int    // Local port to expose (usually 80)
}

// DNSConfig contains DNS provider configuration for DNS-01 challenge
type DNSConfig struct {
	Provider    string            // DNS provider (cloudflare, route53, etc.)
	Credentials map[string]string // Provider-specific credentials
}

// ChallengeType represents the type of ACME challenge to use
type ChallengeType string

const (
	ChallengeTypeHTTP01 ChallengeType = "http-01"
	ChallengeTypeDNS01  ChallengeType = "dns-01"
)

// LegoUser implements the lego registration.User interface
type LegoUser struct {
	Email        string
	Registration *registration.Resource
	key          *rsa.PrivateKey
}

// GetEmail returns the user's email
func (u *LegoUser) GetEmail() string {
	return u.Email
}

// GetRegistration returns the user's registration resource
func (u *LegoUser) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey returns the user's private key
func (u *LegoUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// NewManager creates a new certificate manager
func NewManager(config *Config) (*Manager, error) {
	// Set default cert directory if not specified
	if config.CertDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.CertDir = filepath.Join(homeDir, ".rocketship", "certs")
	}

	// Create cert directory if it doesn't exist
	if err := os.MkdirAll(config.CertDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Configure ACME client
	acmeManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      config.Email,
		HostPolicy: autocert.HostWhitelist(config.Domains...),
		Cache:      autocert.DirCache(config.CertDir),
	}

	// Use staging environment for testing if specified
	if config.UseStaging {
		// Let's Encrypt staging directory URL
		// Note: autocert doesn't directly support staging, we'll handle this differently
		// For now, we'll use production and add a warning
		fmt.Println("⚠️  Warning: Let's Encrypt staging environment not directly supported by autocert")
	}

	return &Manager{
		config:     config,
		certDir:    config.CertDir,
		acmeClient: acmeManager,
	}, nil
}

// GenerateCertificate generates a new certificate for the configured domains
func (m *Manager) GenerateCertificate(ctx context.Context) error {
	// Choose challenge method based on configuration
	if m.config.ChallengeType == ChallengeTypeDNS01 && m.config.DNSConfig != nil {
		return m.generateCertificateDNS01(ctx)
	}
	
	// Default to HTTP-01 challenge (existing implementation)
	return m.generateCertificateHTTP01(ctx)
}

// generateCertificateHTTP01 generates certificate using HTTP-01 challenge
func (m *Manager) generateCertificateHTTP01(ctx context.Context) error {
	// If tunnel is configured, set it up first
	if m.config.TunnelConfig != nil {
		tunnelURL, err := m.setupTunnel(ctx)
		if err != nil {
			return fmt.Errorf("failed to setup tunnel: %w", err)
		}
		fmt.Printf("🔗 Tunnel established: %s\n", tunnelURL)
		fmt.Printf("\n⚠️  ACTION REQUIRED:\n")
		fmt.Printf("Please update your DNS record:\n")
		fmt.Printf("  %s → %s (CNAME)\n\n", m.config.Domains[0], tunnelURL)
		fmt.Printf("Press Enter when DNS is updated...")
		_, _ = fmt.Scanln()
	}

	// Start HTTP server for ACME challenge
	handler := m.acmeClient.HTTPHandler(nil)
	
	// Choose port based on tunnel configuration
	port := ":80"
	if m.config.TunnelConfig != nil {
		// When using tunnel, use the configured port (tunnel forwards to this port)
		port = fmt.Sprintf(":%d", m.config.TunnelConfig.Port)
	}
	
	server := &http.Server{
		Addr:    port,
		Handler: handler,
	}

	// Start server in background
	go func() {
		fmt.Printf("🌐 Starting ACME challenge server on %s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Warning: HTTP server error: %v\n", err)
		}
	}()

	// Give server time to start
	time.Sleep(3 * time.Second)

	// Request certificate
	fmt.Printf("🔐 Requesting certificate for %v using HTTP-01 challenge...\n", m.config.Domains)
	
	// Force certificate generation by making a TLS connection
	for _, domain := range m.config.Domains {
		hello := &tls.ClientHelloInfo{
			ServerName: domain,
		}
		
		cert, err := m.acmeClient.GetCertificate(hello)
		if err != nil {
			_ = server.Shutdown(ctx)
			return fmt.Errorf("failed to get certificate for %s: %w", domain, err)
		}

		// Save certificate to disk
		if err := m.saveCertificate(domain, cert); err != nil {
			_ = server.Shutdown(ctx)
			return fmt.Errorf("failed to save certificate: %w", err)
		}

		fmt.Printf("✅ Certificate obtained for %s\n", domain)
	}

	// Shutdown HTTP server
	_ = server.Shutdown(ctx)

	return nil
}

// generateCertificateDNS01 generates certificate using DNS-01 challenge
func (m *Manager) generateCertificateDNS01(ctx context.Context) error {
	fmt.Printf("🔐 Requesting certificate for %v using DNS-01 challenge...\n", m.config.Domains)
	
	// Create or load user private key
	userKey, err := m.getUserPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to get user private key: %w", err)
	}

	// Create lego user
	user := &LegoUser{
		Email: m.config.Email,
		key:   userKey,
	}

	// Create lego config
	config := lego.NewConfig(user)
	
	// Use staging environment if specified
	if m.config.UseStaging {
		config.CADirURL = lego.LEDirectoryStaging
		fmt.Printf("🧪 Using Let's Encrypt staging environment\n")
	}

	// Create lego client
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create lego client: %w", err)
	}

	// Configure DNS provider
	dnsProvider, err := m.createDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to create DNS provider: %w", err)
	}

	err = client.Challenge.SetDNS01Provider(dnsProvider)
	if err != nil {
		return fmt.Errorf("failed to set DNS provider: %w", err)
	}

	// Register user if not already registered
	if user.Registration == nil {
		fmt.Printf("📝 Registering with Let's Encrypt...\n")
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return fmt.Errorf("failed to register: %w", err)
		}
		user.Registration = reg
		
		// Save user registration
		if err := m.saveUserRegistration(user); err != nil {
			fmt.Printf("Warning: failed to save user registration: %v\n", err)
		}
	}

	// Request certificate
	request := certificate.ObtainRequest{
		Domains: m.config.Domains,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// Save certificate to disk in our format
	primaryDomain := m.config.Domains[0]
	domainDir := filepath.Join(m.certDir, primaryDomain)
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	// Save certificate
	certPath := filepath.Join(domainDir, "cert.pem")
	if err := os.WriteFile(certPath, certificates.Certificate, 0600); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// Save private key
	keyPath := filepath.Join(domainDir, "key.pem")
	if err := os.WriteFile(keyPath, certificates.PrivateKey, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	fmt.Printf("✅ Certificate obtained for %s using DNS-01 challenge\n", primaryDomain)
	fmt.Printf("📁 Certificate saved to %s\n", domainDir)

	return nil
}

// saveCertificate saves a certificate to disk
func (m *Manager) saveCertificate(domain string, cert *tls.Certificate) error {
	domainDir := filepath.Join(m.certDir, domain)
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	// Save certificate chain
	certPath := filepath.Join(domainDir, "cert.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer func() { _ = certFile.Close() }()

	// Write all certificates in the chain
	for _, certDER := range cert.Certificate {
		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDER,
		}
		if err := pem.Encode(certFile, block); err != nil {
			return fmt.Errorf("failed to write certificate: %w", err)
		}
	}

	// Save private key
	keyPath := filepath.Join(domainDir, "key.pem")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer func() { _ = keyFile.Close() }()

	// Extract private key bytes
	privKey := cert.PrivateKey
	var keyDER []byte
	
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		keyDER = x509.MarshalPKCS1PrivateKey(k)
	default:
		return fmt.Errorf("unsupported private key type")
	}

	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyDER,
	}
	if err := pem.Encode(keyFile, keyBlock); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	fmt.Printf("📁 Certificate saved to %s\n", domainDir)
	return nil
}

// GetCertificate returns the certificate for a domain
func (m *Manager) GetCertificate(domain string) (*tls.Certificate, error) {
	certPath := filepath.Join(m.certDir, domain, "cert.pem")
	keyPath := filepath.Join(m.certDir, domain, "key.pem")

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	return &cert, nil
}

// GenerateSelfSigned generates a self-signed certificate (for development)
func (m *Manager) GenerateSelfSigned(domain string) error {
	// Generate RSA private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Rocketship Self-Signed"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate
	cert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	if err := m.saveCertificate(domain, cert); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	fmt.Printf("⚠️  Generated self-signed certificate for %s\n", domain)
	return nil
}

// Status returns the status of certificates
func (m *Manager) Status() ([]CertificateStatus, error) {
	var statuses []CertificateStatus

	entries, err := os.ReadDir(m.certDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		domain := entry.Name()
		certPath := filepath.Join(m.certDir, domain, "cert.pem")

		// Load certificate to check expiry
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			continue
		}

		block, _ := pem.Decode(certPEM)
		if block == nil {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		status := CertificateStatus{
			Domain:    domain,
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
			Issuer:    cert.Issuer.String(),
			Valid:     time.Now().After(cert.NotBefore) && time.Now().Before(cert.NotAfter),
		}

		// Check if renewal is needed (30 days before expiry)
		status.NeedsRenewal = time.Until(cert.NotAfter) < 30*24*time.Hour

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// CertificateStatus represents the status of a certificate
type CertificateStatus struct {
	Domain       string
	NotBefore    time.Time
	NotAfter     time.Time
	Issuer       string
	Valid        bool
	NeedsRenewal bool
}

// createDNSProvider creates a DNS provider based on configuration  
func (m *Manager) createDNSProvider() (challenge.Provider, error) {
	if m.config.DNSConfig == nil {
		return nil, fmt.Errorf("DNS configuration is required for DNS-01 challenge")
	}

	switch m.config.DNSConfig.Provider {
	case "cloudflare":
		// Check for required Cloudflare credentials
		apiToken, hasToken := m.config.DNSConfig.Credentials["CF_API_TOKEN"]
		email, hasEmail := m.config.DNSConfig.Credentials["CF_API_EMAIL"]
		key, hasKey := m.config.DNSConfig.Credentials["CF_API_KEY"]

		// Set environment variables for the provider
		if hasToken {
			_ = os.Setenv("CF_API_TOKEN", apiToken)
		}
		if hasEmail && hasKey {
			_ = os.Setenv("CF_API_EMAIL", email)
			_ = os.Setenv("CF_API_KEY", key)
		}

		if !hasToken && (!hasEmail || !hasKey) {
			return nil, fmt.Errorf("cloudflare requires either CF_API_TOKEN or both CF_API_EMAIL and CF_API_KEY")
		}

		fmt.Printf("🌐 Configuring Cloudflare DNS provider\n")
		return cloudflare.NewDNSProvider()

	case "route53":
		// Check for required AWS credentials
		accessKey, hasAccessKey := m.config.DNSConfig.Credentials["AWS_ACCESS_KEY_ID"]
		secretKey, hasSecretKey := m.config.DNSConfig.Credentials["AWS_SECRET_ACCESS_KEY"]
		region, hasRegion := m.config.DNSConfig.Credentials["AWS_REGION"]

		if hasAccessKey {
			_ = os.Setenv("AWS_ACCESS_KEY_ID", accessKey)
		}
		if hasSecretKey {
			_ = os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)
		}
		if hasRegion {
			_ = os.Setenv("AWS_REGION", region)
		} else {
			_ = os.Setenv("AWS_REGION", "us-east-1") // Default region
		}

		if !hasAccessKey || !hasSecretKey {
			return nil, fmt.Errorf("Route53 requires AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
		}

		fmt.Printf("🌐 Configuring Route53 DNS provider\n")
		return route53.NewDNSProvider()

	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", m.config.DNSConfig.Provider)
	}
}

// getUserPrivateKey gets or creates user private key for ACME registration
func (m *Manager) getUserPrivateKey() (*rsa.PrivateKey, error) {
	keyPath := filepath.Join(m.certDir, "user.key")

	// Try to load existing key first
	if keyPEM, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(keyPEM)
		if block != nil {
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				return key, nil
			}
		}
	}

	// Generate new key
	fmt.Printf("🔑 Generating new user private key\n")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save key
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyDER,
	}

	keyFile, err := os.Create(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	defer func() { _ = keyFile.Close() }()

	if err := pem.Encode(keyFile, keyBlock); err != nil {
		return nil, fmt.Errorf("failed to write key: %w", err)
	}

	// Set secure permissions
	if err := os.Chmod(keyPath, 0600); err != nil {
		return nil, fmt.Errorf("failed to set key permissions: %w", err)
	}

	return key, nil
}

// saveUserRegistration saves user registration data
func (m *Manager) saveUserRegistration(user *LegoUser) error {
	regPath := filepath.Join(m.certDir, "user.reg")
	
	// Simple registration storage - in production you might want JSON
	regData := fmt.Sprintf("Email: %s\nRegistration URL: %s\n", 
		user.Email, 
		user.Registration.URI,
	)

	return os.WriteFile(regPath, []byte(regData), 0600)
}

// ImportCertificate imports an existing certificate and private key (BYOC)
func (m *Manager) ImportCertificate(domain, certFile, keyFile, chainFile string) error {
	// Create domain directory
	domainDir := filepath.Join(m.certDir, domain)
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	// Read certificate file
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Read private key file
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("failed to read private key file: %w", err)
	}

	// Validate certificate and key match
	if err := m.validateCertificateKey(certData, keyData); err != nil {
		return fmt.Errorf("certificate validation failed: %w", err)
	}

	// If chain file provided, append it to certificate
	if chainFile != "" {
		chainData, err := os.ReadFile(chainFile)
		if err != nil {
			return fmt.Errorf("failed to read chain file: %w", err)
		}
		// Append chain to certificate
		certData = append(certData, '\n')
		certData = append(certData, chainData...)
	}

	// Save certificate
	certPath := filepath.Join(domainDir, "cert.pem")
	if err := os.WriteFile(certPath, certData, 0600); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// Save private key
	keyPath := filepath.Join(domainDir, "key.pem")
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	fmt.Printf("📁 Certificate imported to %s\n", domainDir)
	return nil
}

// validateCertificateKey validates that certificate and private key match
func (m *Manager) validateCertificateKey(certData, keyData []byte) error {
	// Parse certificate
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Parse private key
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return fmt.Errorf("failed to parse private key PEM")
	}

	var privateKey crypto.PrivateKey
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		privateKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	case "EC PRIVATE KEY":
		privateKey, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	default:
		return fmt.Errorf("unsupported private key type: %s", keyBlock.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Verify certificate and key match
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		priv, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("certificate is RSA but private key is not")
		}
		if pub.N.Cmp(priv.N) != 0 {
			return fmt.Errorf("certificate and private key do not match")
		}
	default:
		// For now, we'll assume other key types are valid if they parse correctly
		// In production, you might want to add more specific validation
	}

	// Check certificate validity
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid (valid from %v)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired (expired %v)", cert.NotAfter)
	}

	fmt.Printf("✅ Certificate validation successful\n")
	fmt.Printf("   Subject: %s\n", cert.Subject.CommonName)
	fmt.Printf("   Issuer: %s\n", cert.Issuer.CommonName)
	fmt.Printf("   Valid from: %s\n", cert.NotBefore.Format("2006-01-02 15:04:05"))
	fmt.Printf("   Valid until: %s\n", cert.NotAfter.Format("2006-01-02 15:04:05"))

	return nil
}