# RBAC and CODEOWNERS Verification Guide

This guide provides comprehensive verification steps for all implemented RBAC and CODEOWNERS functionality in Rocketship.

## Prerequisites

Choose one of two approaches for testing:

### Option A: Multi-Stack Development Environment (RECOMMENDED for isolated testing)
```bash
# Initialize and start isolated stack
./.docker/rocketship init
./.docker/rocketship start

# Set environment variables
export ROCKETSHIP_OIDC_ISSUER="https://dev-0ankenxegmh7xfjm.us.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-client-id" 
export ROCKETSHIP_OIDC_CLIENT_SECRET=""  # Empty for PKCE flow
export ROCKETSHIP_ADMIN_EMAILS="your-email@example.com"

# Use the isolated stack's CLI (includes authentication commands)
alias rocketship="./.docker/rocketship"
```

### Option B: Self-Hosted Production Pattern (for enterprise testing)
```bash
# Build and install standalone CLI
make install

# Start self-hosted Docker services
docker-compose -f .docker/docker-compose.yaml up -d

# Set environment variables for standalone CLI
export ROCKETSHIP_OIDC_ISSUER="https://dev-0ankenxegmh7xfjm.us.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-client-id"
export ROCKETSHIP_OIDC_CLIENT_SECRET=""  # Empty for PKCE flow
export ROCKETSHIP_ADMIN_EMAILS="your-email@example.com"
export ROCKETSHIP_ENGINE_ADDRESS="localhost:7700"  # Connect to self-hosted engine

# Use standalone CLI
rocketship auth login
```

**For this verification, we'll use Option A** since it provides a complete isolated environment with all authentication features.

## Quick Start Verification

### Using Multi-Stack Environment (RECOMMENDED)
```bash
# 1. Initialize and start isolated stack
./.docker/rocketship init
./.docker/rocketship start

# 2. Set environment variables
export ROCKETSHIP_OIDC_ISSUER="https://dev-0ankenxegmh7xfjm.us.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-client-id"
export ROCKETSHIP_OIDC_CLIENT_SECRET=""
export ROCKETSHIP_ADMIN_EMAILS="your-email@example.com"

# 3. Check stack status
./.docker/rocketship status

# 4. Test authentication and RBAC
./.docker/rocketship auth login
./.docker/rocketship team create "Backend Team"
./.docker/rocketship repo add "https://github.com/rocketship-ai/test-codeowners-repo" --enforce-codeowners
./.docker/rocketship repo assign "https://github.com/rocketship-ai/test-codeowners-repo" "Backend Team"
./.docker/rocketship repo show "https://github.com/rocketship-ai/test-codeowners-repo"
```

## Phase 1: Authentication and Basic User Management

### 1.1 Login and Authentication
```bash
# Test login flow
rocketship auth login

# Verify authentication status
rocketship auth status

# Should show: "Authenticated as: your-email@example.com"
```

### 1.2 Admin User Verification
```bash
# Check if you're an admin (should be true if your email is in ROCKETSHIP_ADMIN_EMAILS)
rocketship auth status
# Look for admin status indicators
```

## Phase 2: Team Management

### 2.1 Create Teams
```bash
# Create backend team
rocketship team create "Backend Team"

# Create frontend team  
rocketship team create "Frontend Team"

# Create QA team
rocketship team create "QA Team"

# List all teams
rocketship team list
```

### 2.2 Add Team Members
```bash
# Add yourself to backend team as admin
rocketship team add-member "Backend Team" "your-email@example.com" --role admin

# Add yourself to QA team as member
rocketship team add-member "QA Team" "your-email@example.com" --role member

# List team memberships
rocketship team list
```

## Phase 3: Repository Management

### 3.1 Add Test Repository
```bash
# Add the test repository with CODEOWNERS enforcement
rocketship repo add "https://github.com/rocketship-ai/test-codeowners-repo" --enforce-codeowners

# List repositories
rocketship repo list
```

### 3.2 Assign Teams to Repository
```bash
# Assign backend team to repository
rocketship repo assign "https://github.com/rocketship-ai/test-codeowners-repo" "Backend Team"

# Assign QA team to repository
rocketship repo assign "https://github.com/rocketship-ai/test-codeowners-repo" "QA Team"

# Show repository details with assigned teams
rocketship repo show "https://github.com/rocketship-ai/test-codeowners-repo"
```

## Phase 4: CODEOWNERS Permissions Testing

### 4.1 Create Test YAML Files
Create test files in different directories to test CODEOWNERS enforcement:

```bash
# Create a backend test file
mkdir -p /tmp/test-permissions
cat > /tmp/test-permissions/backend-test.yaml << 'EOF'
name: "Backend Test"
description: "Test for backend directory"
tests:
  - name: "Backend API Test"
    steps:
      - name: "Test backend endpoint"
        plugin: http
        config:
          url: "https://tryme.rocketship.sh/users"
          method: "GET"
        assertions:
          - type: status_code
            expected: 200
EOF

# Create a frontend test file
cat > /tmp/test-permissions/frontend-test.yaml << 'EOF'
name: "Frontend Test"
description: "Test for frontend directory"
tests:
  - name: "Frontend UI Test"
    steps:
      - name: "Test frontend endpoint"
        plugin: http
        config:
          url: "https://tryme.rocketship.sh/health"
          method: "GET"
        assertions:
          - type: status_code
            expected: 200
EOF

# Create a Go file test
cat > /tmp/test-permissions/go-file-test.yaml << 'EOF'
name: "Go File Test"
description: "Test for Go files"
tests:
  - name: "Go Code Test"
    steps:
      - name: "Test Go endpoint"
        plugin: http
        config:
          url: "https://tryme.rocketship.sh/api/version"
          method: "GET"
        assertions:
          - type: status_code
            expected: 200
EOF
```

### 4.2 Test CODEOWNERS Permissions

Now test the permissions resolution with different file paths:

```bash
# Test 1: Backend team should have access to backend/ files
echo "Testing Backend Team access to backend/ files..."
# This should work since Backend Team is assigned and CODEOWNERS grants @rocketship-ai/backend-team access to backend/
# You'll need to modify the engine to accept repository URL and file path for testing

# Test 2: Backend team should NOT have access to frontend/ files  
echo "Testing Backend Team access to frontend/ files (should be denied)..."

# Test 3: Backend team should have access to *.go files
echo "Testing Backend Team access to Go files..."

# Test 4: QA team should have access to files without CODEOWNERS restrictions
echo "Testing QA Team access to unrestricted files..."

# Test 5: Organization Admin should have access to everything
echo "Testing Organization Admin access (should always work)..."
```

## Phase 5: Database Verification

### 5.1 Check Database Records
```bash
# Connect to the database to verify records
./.docker/rocketship logs postgres

# Or use a database client to connect and run these queries:
```

```sql
-- Check users table
SELECT id, email, name, is_admin, created_at FROM users;

-- Check teams table  
SELECT id, name, created_at FROM teams;

-- Check team_members table
SELECT tm.*, t.name as team_name, u.email as user_email 
FROM team_members tm 
JOIN teams t ON tm.team_id = t.id 
JOIN users u ON tm.user_id = u.id;

-- Check repositories table
SELECT id, url, enforce_codeowners, 
       CASE WHEN codeowners_cache IS NOT NULL THEN 'Cached' ELSE 'No Cache' END as cache_status,
       codeowners_cached_at, created_at 
FROM repositories;

-- Check team_repositories table
SELECT tr.*, t.name as team_name, r.url as repo_url
FROM team_repositories tr
JOIN teams t ON tr.team_id = t.id  
JOIN repositories r ON tr.repository_id = r.id;
```

## Phase 6: API Integration Testing

### 6.1 Test gRPC Authentication
```bash
# Engine should already be running from docker-compose
# Check if it's running
docker-compose -f .docker/docker-compose.yaml ps engine

# Try to run tests - should work with authentication
rocketship run -f /tmp/test-permissions/backend-test.yaml

# Check logs for authentication flow
docker-compose -f .docker/docker-compose.yaml logs engine
```

### 6.2 Test Unauthorized Access
```bash
# Logout first
rocketship auth logout

# Try to run commands without authentication (should fail)
rocketship team list
rocketship repo list

# Login again
rocketship auth login
```

## Phase 7: CODEOWNERS Cache Testing

### 7.1 Verify CODEOWNERS Fetching
The system should automatically fetch and cache CODEOWNERS when a repository is added. Check:

```bash
# Check repository details - should show cached CODEOWNERS
rocketship repo show "https://github.com/rocketship-ai/test-codeowners-repo"

# Look for cache status and last updated timestamp
```

### 7.2 Test CODEOWNERS Rules
Based on the test repository CODEOWNERS:
```
# Global ownership
* @rocketship-ai/qa-team

# Backend team owns backend directory
backend/ @rocketship-ai/backend-team

# Frontend team owns frontend directory  
frontend/ @rocketship-ai/frontend-team

# Backend team owns all Go files
*.go @rocketship-ai/backend-team
```

Expected permissions:
- **QA Team**: Access to all files (global ownership)
- **Backend Team**: Access to `backend/`, `*.go` files
- **Frontend Team**: Access to `frontend/` files
- **Organization Admin**: Access to everything

## Expected Results Summary

### ✅ What Should Work:
1. **Authentication**: Login/logout flow with Auth0
2. **Admin Access**: Admin users can perform all operations
3. **Team Management**: Create teams, add/remove members
4. **Repository Management**: Add repositories, assign teams
5. **CODEOWNERS Enforcement**: Path-based permissions working correctly
6. **Database Persistence**: All data properly stored and retrieved
7. **gRPC Authentication**: Engine enforces authentication

### ❌ What Should Fail:
1. **Unauthenticated Access**: Commands fail without login
2. **Unauthorized Paths**: Users can't access paths not owned by their teams
3. **Non-existent Resources**: Proper error messages for missing teams/repos
4. **Invalid Data**: Validation errors for malformed inputs

## Troubleshooting

### Common Issues:
1. **OAuth Errors**: Check OIDC environment variables
2. **Database Connection**: Verify PostgreSQL is running in Docker
3. **Team Assignment**: Ensure teams are assigned to repositories
4. **CODEOWNERS Cache**: Verify GitHub API access and cache population

### Debug Commands:
```bash
# Check Docker services (Option A - Multi-Stack)
./.docker/rocketship status

# Check engine logs (Option A - Multi-Stack)
./.docker/rocketship logs engine

# Check database logs (Option A - Multi-Stack)
./.docker/rocketship logs postgres

# Check authentication details
./.docker/rocketship auth status

# Alternative: Raw docker-compose commands (Option B - Self-Hosted)
docker-compose -f .docker/docker-compose.yaml ps
docker-compose -f .docker/docker-compose.yaml logs engine
docker-compose -f .docker/docker-compose.yaml logs postgres
```

## Performance Testing

### Load Testing:
```bash
# Test with multiple concurrent operations
for i in {1..10}; do
  rocketship team list &
done
wait

# Test permissions resolution performance
time rocketship repo show "https://github.com/rocketship-ai/test-codeowners-repo"
```

This comprehensive verification ensures all RBAC and CODEOWNERS functionality is working correctly before proceeding to implement additional features.