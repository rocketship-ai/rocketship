# Production Self-Hosting with HTTPS and Authentication - Step-by-Step Guide

This guide will walk you through setting up a complete production Rocketship environment with:
- **Docker multi-stack isolation**
- **Full authentication with RBAC** 
- **HTTPS with Let's Encrypt certificate**
- **Domain**: `globalbank.rocketship.sh`
- **Email**: `magiusdarrigo@gmail.com`

## Prerequisites

- Docker and Docker Compose installed
- `cloudflared` installed (for local HTTPS validation)
- Access to DNS management for `globalbank.rocketship.sh`
- Current branch: `add-auth` (which has all authentication features)

## Step 1: Install Cloudflared (if not already installed)

**macOS:**
```bash
brew install cloudflared
```

**Linux:**
```bash
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o cloudflared.deb
sudo dpkg -i cloudflared.deb
```

## Step 2: Initialize Docker Environment

The Docker multi-stack system auto-detects your branch and creates isolated environments.

```bash
# Navigate to your rocketship directory
cd /Users/magius/Downloads/personal_projects/rocketship-ai/rocketship

# Initialize the isolated Docker environment for add-auth branch
./.docker/rocketship init
```

**Expected Output:**
- Auto-detects branch: `add-auth`
- Creates stack: `rocketship-add-auth`
- Generates unique ports (avoiding conflicts)
- Creates `.docker/.env.add-auth` configuration

## Step 3: Generate Let's Encrypt Certificate

Use the local tunnel method to generate a production Let's Encrypt certificate without requiring a public server:

```bash
# Generate Let's Encrypt certificate with local tunnel
rocketship certs generate --domain globalbank.rocketship.sh --email magiusdarrigo@gmail.com --local
```

**What happens:**
1. Cloudflared creates a tunnel (e.g., `https://random123.trycloudflare.com`)
2. You'll see: "Please update your DNS record: globalbank.rocketship.sh → random123.trycloudflare.com (CNAME)"
3. The process waits for you to update DNS

## Step 4: Update DNS Record

**In your DNS management interface:**
1. Go to your DNS provider for `rocketship.sh`
2. Create/update CNAME record:
   - **Name**: `globalbank`
   - **Value**: `random123.trycloudflare.com` (use the actual tunnel URL shown)
3. Save the DNS change
4. Wait 1-5 minutes for propagation

**Verify DNS propagation:**
```bash
# Check DNS propagation
dig globalbank.rocketship.sh

# Or use online tools like whatsmydns.net
```

## Step 5: Complete Certificate Generation

Once DNS is updated:
1. **Press Enter** in the certificate generation terminal
2. Let's Encrypt will validate domain ownership via the tunnel
3. Certificate will be issued and saved

**Expected Output:**
```
✅ Certificate obtained for globalbank.rocketship.sh
📁 Certificate saved to /Users/magius/.rocketship/certs/globalbank.rocketship.sh
```

## Step 6: Verify Certificate

```bash
rocketship certs status
```

**Expected Output:**
```
Certificate Status:
--------------------------------------------------------------------------------

● globalbank.rocketship.sh
  Valid: Yes
  Issued: 2025-07-21 XX:XX:XX
  Expires: 2025-10-19 XX:XX:XX  (90 days from now)
  Days remaining: 89
  Issuer: CN=R3,O=Let's Encrypt,C=US
```

## Step 7: Configure Docker Environment for HTTPS

Update your Docker environment configuration:

```bash
# Edit the generated environment file
nano .docker/.env.add-auth
```

**Add the following lines to the end of the file:**
```bash
### HTTPS/TLS CONFIGURATION ###
# TLS Configuration for HTTPS support
ROCKETSHIP_TLS_ENABLED=true
ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh
```

**Your file should now have these sections:**
- Authentication configuration (Auth0)
- Database configuration
- **NEW: TLS configuration**

## Step 8: Start Production Docker Stack with HTTPS

```bash
# Start the complete production stack
./.docker/rocketship start
```

**This starts:**
- ✅ **Temporal** (workflow engine)
- ✅ **Elasticsearch** (temporal dependency)
- ✅ **PostgreSQL** (temporal + auth database)
- ✅ **Engine** (with HTTPS enabled)
- ✅ **Worker** (test execution)
- ✅ **Auth services** (PostgreSQL for RBAC)

**Expected Output:**
```bash
🚀 Starting rocketship-add-auth stack...
⏳ Waiting for services to be healthy...
✅ All services are running and healthy!

Stack Information:
- Stack Name: rocketship-add-auth
- Engine API: localhost:12100 (HTTPS)
- Temporal UI: http://localhost:12480
- Status: All services healthy
```

## Step 9: Verify HTTPS Connection

```bash
# Check stack status
./.docker/rocketship status
```

**Verify Engine is running with HTTPS:**
- Engine should show as "healthy"
- Engine port should be accessible

## Step 10: Configure CLI for HTTPS Connection

Set environment variables to connect via HTTPS:

```bash
# Set HTTPS client configuration
export ROCKETSHIP_TLS_ENABLED=true
export ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh

# Set Engine endpoint (use the port from your stack)
export ROCKETSHIP_ENGINE=localhost:12100

# Set authentication configuration (already in environment)
source test-env.sh  # This contains your Auth0 configuration
```

**Verify all environment variables:**
```bash
echo "TLS Enabled: $ROCKETSHIP_TLS_ENABLED"
echo "TLS Domain: $ROCKETSHIP_TLS_DOMAIN" 
echo "Engine: $ROCKETSHIP_ENGINE"
echo "OIDC Issuer: $ROCKETSHIP_OIDC_ISSUER"
echo "Admin Emails: $ROCKETSHIP_ADMIN_EMAILS"
```

## Step 11: Test HTTPS Connection

```bash
# Test HTTPS connection with validation
rocketship validate examples/simple-http/rocketship.yaml
```

**Expected Output:**
- CLI connects to engine via HTTPS
- Validation succeeds with encrypted connection

## Step 12: Authenticate via HTTPS

```bash
# Login via OIDC (over HTTPS)
rocketship auth login
```

**What happens:**
1. Opens browser for Auth0 authentication
2. Complete OAuth flow
3. CLI receives token over secure HTTPS connection

**Verify authentication:**
```bash
rocketship auth status
```

**Expected Output:**
```
✅ Authenticated as: magiusdarrigo@gmail.com
🏢 Organization Role: Organization Admin
🎫 Token expires: [expiry time]
🔗 Connected to: localhost:12100 (HTTPS)
```

## Step 13: Test Full Production Features over HTTPS

### Test Team Management (Authenticated + HTTPS)

```bash
# Create a test team
rocketship team create "GlobalBank Engineering"

# List teams
rocketship team list

# Add a team member
rocketship team add-member "GlobalBank Engineering" "engineer@globalbank.com" "member" \
  --permissions "tests:read,tests:write,workflows:read"
```

### Test API Token Management (Authenticated + HTTPS)

```bash
# Create an API token
rocketship token create "Production API Token" "GlobalBank Engineering"

# List tokens
rocketship token list
```

### Test Workflow Execution (Authenticated + HTTPS)

```bash
# Run tests via authenticated HTTPS
rocketship run -f examples/simple-http/rocketship.yaml

# Check run history
rocketship list

# Get specific run details
rocketship get <run-id>
```

## Step 14: Verify Security

### Check HTTPS Certificate in Use

```bash
# Check what certificate the engine is using
openssl s_client -connect localhost:12100 -servername globalbank.rocketship.sh < /dev/null 2>/dev/null | openssl x509 -text -noout | grep -A 2 "Issuer:"
```

**Expected Output:**
```
Issuer: C=US, O=Let's Encrypt, CN=R3
Subject: CN=globalbank.rocketship.sh
```

### Verify All Traffic is Encrypted

```bash
# All CLI commands now use HTTPS
rocketship auth status    # HTTPS
rocketship team list      # HTTPS
rocketship run -f test.yaml  # HTTPS
```

## Step 15: Production Readiness Checklist

**✅ Authentication & Authorization:**
- [✅] OIDC authentication with Auth0
- [✅] Organization Admin privileges
- [✅] Team-based RBAC
- [✅] API token management

**✅ Security:**
- [✅] HTTPS with Let's Encrypt certificate
- [✅] All API communication encrypted
- [✅] Certificate automatically renewable (90 days)

**✅ Infrastructure:**
- [✅] Docker multi-stack isolation
- [✅] Production-ready service dependencies
- [✅] Health checks and service recovery
- [✅] Persistent data storage

**✅ Operational:**
- [✅] Isolated development environment
- [✅] Port conflict prevention
- [✅] Easy cleanup and management

## Step 16: Clean Up (When Done Testing)

```bash
# Stop the stack
./.docker/rocketship stop

# Or completely remove everything
./.docker/rocketship clean
```

## Troubleshooting

### Certificate Issues

**"Certificate not found":**
```bash
# Regenerate certificate
rocketship certs generate --domain globalbank.rocketship.sh --email magiusdarrigo@gmail.com --local
```

**"TLS handshake failed":**
```bash
# Check environment variables
echo $ROCKETSHIP_TLS_ENABLED
echo $ROCKETSHIP_TLS_DOMAIN

# Ensure they match the certificate domain
```

### DNS Issues

**"DNS not propagated":**
- Wait longer (up to 15 minutes)
- Check with multiple DNS checkers
- Verify CNAME points to exact tunnel URL

### Docker Issues

**"Service not healthy":**
```bash
# Check service logs
./.docker/rocketship logs engine

# Restart if needed
./.docker/rocketship restart
```

## Summary

You now have a **complete production Rocketship environment** with:

🔐 **Enterprise Authentication:**
- Auth0 OIDC integration
- Organization Admin permissions  
- Team-based RBAC system
- API token management

🔒 **Production HTTPS Security:**
- Let's Encrypt SSL certificate
- All traffic encrypted (CLI ↔ Engine)
- Automatic certificate renewal
- Domain: `globalbank.rocketship.sh`

🐳 **Production Infrastructure:**
- Docker multi-stack isolation
- PostgreSQL for auth + temporal
- Elasticsearch for temporal
- Health monitoring
- Service recovery

🚀 **Full Functionality:**
- Authenticated workflow execution
- Team management commands
- API token operations
- Secure test running

This setup is **enterprise-ready** and demonstrates the complete self-hosted Rocketship solution with both authentication and HTTPS security that customers expect to see working before implementing their own certificate management.

## Next Steps

- **Certificate Renewal**: Set up automatic renewal via cron job
- **Monitoring**: Add observability for production operations
- **Backup**: Configure database backups
- **Scaling**: Consider horizontal scaling for production loads