# Complete Profile System Test Guide

This guide provides a comprehensive, from-scratch test of the Rocketship profile system, including Docker environment setup, HTTPS/TLS configuration, authentication, and all profile features.

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- Make installed
- You're in the rocketship repository with the `add-auth` branch checked out

## Phase 1: Clean Environment Setup

### Step 1.1: Stop Everything and Clean Up

```bash
# Stop any running local servers
rocketship stop server 2>/dev/null || true

# Stop Docker environment if running
./.docker/rocketship stop 2>/dev/null || true
./.docker/rocketship clean 2>/dev/null || true

# Clean up old certificates
rm -rf ~/.rocketship/certs

# Backup and remove existing config
cp ~/.rocketship/config.json ~/.rocketship/config.json.backup 2>/dev/null || true
rm -f ~/.rocketship/config.json

# Clean environment variables
unset ROCKETSHIP_TLS_ENABLED ROCKETSHIP_TLS_DOMAIN
```

### Step 1.2: Build Fresh CLI

```bash
# Build and install the latest CLI with profile support
make clean
make install

# Verify installation
rocketship version
```

## Phase 2: Certificate Generation for Docker

### Step 2.1: Generate Localhost Certificate

We only need one certificate for the Docker self-hosted environment:

```bash
# Generate certificate for localhost (for Docker self-hosted testing)
rocketship certs generate --domain localhost --self-signed
```

### Step 2.2: Verify Certificate

```bash
# Check certificate was created
ls -la ~/.rocketship/certs/localhost/

# Verify certificate details
openssl x509 -in ~/.rocketship/certs/localhost/cert.pem -text -noout | grep Subject:
```

**Expected output**: You should see `Subject: O=Rocketship Self-Signed`

## Phase 3: Docker Environment Setup

### Step 3.1: Configure Docker Environment

```bash
# Navigate to project root
cd /path/to/rocketship

# IMPORTANT: Check if .env.add-auth already exists from previous testing
if [ -f ./.docker/.env.add-auth ]; then
    echo "Found existing .env.add-auth file. Backing up..."
    cp ./.docker/.env.add-auth ./.docker/.env.add-auth.backup
fi

# Create/update the Docker environment file for our branch
# NOTE: Make sure to complete the heredoc by typing EOF on a new line!
cat > ./.docker/.env.add-auth << 'EOF'
### BASE CONFIGURATION ###
COMPOSE_PROJECT_NAME=rocketship-add-auth

### HTTPS/TLS CONFIGURATION ###
# TLS Configuration for HTTPS support
ROCKETSHIP_TLS_ENABLED=true
ROCKETSHIP_TLS_DOMAIN=localhost

# Enable debug logging to troubleshoot
ROCKETSHIP_LOG=DEBUG

### OIDC CONFIGURATION (Optional - for auth testing) ###
# Leave these commented out for now - we'll test without auth first
# ROCKETSHIP_OIDC_ISSUER=https://your-auth0-domain.auth0.com
# ROCKETSHIP_OIDC_CLIENT_ID=your-client-id
# ROCKETSHIP_OIDC_CLIENT_SECRET=your-client-secret
# ROCKETSHIP_ADMIN_EMAILS=admin@example.com
EOF

# Verify the file was created correctly
echo "Checking .env.add-auth contents:"
grep -E "ROCKETSHIP_TLS|COMPOSE_PROJECT" ./.docker/.env.add-auth

# If the Docker stack is already running with old config, we need to restart it
if docker ps | grep -q "rocketship-add-auth"; then
    echo "Docker stack is running with old configuration. Restarting..."
    ./.docker/rocketship stop
fi
```

### Step 3.2: Initialize and Start Docker Stack

```bash
# Initialize the Docker environment
./.docker/rocketship init

# Start the stack
./.docker/rocketship start

# Check status
./.docker/rocketship status

# Get stack information (note the ports)
./.docker/rocketship info
```

**Expected output**:

- Engine API port (e.g., 12100)
- Temporal UI port (e.g., 12480)
- All containers healthy

### Step 3.3: Verify Docker Logs

```bash
# Check engine logs to confirm TLS is enabled
./.docker/rocketship logs engine | grep -i tls

# You should see something like:
# "TLS enabled for gRPC server" domain=localhost
#
# If you see domain=globalbank.rocketship.sh or another domain, then:
# 1. Stop the stack: ./.docker/rocketship stop
# 2. Check your .env file: cat ./.docker/.env.add-auth
# 3. Make sure ROCKETSHIP_TLS_DOMAIN=localhost
# 4. Start the stack again: ./.docker/rocketship start

# Also verify the certificate is being loaded for localhost:
./.docker/rocketship logs engine | grep "loading TLS certificate"
```

## Phase 4: Profile System Testing

### Step 4.1: Test Default Behavior (Local Development)

The system should default to local development when no auth/profiles are configured:

```bash
# Check initial state - should auto-create local profile
rocketship profile list

# Test local development (should work without any setup)
rocketship start server --background
rocketship run -f examples/simple-http/rocketship.yaml
rocketship list
rocketship stop server
rocketship run -af examples/simple-http/rocketship.yaml

# Get run details (use the run ID from list)
LAST_RUN=$(rocketship list | grep PASSED | head -1 | awk '{print $1}')
rocketship get $LAST_RUN
```

**Expected**:

- Local profile exists with localhost:7700 and TLS disabled
- Tests run successfully using local auto mode
- No certificates or authentication needed

### Step 4.2: Create Self-Hosted Profile

```bash
# Get the engine port from docker info (e.g., 12100)
ENGINE_PORT=$(docker ps --format "table {{.Ports}}" | grep "7700/tcp" | sed 's/.*:\([0-9]*\)->7700.*/\1/')
echo "Docker engine port: $ENGINE_PORT"

# Create profile for self-hosted Docker deployment
rocketship profile create self-hosted https://localhost:$ENGINE_PORT

# List profiles
rocketship profile list
```

**Expected**:

- Two profiles: `local` and `self-hosted`
- `self-hosted` shows TLS enabled with domain `localhost`
- `local` remains the default (marked with `*`)

### Step 4.3: Test Self-Hosted Connection

```bash
# Switch to self-hosted profile
rocketship profile use self-hosted

# Try to run a test (should require authentication)
rocketship run -f examples/simple-http/rocketship.yaml
```

**Expected**: "authentication required" error (this confirms HTTPS and auth are working!)

### Step 4.4: Test Fallback to Local

```bash
# Switch back to local profile (the default for development)
rocketship profile use local

# Should work without any authentication
rocketship run -f examples/simple-http/rocketship.yaml --auto
```

**Expected**: Test runs successfully, demonstrating seamless fallback to local development

## Phase 5: Advanced Profile Features

### Step 5.1: Profile Switching and Persistence

```bash
# Create additional test profiles
rocketship connect https://staging.example.com:8443

# List all profiles
rocketship profile list

# Show current profile details
rocketship profile show

# Show specific profile that does not exist
rocketship profile show docker-https

# Delete a profile
rocketship profile delete staging-example-com

# Verify deletion
rocketship profile list
```

### Step 5.2: Command Integration Testing

```bash
# Test run with profile flag (overrides active profile)
rocketship run -f examples/simple-http/rocketship.yaml --profile local --auto

# Test engine flag override (bypasses profile system)
rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:7700 --auto
```

### Step 5.3: Environment Variable Compatibility

```bash
# Test that environment variables still work
ROCKETSHIP_TLS_ENABLED=false rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:7700 --auto

# Test TLS environment override
ROCKETSHIP_TLS_ENABLED=true ROCKETSHIP_TLS_DOMAIN=localhost rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:$ENGINE_PORT
```

## Phase 6: Authentication Testing (Optional)

### Step 6.1: Configure Auth in Docker

If you have Auth0 or another OIDC provider configured:

```bash
# Update Docker environment with auth settings
cat >> ./.docker/.env.add-auth << 'EOF'

### OIDC CONFIGURATION ###
ROCKETSHIP_OIDC_ISSUER=https://your-domain.auth0.com
ROCKETSHIP_OIDC_CLIENT_ID=your-client-id
ROCKETSHIP_OIDC_CLIENT_SECRET=your-client-secret
ROCKETSHIP_ADMIN_EMAILS=youremail@example.com
EOF

# Restart the stack to apply auth settings
./.docker/rocketship restart
```

### Step 6.2: Test Authentication Flow

```bash
# Test auth login with self-hosted profile
rocketship auth login --profile self-hosted

# Check auth status
rocketship auth status --profile self-hosted

# Run authenticated test
rocketship run -f examples/simple-http/rocketship.yaml --profile self-hosted
```

### Step 6.3: Test Profile-Specific Auth Storage

```bash
# Create a cloud profile for comparison
rocketship profile create cloud https://app.rocketship.sh

# Login to different profiles (if you have cloud access)
rocketship auth login --profile cloud
rocketship auth login --profile self-hosted

# Verify separate token storage
rocketship auth status --profile cloud
rocketship auth status --profile self-hosted

# Logout from one profile
rocketship auth logout --profile cloud

# Verify other profile still authenticated
rocketship auth status --profile self-hosted

# Verify local profile needs no auth
rocketship auth status --profile local
```

## Phase 7: End-to-End Workflow Test

### Step 7.1: Complete Multi-Environment Workflow

This simulates the three key scenarios: local development, self-hosted deployment, and cloud deployment.

```bash
# Start fresh
rm -f ~/.rocketship/config.json

# Workflow simulating real usage:

# 1. Developer starts with local development (default behavior)
echo "=== Testing local development (default) ==="
rocketship run -f examples/simple-http/rocketship.yaml --auto
echo "✅ Local development works - no setup required"

# 2. Connect to self-hosted environment (Docker with auth)
echo "=== Testing self-hosted deployment ==="
ENGINE_PORT=$(docker ps --format "table {{.Ports}}" | grep "7700/tcp" | sed 's/.*:\([0-9]*\)->7700.*/\1/')
rocketship connect https://localhost:$ENGINE_PORT --name self-hosted
echo "✅ Connected to self-hosted"

# 3. Test self-hosted (should require auth)
rocketship run -f examples/simple-http/rocketship.yaml --profile self-hosted
echo "✅ Self-hosted correctly requires authentication"

# 4. Test profile switching
echo "=== Testing profile switching ==="
rocketship profile use local
rocketship run -f examples/simple-http/rocketship.yaml --auto
echo "✅ Switched back to local development seamlessly"

# 5. Compare environments
echo "=== Comparing environments ==="
echo "Local runs:"
rocketship list --profile local

echo "Self-hosted runs:"
rocketship list --profile self-hosted

# 6. Show current profile system state
echo "=== Final profile state ==="
rocketship profile list
```

### Step 7.2: Verify Configuration Persistence

```bash
# Check config file
cat ~/.rocketship/config.json | jq .

# Restart shell to verify persistence
exec $SHELL

# Verify profiles persist
rocketship profile list

# Verify active profile persists
rocketship profile show
```

## Phase 8: Cleanup

### Step 8.1: Stop Services

```bash
# Stop local server if running
rocketship stop server

# Stop Docker stack
./.docker/rocketship stop

# Clean Docker resources (optional)
./.docker/rocketship clean
```

### Step 8.2: Restore Original Config (if desired)

```bash
# Restore original config if you had one
mv ~/.rocketship/config.json.backup ~/.rocketship/config.json 2>/dev/null || true
```

## Verification Checklist

### ✅ Profile Management

- [ ] Profile creation detects TLS from HTTPS URLs
- [ ] Profile names auto-generated from URLs
- [ ] Profile switching updates default
- [ ] Profile deletion works (except local)
- [ ] Profile details show correctly

### ✅ Command Integration

- [ ] `run` command uses active profile
- [ ] `list` command uses active profile
- [ ] `get` command uses active profile
- [ ] `--profile` flag overrides active profile
- [ ] `--engine` flag bypasses profiles

### ✅ TLS/HTTPS Support

- [ ] HTTPS URLs enable TLS automatically
- [ ] Self-signed certificates work
- [ ] TLS connections to Docker work
- [ ] Non-TLS local connections work

### ✅ Authentication (if configured)

- [ ] Auth login works per profile
- [ ] Tokens stored separately per profile
- [ ] Auth status shows correct info
- [ ] Authenticated requests succeed

### ✅ User Experience

- [ ] No need to specify connection details repeatedly
- [ ] Easy switching between environments
- [ ] Clear error messages
- [ ] Configuration persists between sessions

## Troubleshooting

### Common Issues and Solutions

1. **Wrong TLS Domain in Docker Logs**

   ```bash
   # If you see "domain=globalbank.rocketship.sh" instead of "domain=localhost":

   # Check current .env file
   cat ./.docker/.env.add-auth | grep ROCKETSHIP_TLS_DOMAIN

   # If wrong domain, fix it:
   sed -i '' 's/ROCKETSHIP_TLS_DOMAIN=.*/ROCKETSHIP_TLS_DOMAIN=localhost/' ./.docker/.env.add-auth

   # Restart Docker stack
   ./.docker/rocketship stop
   ./.docker/rocketship start

   # Verify the fix
   ./.docker/rocketship logs engine | grep -i tls
   ```

2. **Certificate Errors**

   ```bash
   # Regenerate certificates
   rm -rf ~/.rocketship/certs/localhost
   rocketship certs generate --domain localhost --self-signed

   # Check permissions
   chmod -R 755 ~/.rocketship/certs/
   ```

3. **Connection Refused**

   ```bash
   # Check Docker stack
   ./.docker/rocketship status
   docker ps

   # Check ports
   netstat -an | grep LISTEN | grep 12100
   ```

4. **Authentication Errors**

   ```bash
   # Check if auth is required
   ./.docker/rocketship logs engine | grep -i auth

   # Try without auth first
   # Comment out OIDC vars in .env.add-auth and restart
   ```

5. **Profile Not Found**

   ```bash
   # List all profiles
   rocketship profile list

   # Check config directly
   cat ~/.rocketship/config.json
   ```

6. **Heredoc Command Not Completing**

   ```bash
   # If you see "heredoc>" prompt when creating .env file, type:
   EOF

   # Then press Enter to complete the command
   # Verify the file was created:
   cat ./.docker/.env.add-auth
   ```

## Summary

This comprehensive test guide covers:

1. **Complete environment setup** from scratch
2. **Certificate generation** for HTTPS/TLS
3. **Docker stack configuration** with TLS support
4. **Profile system testing** including all features
5. **Authentication integration** (optional)
6. **Real-world workflow simulation**
7. **Troubleshooting common issues**

The profile system transforms Rocketship into a true multi-environment testing platform, eliminating the need to specify connection details on every command and providing seamless switching between local development, staging, and production environments.
