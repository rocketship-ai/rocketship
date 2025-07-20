# Auth0 Testing Guide for Rocketship Self-Hosting

This comprehensive guide walks you through testing Rocketship's complete authentication system using Auth0 as your OIDC provider. This simulates the exact experience an enterprise would have when self-hosting Rocketship with their existing identity provider.

## 🎯 Overview

You'll test:
- **PKCE OAuth2 flow** with Auth0 integration
- **Organization Admin setup** via email-based configuration
- **Team creation and management** with Buildkite-inspired permissions
- **CLI authentication** with secure token storage
- **API token system** for CI/CD integration
- **Permission enforcement** across all operations

## 📋 Prerequisites

- Docker and Docker Compose installed
- Auth0 account (free tier works perfectly)
- Terminal access with bash/zsh
- Modern web browser

## 🔧 Step 1: Auth0 Setup

### 1.1 Create Auth0 Account

1. Go to [auth0.com](https://auth0.com) → **Sign Up**
2. Choose **Personal** plan (free)
3. Create tenant name: `your-name-rocketship-test` (e.g., `john-rocketship-test`)
4. Choose region closest to you

### 1.2 Create Application

1. Navigate to **Applications** → **Create Application**
2. **Name**: `Rocketship CLI Test`
3. **Application Type**: **Single Page Application** (this enables PKCE)
4. Click **Create**

### 1.3 Configure Application Settings

In your new application's **Settings** tab:

```
# Basic Information
Name: Rocketship CLI Test
Domain: your-tenant.auth0.com (note this down)
Client ID: [copy this - you'll need it]

# Application URIs
Allowed Callback URLs:
http://localhost:8000/callback

Allowed Logout URLs:
http://localhost:8000/callback

Allowed Web Origins:
http://localhost:8000

# Advanced Settings → Grant Types
✓ Authorization Code
✓ Refresh Token

# Advanced Settings → OAuth
✓ OIDC Conformant: ON (should be default)
✓ PKCE: ON (should be default for SPA)
```

Click **Save Changes**

### 1.4 Create Test User

1. Go to **User Management** → **Users** → **Create User**
2. **Email**: Use your real email (you'll need to access it)
3. **Password**: Set a secure password
4. **Connection**: Username-Password-Authentication
5. Click **Create**

**Note your credentials:**
- Email: `your-email@example.com`
- Password: `your-secure-password`

## 🚀 Step 2: Rocketship Environment Setup

### 2.1 Clone and Navigate

```bash
# Clone the repository
git clone https://github.com/rocketship-ai/rocketship.git
cd rocketship

# Ensure you're on the auth branch
git checkout add-auth

# Navigate to Docker environment
cd .docker
```

### 2.2 Configure Environment Variables

Create your environment configuration:

```bash
# Set Auth0 configuration (replace with your values)
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-auth0-client-id"
export ROCKETSHIP_OIDC_CLIENT_SECRET=""  # Empty for PKCE flow (SPA apps)
export ROCKETSHIP_ADMIN_EMAILS="your-email@example.com"

# Verify configuration
echo "Issuer: $ROCKETSHIP_OIDC_ISSUER"
echo "Client ID: $ROCKETSHIP_OIDC_CLIENT_ID"
echo "Client Secret: ${ROCKETSHIP_OIDC_CLIENT_SECRET:-'(empty - PKCE flow)'}"
echo "Admin Emails: $ROCKETSHIP_ADMIN_EMAILS"
```

### 2.3 Initialize and Start Stack

```bash
# Initialize your isolated testing environment
./rocketship init

# Start the complete authentication-enabled stack
./rocketship start

# Wait for all services to be healthy (2-3 minutes)
./rocketship status
```

**Expected Output:**
```
Stack: rocketship-add-auth
Status: ✓ Running
Services:
  ✓ temporal (healthy)
  ✓ temporal-ui (healthy)
  ✓ postgresql (healthy)
  ✓ elasticsearch (healthy)
  ✓ engine (healthy)
  ✓ worker (healthy)
  ✓ auth-postgres (healthy)
  ✓ postgres-test (healthy)
  ✓ mysql-test (healthy)

Temporal UI: http://localhost:12480
Engine API: localhost:12100
```

### 2.4 Build CLI with Authentication

```bash
# Navigate back to project root
cd ..

# Build CLI with embedded binaries
make install

# Verify installation
rocketship --version
```

## 🔐 Step 3: Authentication Testing

### 3.1 Test Authentication Status (Pre-Login)

```bash
# Check status before authentication
rocketship auth status
```

**Expected Output:**
```
Status: Not authenticated (authentication not configured)
```

Wait, this means auth isn't configured yet. Let's verify environment variables are set:

```bash
# Check environment variables
echo $ROCKETSHIP_OIDC_ISSUER
echo $ROCKETSHIP_OIDC_CLIENT_ID
echo $ROCKETSHIP_ADMIN_EMAILS

# If empty, re-export them
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-auth0-client-id"
export ROCKETSHIP_ADMIN_EMAILS="your-email@example.com"
```

### 3.2 First Admin Login

```bash
# Attempt authentication
rocketship auth login
```

**Expected Flow:**
1. CLI opens your browser to Auth0
2. You're redirected to Auth0 login page
3. Enter your test user credentials
4. Auth0 redirects back to CLI
5. CLI displays success message

**Expected Output:**
```
Please visit the following URL to authenticate:

  https://your-tenant.auth0.com/authorize?client_id=...

Waiting for authentication... (timeout: 5 minutes)

✓ Authentication successful!
Welcome, Your Name (your-email@example.com)
Admin role: Yes
```

### 3.3 Verify Admin Status

```bash
# Check authentication status
rocketship auth status
```

**Expected Output:**
```
Status: Authenticated
User: Your Name (your-email@example.com)
Subject: auth0|user_id_string
Admin role: Yes
```

**🎉 Success!** You're now authenticated as an Organization Admin.

## 👥 Step 4: Team Management Testing

### 4.1 Create Teams

```bash
# Create development teams
rocketship team create "Backend Team"
rocketship team create "Frontend Team"
rocketship team create "QA Team"

# List teams to verify
rocketship team list
```

**Expected Output:**
```
Teams:
- Backend Team (created: 2025-01-19T...)
- Frontend Team (created: 2025-01-19T...)
- QA Team (created: 2025-01-19T...)
```

### 4.2 Add Team Members with Different Permissions

```bash
# Add a senior developer with comprehensive permissions
rocketship team add-member "Backend Team" senior.dev@example.com admin \
  --permissions "tests:read,tests:write,tests:manage,workflows:read,workflows:write,workflows:manage,repositories:read,repositories:write,team:members:read,team:members:write"

# Add a junior developer with basic permissions
rocketship team add-member "Backend Team" junior.dev@example.com member \
  --permissions "tests:read,tests:write,workflows:read,workflows:write"

# Add a QA engineer with testing focus
rocketship team add-member "QA Team" qa.engineer@example.com admin \
  --permissions "tests:read,tests:write,tests:manage,workflows:read,workflows:write,team:members:manage"

# Add a frontend developer
rocketship team add-member "Frontend Team" frontend.dev@example.com member \
  --permissions "tests:read,tests:write,workflows:read"

# List team members
rocketship team list
```

**Expected Output:**
```
Teams:
- Backend Team (created: 2025-01-19T...)
  Members: 2
- Frontend Team (created: 2025-01-19T...)
  Members: 1
- QA Team (created: 2025-01-19T...)
  Members: 1
```

### 4.3 Test Permission Variations

```bash
# Test wildcard permissions (all test permissions)
rocketship team add-member "QA Team" lead.qa@example.com admin \
  --permissions "tests:*,workflows:*,team:members:*"

# Test read-only permissions
rocketship team add-member "Frontend Team" intern@example.com member \
  --permissions "tests:read,workflows:read"
```

## 🧪 Step 5: Test Execution with Authentication

### 5.1 Test Unauthenticated Access (Should Work)

```bash
# Logout temporarily to test backward compatibility
rocketship auth logout

# Verify logout
rocketship auth status

# Try running a test without authentication
rocketship run -af examples/simple-http/rocketship.yaml
```

**Expected Behavior:** Test should run successfully, proving backward compatibility.

### 5.2 Test Authenticated Access

```bash
# Login again
rocketship auth login

# Verify authentication
rocketship auth status

# Run test with authentication
rocketship run -af examples/simple-http/rocketship.yaml
```

**Expected Behavior:** Test runs successfully with authenticated user context.

### 5.3 Test Engine Connection with Authentication

```bash
# Start engine in background mode to test gRPC authentication
rocketship start server --background

# Run test against authenticated engine
rocketship run -f examples/simple-http/rocketship.yaml

# Check test runs (requires authenticated access)
rocketship list runs

# Get specific run details
rocketship get <run-id-from-list>

# Stop background server
rocketship stop server
```

## 🔑 Step 6: API Token System Testing

### 6.1 Create API Tokens for CI/CD

```bash
# Create a CI/CD token for the Backend Team
rocketship token create "CI-Backend-Pipeline" \
  --team "Backend Team" \
  --permissions "tests:write,workflows:write" \
  --expires-in 30d

# Create a QA automation token
rocketship token create "QA-Automation" \
  --team "QA Team" \
  --permissions "tests:read,tests:write,tests:manage" \
  --expires-in 90d

# List all tokens
rocketship token list
```

**Expected Output:**
```
API Tokens:
- CI-Backend-Pipeline (Backend Team) - expires in 30 days
- QA-Automation (QA Team) - expires in 90 days
```

### 6.2 Test API Token Usage

```bash
# Get the token value (will be displayed during creation)
export ROCKETSHIP_API_TOKEN="rs_token_value_from_creation"

# Test token-based authentication
rocketship run -f examples/simple-http/rocketship.yaml \
  --engine localhost:12100

# The test should run using token authentication instead of OIDC
```

## 🎨 Step 7: Advanced Permission Testing

### 7.1 Test Permission Enforcement

Create a test user with limited permissions and simulate their access:

```bash
# Create a read-only user
rocketship team add-member "Frontend Team" readonly@example.com member \
  --permissions "tests:read"

# Note: Since we can't actually login as different users in this test,
# we'll verify the permission structure is correct
rocketship team list

# Verify permissions are stored correctly by checking team details
```

### 7.2 Test Buildkite-Style Permission Patterns

```bash
# Test various permission combinations that match Buildkite patterns
rocketship team add-member "Backend Team" devops@example.com admin \
  --permissions "tests:*,workflows:*,repositories:*,team:members:*"

# Test granular permissions
rocketship team add-member "QA Team" manual.tester@example.com member \
  --permissions "tests:read,workflows:read"

# Test team admin permissions
rocketship team add-member "Frontend Team" team.lead@example.com admin \
  --permissions "tests:*,workflows:*,team:members:read,team:members:write,team:members:manage"
```

## 🔍 Step 8: Integration and Monitoring Testing

### 8.1 Test Temporal Integration

```bash
# Access Temporal UI to see authenticated workflow execution
echo "Open Temporal UI: http://localhost:12480"

# Run a test and observe it in Temporal UI
rocketship run -af examples/simple-http/rocketship.yaml

# Check that user information is properly associated with workflow execution
```

### 8.2 Test Docker Multi-Stack Isolation

```bash
# Verify your isolated environment
./rocketship info

# Check that authentication is properly isolated to your stack
docker ps | grep rocketship
```

### 8.3 Test Database Integration

```bash
# Verify authentication database has proper data
cd .docker
docker exec -it rocketship-add-auth-auth-postgres-1 \
  psql -U authuser -d auth -c "SELECT email, is_admin FROM users;"

# Check teams and team members
docker exec -it rocketship-add-auth-auth-postgres-1 \
  psql -U authuser -d auth -c "SELECT name FROM teams;"

docker exec -it rocketship-add-auth-auth-postgres-1 \
  psql -U authuser -d auth -c "SELECT t.name, tm.user_id, tm.role, tm.permissions FROM teams t JOIN team_members tm ON t.id = tm.team_id;"
```

**Expected Output:**
```
# Users table
        email         | is_admin 
---------------------+----------
 your-email@example.com |        t

# Teams table
     name      
--------------
 Backend Team
 Frontend Team
 QA Team

# Team members with permissions
     name      |          user_id           | role  |                    permissions                    
--------------+----------------------------+-------+------------------------------------------------
 Backend Team | auth0|user_id_string      | admin | {tests:read,tests:write,tests:manage,...}
 QA Team      | qa.engineer@example.com    | admin | {tests:read,tests:write,tests:manage,...}
```

## 🚨 Step 9: Error Handling and Edge Cases

### 9.1 Test Token Expiration

```bash
# Create a short-lived token for testing
rocketship token create "Test-Expiry" \
  --team "Backend Team" \
  --permissions "tests:read" \
  --expires-in 1m

# Wait 2 minutes, then try to use the token
# (This would require actual wait time - document expected behavior)
```

### 9.2 Test Invalid Permissions

```bash
# Try to add invalid permissions (should fail gracefully)
rocketship team add-member "Backend Team" test@example.com member \
  --permissions "invalid:permission,tests:read"
```

**Expected Output:**
```
Error: invalid permission: invalid:permission. Valid permissions: tests:read, tests:write, tests:manage, workflows:read, workflows:write, workflows:manage, repositories:read, repositories:write, repositories:manage, team:members:read, team:members:write, team:members:manage
```

### 9.3 Test Network Issues

```bash
# Test behavior when Auth0 is unreachable (simulate by using wrong issuer)
export ROCKETSHIP_OIDC_ISSUER="https://invalid-tenant.auth0.com/"
rocketship auth login

# Restore correct issuer
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
```

## 🧹 Step 10: Cleanup and Reset Testing

### 10.1 Test Logout Functionality

```bash
# Test proper logout
rocketship auth logout

# Verify logout cleared local tokens
rocketship auth status

# Verify you can't access authenticated endpoints
rocketship list runs
```

### 10.2 Clean Up Test Environment

```bash
# Stop your stack
cd .docker
./rocketship stop

# Remove all containers and volumes (optional)
./rocketship clean

# Verify cleanup
docker ps | grep rocketship
```

## ✅ Success Criteria Checklist

Mark each item as ✅ when successfully tested:

### Authentication Flow
- [ ] Auth0 OIDC configuration works
- [ ] PKCE flow completes successfully
- [ ] Admin user setup via email list works
- [ ] Login/logout cycle works properly
- [ ] Token storage and retrieval works

### Team Management
- [ ] Organization Admin can create teams
- [ ] Team members can be added with various roles
- [ ] Buildkite-style permissions work correctly
- [ ] Permission validation prevents invalid permissions
- [ ] Team listing shows correct information

### API Integration
- [ ] Authenticated test execution works
- [ ] API tokens can be created and used
- [ ] Token expiration is handled properly
- [ ] gRPC authentication interceptors work
- [ ] Database stores auth data correctly

### Enterprise Features
- [ ] OIDC provider integration works end-to-end
- [ ] Multi-stack isolation maintains authentication
- [ ] Backward compatibility (no auth) still works
- [ ] All permission levels function as expected
- [ ] Audit trail is properly maintained

### Error Handling
- [ ] Invalid permissions are rejected
- [ ] Network issues are handled gracefully
- [ ] Token expiration is managed properly
- [ ] Logout clears authentication state

## 🎉 Completion

If all checkboxes are ✅, you have successfully:

1. **Simulated enterprise deployment** using Auth0 as OIDC provider
2. **Tested the complete authentication system** from admin setup to team management
3. **Verified PKCE OAuth2 flow** works with external identity providers
4. **Validated Buildkite-inspired RBAC** with granular permissions
5. **Confirmed backward compatibility** for users without authentication
6. **Demonstrated enterprise-ready features** that work with any OIDC provider

## 🔄 Next Steps

This testing validates that Rocketship is ready for:

- **Enterprise deployment** with existing identity providers (Okta, Azure AD, etc.)
- **Production self-hosting** with proper authentication and RBAC
- **CI/CD integration** using API tokens
- **Team-based development workflows** with appropriate access controls

Your testing proves the system works exactly like the Twilio Okta integration pattern you wanted to achieve! 🚀

## 🛠️ Troubleshooting

### Common Auth0 Issues

**OAuth2 "access_denied" "Unauthorized" Error:**
- **Cause**: Client secret is set when it should be empty for PKCE flow
- **Fix**: Ensure `ROCKETSHIP_OIDC_CLIENT_SECRET=""` (empty value)
- **Verify**: Check `.env.add-auth` file has `ROCKETSHIP_OIDC_CLIENT_SECRET=` with no value

**"unsupported protocol scheme" Error:**
- **Cause**: Missing `https://` in OIDC issuer URL
- **Fix**: Ensure issuer URL starts with `https://`
- **Example**: `https://your-tenant.auth0.com/` (note trailing slash)

**Authentication not configured:**
- **Cause**: Environment variables not exported or stack not restarted
- **Fix**: Re-export variables and restart stack:
```bash
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-client-id"
export ROCKETSHIP_OIDC_CLIENT_SECRET=""
cd .docker && ./rocketship stop && ./rocketship start
```

## 📞 Support

If you encounter issues during testing:

1. Check the [AUTH_README.md](./AUTH_README.md) for detailed authentication documentation
2. Review [.docker/README.md](./.docker/README.md) for Docker environment help  
3. Use `ROCKETSHIP_LOG=DEBUG` for detailed debugging output
4. Verify Auth0 configuration matches the requirements exactly

This comprehensive test validates that your self-hosted Rocketship deployment will work seamlessly with enterprise identity providers!