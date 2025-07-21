package certs

import (
	"context"
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
	Email        string   // Contact email for Let's Encrypt
	Domains      []string // Domains to get certificates for
	CertDir      string   // Directory to store certificates
	UseStaging   bool     // Use Let's Encrypt staging environment
	AutoRenew    bool     // Enable automatic renewal
	TunnelConfig *TunnelConfig
}

// TunnelConfig contains tunnel configuration for local HTTPS
type TunnelConfig struct {
	Provider string // "cloudflared" 
	Port     int    // Local port to expose (usually 80)
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
	fmt.Printf("🔐 Requesting certificate for %v...\n", m.config.Domains)
	
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