# HTTPS Certificate Management Testing Guide

This guide provides step-by-step instructions for testing the complete HTTPS certificate management system in Rocketship.

## Overview

Rocketship now supports HTTPS with:
- **Self-signed certificates** for development/testing
- **Let's Encrypt certificates** for production
- **Automatic certificate management** and renewal
- **Cloudflared tunnel integration** for local HTTPS validation

## Prerequisites

- Rocketship CLI installed (`make install`)
- Access to DNS management for your domain (for Let's Encrypt)
- `cloudflared` installed for local tunnel testing (optional)

## Test Scenario 1: Self-Signed Certificates (Development)

### Step 1: Generate Self-Signed Certificate

```bash
# Generate self-signed certificate for localhost
rocketship certs generate --domain localhost --self-signed
```

**Expected Output:**
- Success message with certificate location
- Warning about browser security warnings

### Step 2: Check Certificate Status

```bash
rocketship certs status
```

**Expected Output:**
- Shows localhost certificate as valid
- Displays expiry date (1 year from now)
- Shows "Rocketship Self-Signed" as issuer

### Step 3: Start Engine with HTTPS

```bash
rocketship start server --https --domain localhost --background
```

**Expected Output:**
- Engine starts with TLS enabled
- Listens on port 7700 with HTTPS

### Step 4: Test HTTPS Connection

```bash
# Set environment variables for TLS client
export ROCKETSHIP_TLS_ENABLED=true
export ROCKETSHIP_TLS_DOMAIN=localhost

# Test with a simple validation
rocketship validate examples/simple-http/rocketship.yaml
```

**Expected Output:**
- CLI connects to engine via HTTPS
- Validation succeeds

### Step 5: Run Tests via HTTPS

```bash
# Run tests through HTTPS connection
rocketship run -f examples/simple-http/rocketship.yaml
```

**Expected Output:**
- Tests run successfully via encrypted connection
- All existing functionality works

### Step 6: Clean Up

```bash
rocketship stop server
unset ROCKETSHIP_TLS_ENABLED
unset ROCKETSHIP_TLS_DOMAIN
```

## Test Scenario 2: Let's Encrypt with Real Domain

### Prerequisites
- Own a domain (e.g., `globalbank.rocketship.sh`)
- Access to DNS management
- Server accessible from internet OR cloudflared installed

### Step 1: Generate Let's Encrypt Certificate

**Option A: With Public Server**
```bash
rocketship certs generate --domain globalbank.rocketship.sh --email your@email.com
```

**Option B: With Local Tunnel (Recommended for Testing)**
```bash
rocketship certs generate --domain globalbank.rocketship.sh --email your@email.com --local
```

**For Option B, you'll see:**
1. Cloudflared tunnel URL (e.g., `https://random.trycloudflare.com`)
2. Prompt to update DNS: `globalbank.rocketship.sh → random.trycloudflare.com (CNAME)`
3. Wait for "Press Enter when DNS is updated..."

### Step 2: Update DNS Record

In your DNS management interface:
- Add CNAME record: `globalbank.rocketship.sh` → `random.trycloudflare.com`
- Wait for DNS propagation (1-5 minutes)
- Press Enter in the CLI

**Expected Output:**
- ACME challenge completes successfully
- Certificate obtained from Let's Encrypt
- Certificate saved locally

### Step 3: Verify Certificate

```bash
rocketship certs status
```

**Expected Output:**
- Shows your domain certificate as valid
- Issued by "Let's Encrypt"
- Valid for 90 days

### Step 4: Test Production HTTPS

```bash
# Start engine with Let's Encrypt certificate
export ROCKETSHIP_TLS_ENABLED=true
export ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh

rocketship start server --https --domain globalbank.rocketship.sh --background
```

**Expected Output:**
- Engine starts with production TLS certificate
- No browser warnings when accessing via domain

### Step 5: Production Test

```bash
# Run tests through production HTTPS
rocketship run -f examples/simple-http/rocketship.yaml
```

**Expected Output:**
- Tests run successfully via production HTTPS
- Full encryption end-to-end

## Test Scenario 3: Certificate Renewal

### Step 1: Force Certificate Renewal

```bash
# Force renewal of a certificate
rocketship certs renew globalbank.rocketship.sh --force
```

**Expected Output:**
- Certificate renewal process starts
- New certificate issued
- Success confirmation

### Step 2: Verify Renewed Certificate

```bash
rocketship certs status
```

**Expected Output:**
- Updated "Issued" date
- New expiry date
- Certificate still valid

## Test Scenario 4: Complete Authentication + HTTPS

### Step 1: Set Up Full Environment

```bash
# Set all environment variables for authenticated HTTPS
source test-env.sh  # Contains all your auth variables
export ROCKETSHIP_TLS_ENABLED=true
export ROCKETSHIP_TLS_DOMAIN=localhost
```

### Step 2: Start Authenticated HTTPS Engine

```bash
rocketship start server --https --domain localhost --background
```

### Step 3: Authenticate

```bash
rocketship auth login
# Complete OIDC flow
```

### Step 4: Test Authenticated HTTPS Operations

```bash
# Test team operations via HTTPS
rocketship team list
rocketship team create "Test Team"

# Test token operations via HTTPS  
rocketship token create "Test Token"
rocketship token list

# Test runs via authenticated HTTPS
rocketship run -f examples/simple-http/rocketship.yaml
```

**Expected Output:**
- All operations work via authenticated HTTPS
- Full security: authentication + encryption

## Test Scenario 5: Error Handling

### Step 1: Test Missing Certificate

```bash
export ROCKETSHIP_TLS_ENABLED=true
export ROCKETSHIP_TLS_DOMAIN=nonexistent.domain

rocketship start server --https --domain nonexistent.domain
```

**Expected Output:**
- Clear error message about missing certificate
- Suggestion to run `rocketship certs generate`

### Step 2: Test Invalid Domain

```bash
rocketship certs generate --domain "invalid domain name" --self-signed
```

**Expected Output:**
- Clear error about invalid domain format

### Step 3: Test Missing Email for Let's Encrypt

```bash
rocketship certs generate --domain test.example.com
```

**Expected Output:**
- Error requiring email for Let's Encrypt certificates

## Installation Notes

If you encounter "cloudflared not found" during local tunnel testing:

**macOS:**
```bash
brew install cloudflared
```

**Linux (Debian/Ubuntu):**
```bash
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o cloudflared.deb
sudo dpkg -i cloudflared.deb
```

**Linux (Other):**
```bash
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared
chmod +x cloudflared
sudo mv cloudflared /usr/local/bin/
```

## Troubleshooting

### TLS Handshake Errors
- Ensure both client and server have same TLS domain configured
- Check certificate exists: `rocketship certs status`
- Verify DNS resolution for domain

### Certificate Not Found
- Generate certificate first: `rocketship certs generate --domain yourdomain`
- Check certificate directory: `~/.rocketship/certs/`

### Let's Encrypt Rate Limits
- Use staging environment: `--staging` flag
- Wait before retrying (rate limits reset)

### DNS Propagation Issues
- Wait longer for DNS changes (up to 15 minutes)
- Use online DNS propagation checkers
- Verify CNAME record points correctly

## Security Considerations

### Self-Signed Certificates
- ⚠️ **Development only** - browsers will show warnings
- ✅ **Traffic is encrypted** but identity not verified
- ✅ **Perfect for local development**

### Let's Encrypt Certificates
- ✅ **Fully trusted** by all browsers and clients
- ✅ **Production ready** with automatic renewal
- ✅ **Free and automated**

### Environment Variables
- 🔒 **Never commit** certificates or keys to git
- 🔒 **Use secure storage** for production secrets
- 🔒 **Rotate certificates** before expiry

## Summary

The HTTPS certificate management system provides:

1. **Flexible certificate generation** (self-signed or Let's Encrypt)
2. **Local tunnel integration** for HTTPS validation without public servers
3. **Automatic certificate renewal** and management
4. **Seamless integration** with existing authentication system
5. **Production-ready security** with Let's Encrypt certificates

All existing Rocketship functionality works transparently over HTTPS once certificates are configured and TLS is enabled.