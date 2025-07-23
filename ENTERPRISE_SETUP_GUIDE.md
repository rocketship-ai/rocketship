# Rocketship Enterprise Self-Hosting Guide

**Complete step-by-step guide for IT administrators to deploy Rocketship with authentication, HTTPS, and team management.**

## 🎯 What You'll Get

By following this guide, you'll have a complete enterprise-ready Rocketship deployment with:

- ✅ **HTTPS with SSL certificates** (self-signed or Let's Encrypt)
- ✅ **Enterprise authentication** (Auth0, Okta, Azure AD, Google Workspace)
- ✅ **Team-based access control** with granular permissions
- ✅ **Docker containerized deployment** with isolation
- ✅ **API token management** for CI/CD integration
- ✅ **Repository management** with CODEOWNERS enforcement

## 📋 Prerequisites

### Required Software

- Docker and Docker Compose
- Git
- A domain name (for production Let's Encrypt certificates)
- Access to an OIDC provider (Auth0, Okta, Azure AD, etc.)

### Required Access

- Admin access to your DNS provider
- Admin access to your identity provider
- Port access: 7700, 8080, 5432, 9200 (or auto-allocated ports)

### Time Requirement

- Initial setup: 30-45 minutes
- Testing and verification: 15-30 minutes

---

## 🚀 Quick Start (30 Minutes)

If you want to get up and running quickly with Auth0:

```bash
# 1. Clone repository
git clone https://github.com/rocketship-ai/rocketship.git
cd rocketship
# create and checkout your own local branch (this will be your "stack" name)
git checkout -b rocketship-enterprise-test

# 2 Wipe out any previous rocketship artifacts
./.docker/rocketship clean

# 3. Install the rocketship CLI
make install

# 4. Create the docker compose network environment
./.docker/rocketship init
./.docker/rocketship start

# 5. Generate localhost certificate for quick self host testing
rocketship certs generate --domain localhost --self-signed

# 6. Connect to the self-hosted environment
rocketship connect https://localhost:12100

# 7. Test authentication
rocketship auth login

# 8. Create a team
rocketship team create "Engineering"

# 9. Connect a repository to the team
rocketship repo connect https://github.com/rocketship-ai/rocketship.git --team "Engineering"

# 10. Add a team member
rocketship team add-member "Engineering" "admin@yourcompany.com"

# 11. Run a test
rocketship run -f examples/simple-http/rocketship.yaml
```

For full production setup with Let's Encrypt certificates, continue with the detailed steps below.

---

## 🔐 Step 1: OIDC Provider Setup

Choose your organization's identity provider and follow the appropriate setup:

### Auth0 Setup Example

1. **Create Auth0 Account**

   - Go to [auth0.com](https://auth0.com) → Sign Up
   - Choose tenant name: `yourcompany-rocketship`

2. **Create Application**

   - Applications → Create Application
   - Name: `Rocketship Enterprise`
   - Type: **Native**

3. **Configure Application**

   ```
   Allowed Callback URLs: http://localhost:8000/callback
   Allowed Logout URLs: http://localhost:8000/callback
   Allowed Web Origins: http://localhost:8000
   Grant Types: ✓ Authorization Code, ✓ Refresh Token
   ```

4. **Note Your Configuration**
   ```bash
   Domain: your-tenant.auth0.com
   Client ID: [copy from Auth0 dashboard]
   Client Secret: [leave empty for PKCE]
   ```

## 🔒 Step 2: SSL Certificate Setup

Choose your certificate approach based on your environment:

### Option A: Self-Signed Certificates (Development)

```bash
# Generate self-signed certificate
rocketship certs generate --domain localhost --self-signed
```

### Option B: Bring Your Own Certificate (BYOC)

**For enterprises with existing certificates or using different CA providers**

#### Method 1: Free Certificate from ZeroSSL

1. **Get Free Certificate from ZeroSSL:**

   ```bash
   # Visit https://zerossl.com
   # Create free account
   # Generate certificate for your domain
   # Download certificate files
   ```

2. **Import Certificate:**
   ```bash
   # Import the certificate into Rocketship
   rocketship certs import \
     --domain rocketship.yourcompany.com \
     --cert-file /path/to/certificate.crt \
     --key-file /path/to/private.key \
     --chain-file /path/to/ca_bundle.crt
   ```

#### Method 2: Corporate Certificate Authority

1. **Obtain Certificate from Your CA:**

   - Request certificate for `rocketship.yourcompany.com`
   - Download certificate and private key files
   - Ensure certificate includes intermediate chain

2. **Import Certificate:**
   ```bash
   rocketship certs import \
     --domain rocketship.yourcompany.com \
     --cert-file /path/to/your-cert.pem \
     --key-file /path/to/your-key.pem \
     --chain-file /path/to/intermediate-chain.pem
   ```

**Verify imported certificate:**

```bash
# Check certificate status
rocketship certs status

# Should show your imported certificate with proper validity dates
```

---

## 🐳 Step 3: Docker Environment Setup

### 3.1 Clean Start (Important!)

**For a fresh deployment**, always start with a clean environment to avoid database conflicts:

```bash
# Clean any previous deployment (removes containers and persistent data)
./.docker/rocketship clean
```

### 3.2 Configure Environment

Create the environment configuration for your deployment:

```bash
# Navigate to Docker directory
cd .docker

# Create environment configuration
cat > .env.<YOUR_STACK_NAME> << 'EOF'
### BASE CONFIGURATION ###
COMPOSE_PROJECT_NAME=rocketship-production

### OIDC CONFIGURATION ###
# Replace with your identity provider settings
ROCKETSHIP_OIDC_ISSUER=https://your-tenant.auth0.com/
ROCKETSHIP_OIDC_CLIENT_ID=your-client-id
ROCKETSHIP_OIDC_CLIENT_SECRET=
ROCKETSHIP_ADMIN_EMAILS=admin@yourcompany.com,it@yourcompany.com

### HTTPS/TLS CONFIGURATION ###
ROCKETSHIP_TLS_ENABLED=true
ROCKETSHIP_TLS_DOMAIN=localhost

### DATABASE CONFIGURATION ###
# Auth Database
AUTH_DB_HOST=auth-postgres
AUTH_DB_PORT=5432
AUTH_DB_NAME=auth
AUTH_DB_USER=authuser
AUTH_DB_PASSWORD=authpass

# Temporal Database
TEMPORAL_DB_HOST=temporal-postgres
TEMPORAL_DB_PORT=5432
TEMPORAL_DB_NAME=temporal
TEMPORAL_DB_USER=temporal
TEMPORAL_DB_PASSWORD=temporal

### DEVELOPMENT SETTINGS ###
ROCKETSHIP_LOG=INFO
EOF
```

**For Let's Encrypt certificates**, update the TLS domain:

```bash
# Update for your domain
sed -i 's/ROCKETSHIP_TLS_DOMAIN=localhost/ROCKETSHIP_TLS_DOMAIN=rocketship.yourcompany.com/' .env.<YOUR_STACK_NAME>
```

### 3.3 Initialize and Start Services

```bash
# Initialize the isolated environment
./rocketship init

# Start all services
./rocketship start

# Wait for all services to be healthy (2-3 minutes)
./rocketship status
```

**Expected Output:**

```
Stack: <YOUR_STACK_NAME>
Status: ✓ Running
Services:
  ✓ temporal (healthy)
  ✓ temporal-ui (healthy)
  ✓ postgresql (healthy)
  ✓ elasticsearch (healthy)
  ✓ engine (healthy)
  ✓ worker (healthy)
  ✓ auth-postgres (healthy)

Temporal UI: http://localhost:8080
Engine API: localhost:7700
```

### 3.4 Verify HTTPS Setup

```bash
# Check engine logs for TLS confirmation
./rocketship logs engine | grep -i tls

# Should see: "TLS enabled for gRPC server" domain=localhost
```

---

## 🔑 Step 4: Authentication and Admin Setup

### 4.1 Connect to Your Deployment

After starting the Docker services, you need to create a connection profile:

```bash
# Connect CLI to your Docker deployment (use HTTPS since TLS is enabled)
rocketship connect https://localhost:12100 --name production
```

### 4.2 First Admin Login

```bash
# Login as organization admin
rocketship auth login
```

**Expected Flow:**

1. Browser opens to your identity provider
2. Login with your admin credentials
3. Redirect back to CLI
4. Success message with admin privileges confirmed

```bash
# Verify admin status
./rocketship auth status
```

**Expected Output:**

```
✅ Authenticated as: admin@yourcompany.com
🏢 Organization Role: Organization Admin
🎫 Token expires: [timestamp]
🔗 Connected to: localhost:7700 (HTTPS)
```

### 4.3 Test HTTPS Connection

```bash
# Test basic functionality over HTTPS
rocketship validate examples/simple-http/rocketship.yaml
rocketship run -f examples/simple-http/rocketship.yaml

# Check successful runs
rocketship list
```

---

## 👥 Step 5: Team and Repository Management

### 5.1 Create Teams

```bash
# Create teams for your organization
rocketship team create "Platform Engineering"
rocketship team create "Backend Development"
rocketship team create "Frontend Development"
rocketship team create "QA Engineering"
rocketship team create "DevOps"

# List teams
rocketship team list
```

### 5.2 Add Team Members

```bash
# Add platform engineers with full permissions
./rocketship team add-member "Platform Engineering" "platform-lead@yourcompany.com" admin \
  --permissions "tests:*,repositories:*,team:members:*"

# Add backend developers
./rocketship team add-member "Backend Development" "backend-dev@yourcompany.com" member \
  --permissions "tests:read,tests:write,repositories:read"

# Add frontend developers
./rocketship team add-member "Frontend Development" "frontend-dev@yourcompany.com" member \
  --permissions "tests:read,tests:write,repositories:read"

# Add QA engineers with testing focus
./rocketship team add-member "QA Engineering" "qa-lead@yourcompany.com" admin \
  --permissions "tests:*,repositories:read,team:members:read,team:members:write"

# Add DevOps with infrastructure permissions
./rocketship team add-member "DevOps" "devops@yourcompany.com" admin \
  --permissions "tests:*,repositories:*,team:members:read"

# Verify team setup
./rocketship team list
```

### 5.3 Repository Management

```bash
# Add your company's repositories
./rocketship repo add "https://github.com/yourcompany/backend-api" --enforce-codeowners
./rocketship repo add "https://github.com/yourcompany/frontend-app" --enforce-codeowners
./rocketship repo add "https://github.com/yourcompany/shared-components"

# Assign teams to repositories
./rocketship repo assign "https://github.com/yourcompany/backend-api" "Backend Development"
./rocketship repo assign "https://github.com/yourcompany/backend-api" "Platform Engineering"
./rocketship repo assign "https://github.com/yourcompany/frontend-app" "Frontend Development"
./rocketship repo assign "https://github.com/yourcompany/shared-components" "Platform Engineering"

# Assign QA team to all repositories for testing
./rocketship repo assign "https://github.com/yourcompany/backend-api" "QA Engineering"
./rocketship repo assign "https://github.com/yourcompany/frontend-app" "QA Engineering"

# Verify repository assignments
./rocketship repo list
./rocketship repo show "https://github.com/yourcompany/backend-api"
```

---

## 🔑 Step 6: API Token Management for CI/CD

### 6.1 Create CI/CD Tokens

```bash
# Create production CI token for backend
rocketship token create "Backend-CI-Production" \
  --team "Backend Development" \
  --permissions "tests:write" \
  --expires-in 90d

# Create staging CI token for frontend
rocketship token create "Frontend-CI-Staging" \
  --team "Frontend Development" \
  --permissions "tests:write" \
  --expires-in 60d

# Create QA automation token
rocketship token create "QA-Automation" \
  --team "QA Engineering" \
  --permissions "tests:read,tests:write,tests:manage" \
  --expires-in 180d

# List all tokens
rocketship token list
```

### 6.2 CI/CD Integration

**For GitHub Actions:**

```yaml
# .github/workflows/rocketship-tests.yml
name: Rocketship Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run Rocketship Tests
        env:
          ROCKETSHIP_API_TOKEN: ${{ secrets.ROCKETSHIP_TOKEN }}
          ROCKETSHIP_ENGINE: rocketship.yourcompany.com:7700
          ROCKETSHIP_TLS_ENABLED: "true"
          ROCKETSHIP_TLS_DOMAIN: rocketship.yourcompany.com
        run: |
          # Install rocketship CLI
          curl -L https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64 -o rocketship
          chmod +x rocketship

          # Run tests
          ./rocketship run -f tests/integration.yaml
```

---

## 🧪 Step 7: Running Your First Tests

### 7.1 Run Tests

```bash
# Run your test suite
./rocketship run -f tests/api-integration.yaml

# Check test results
./rocketship list

# Get detailed results (use run ID from list)
./rocketship get <run-id>
```

---

## 🔄 Step 8: User Management

**Adding New Users:**

```bash
# Add user to appropriate team
./rocketship team add-member "Backend Development" "newdev@yourcompany.com" member \
  --permissions "tests:read,tests:write"
```

**Removing Users:**

```bash
# Remove user from teams (done through your identity provider)
# Revoke any personal API tokens if needed
./rocketship token list
./rocketship token revoke <token-id>
```
