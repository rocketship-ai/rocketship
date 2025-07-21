package certs

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// setupTunnel sets up a cloudflared tunnel for local HTTPS
func (m *Manager) setupTunnel(ctx context.Context) (string, error) {
	// Check if cloudflared is installed
	cloudflaredPath, err := m.ensureCloudflared()
	if err != nil {
		return "", fmt.Errorf("failed to setup cloudflared: %w", err)
	}

	// Start cloudflared tunnel
	port := m.config.TunnelConfig.Port
	if port == 0 {
		port = 80
	}

	fmt.Printf("🚇 Starting cloudflared tunnel on port %d...\n", port)

	// Run cloudflared tunnel
	cmd := exec.CommandContext(ctx, cloudflaredPath, "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
	
	// Create pipes for output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start cloudflared: %w", err)
	}

	// Read output to find tunnel URL
	tunnelURL := ""
	urlRegex := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
	
	// Create scanner for stderr (cloudflared outputs to stderr)
	scanner := bufio.NewScanner(stderr)
	
	// Also scan stdout just in case
	go func() {
		stdoutScanner := bufio.NewScanner(stdout)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			if matches := urlRegex.FindStringSubmatch(line); len(matches) > 0 {
				tunnelURL = matches[0]
			}
		}
	}()

	// Set timeout for finding URL
	timeout := time.After(30 * time.Second)
	
	for {
		select {
		case <-timeout:
			_ = cmd.Process.Kill()
			return "", fmt.Errorf("timeout waiting for tunnel URL")
		default:
			if scanner.Scan() {
				line := scanner.Text()
				fmt.Printf("  %s\n", line) // Print cloudflared output for debugging
				
				if matches := urlRegex.FindStringSubmatch(line); len(matches) > 0 {
					tunnelURL = matches[0]
					// Extract just the subdomain for DNS
					parts := strings.Split(tunnelURL, ".")
					if len(parts) >= 3 {
						subdomain := strings.TrimPrefix(parts[0], "https://")
						return fmt.Sprintf("%s.trycloudflare.com", subdomain), nil
					}
				}
			}
		}
		
		if tunnelURL != "" {
			break
		}
	}

	// Keep tunnel running in background
	go func() {
		_ = cmd.Wait()
	}()

	return tunnelURL, nil
}

// ensureCloudflared ensures cloudflared is installed
func (m *Manager) ensureCloudflared() (string, error) {
	// Check if cloudflared is already in PATH
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}

	// Provide installation instructions
	fmt.Printf("\n❌ cloudflared not found. Please install it first:\n\n")
	
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("  macOS:")
		fmt.Println("    brew install cloudflared")
		fmt.Println("    # or")
		fmt.Println("    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-amd64.tgz | tar xz")
		
	case "linux":
		fmt.Println("  Linux:")
		fmt.Println("    # Debian/Ubuntu:")
		fmt.Println("    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o cloudflared.deb")
		fmt.Println("    sudo dpkg -i cloudflared.deb")
		fmt.Println("    # or for other distros:")
		fmt.Println("    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared")
		fmt.Println("    chmod +x cloudflared")
		fmt.Println("    sudo mv cloudflared /usr/local/bin/")
		
	case "windows":
		fmt.Println("  Windows:")
		fmt.Println("    Download from: https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe")
		fmt.Println("    Add to PATH")
	}
	
	fmt.Printf("\nAfter installation, run this command again.\n")
	
	return "", fmt.Errorf("cloudflared not installed")
}


// StopTunnel stops any running cloudflared tunnels
func (m *Manager) StopTunnel() error {
	// Find and kill cloudflared processes
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/F", "/IM", "cloudflared.exe")
		_ = cmd.Run() // Ignore errors
	} else {
		cmd := exec.Command("pkill", "-f", "cloudflared")
		_ = cmd.Run() // Ignore errors
	}
	return nil
}