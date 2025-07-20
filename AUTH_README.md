# Rocketship Authentication & RBAC System

This document describes the authentication and Role-Based Access Control (RBAC) system implemented for Rocketship.

## Overview

The authentication system provides:
- **PKCE OAuth2 flow** for secure CLI and web UI authentication
- **OIDC-agnostic design** - works with Auth0, Okta, Azure AD, Google Workspace, etc.
- **Buildkite-inspired RBAC model** with granular permissions
- **Enterprise-ready** self-hosting capabilities
- **Admin API** for team and permission management
- **Backward compatibility** - unauthenticated usage still works

## Architecture

### Components

1. **Auth Package** (`internal/auth/`): Core OIDC authentication with PKCE flow
2. **RBAC Package** (`internal/rbac/`): Permission enforcement and data models
3. **GitHub Package** (`internal/github/`): GitHub API integration
4. **Tokens Package** (`internal/tokens/`): API token management
5. **Database Package** (`internal/database/`): PostgreSQL connection and migrations

### Database Schema

```sql
-- Users (from OIDC providers)
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,           -- OIDC subject
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    is_admin BOOLEAN DEFAULT false,        -- Organization-level admin
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_login TIMESTAMPTZ
);

-- Teams (Buildkite-inspired)
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Team Members with Roles & Permissions
CREATE TABLE team_members (
    team_id UUID REFERENCES teams(id),
    user_id VARCHAR(255) REFERENCES users(id),
    role VARCHAR(50) CHECK (role IN ('admin', 'member')),
    permissions TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);
```

## RBAC Model (Buildkite-Inspired)

### Roles

1. **Organization Admin**
   - Global admin access to entire system
   - Automatically assigned based on `ROCKETSHIP_ADMIN_EMAILS`
   - Can manage teams, users, and system settings

2. **Team Admin** 
   - Manage team members and their permissions
   - Full access to team resources
   - Can add/remove team members

3. **Team Member**
   - Configurable permissions per team
   - Granular access control based on assigned permissions

### Permissions (Buildkite-Style)

#### Test & Workflow Permissions
- `tests:read` - View test results
- `tests:write` - Run tests  
- `tests:manage` - Create/edit test suites
- `workflows:read` - View workflows
- `workflows:write` - Run workflows
- `workflows:manage` - Create/edit workflows

#### Repository Permissions
- `repositories:read` - View repository settings
- `repositories:write` - Modify repository settings
- `repositories:manage` - Add/remove repositories

#### Team Management (for Team Admins)
- `team:members:read` - View team members
- `team:members:write` - Add/remove team members
- `team:members:manage` - Manage member permissions

## Authentication Flow

### 1. OIDC Provider Configuration

The system works with any OIDC provider. Configure via environment variables:

```bash
# Auth0 Example
ROCKETSHIP_OIDC_ISSUER=https://your-tenant.auth0.com/
ROCKETSHIP_OIDC_CLIENT_ID=your-auth0-client-id

# Okta Example (Enterprise)
ROCKETSHIP_OIDC_ISSUER=https://your-company.okta.com/oauth2/default
ROCKETSHIP_OIDC_CLIENT_ID=your-okta-client-id

# Azure AD Example
ROCKETSHIP_OIDC_ISSUER=https://login.microsoftonline.com/tenant-id/v2.0
ROCKETSHIP_OIDC_CLIENT_ID=your-azure-client-id

# Admin Setup (required for initial setup)
ROCKETSHIP_ADMIN_EMAILS=admin@company.com,devops@company.com
```

### 2. PKCE Authorization Flow

```bash
# CLI Authentication
rocketship auth login
# → Opens browser to OIDC provider
# → User authenticates with their identity provider
# → PKCE code exchange happens securely
# → Tokens stored in system keyring

# Check status
rocketship auth status
# User: John Doe (john@company.com)
# Admin role: Yes

# Logout
rocketship auth logout
```

### 3. Initial Admin Setup

- Users listed in `ROCKETSHIP_ADMIN_EMAILS` automatically become Organization Admins
- On first login, user is created in database with admin privileges
- Admin can then create teams and manage other users

## Admin API Usage

### Team Management

```bash
# Create teams
rocketship team create backend-team
rocketship team create frontend-team

# Add team members with permissions
rocketship team add-member backend-team john@company.com member \
  --permissions "tests:read,tests:write,workflows:read,workflows:write"

# Make someone a team admin
rocketship team add-member frontend-team jane@company.com admin \
  --permissions "tests:*,workflows:*,team:members:*"

# List teams
rocketship team list
```

### Permission Examples

```bash
# Basic developer permissions
--permissions "tests:read,tests:write,workflows:read"

# Senior developer permissions  
--permissions "tests:*,workflows:*,repositories:read"

# Team admin permissions
--permissions "tests:*,workflows:*,repositories:*,team:members:*"

# Using wildcards for convenience
--permissions "tests:*"  # All test permissions
```

## Enterprise Deployment

### 1. Deploy Rocketship

```bash
# Deploy using Docker Compose
docker-compose up -d

# Or use your preferred container orchestration
```

### 2. Configure Identity Provider

Configure your existing enterprise IdP (Okta, Azure AD, etc.) with:
- **Application Type**: Single Page Application (for PKCE)
- **Redirect URI**: `http://localhost:8000/callback` (for CLI)
- **Grant Types**: Authorization Code + PKCE
- **Scopes**: `openid profile email`

### 3. Set Environment Variables

```bash
export ROCKETSHIP_OIDC_ISSUER="https://your-company.okta.com/oauth2/default"
export ROCKETSHIP_OIDC_CLIENT_ID="your-okta-client-id"
export ROCKETSHIP_ADMIN_EMAILS="admin@company.com,devops@company.com"
```

### 4. Initial Setup

```bash
# First admin logs in
rocketship auth login

# System automatically grants admin privileges
# Admin sets up teams and permissions
rocketship team create engineering
rocketship team add-member engineering dev@company.com member \
  --permissions "tests:read,tests:write"
```

## Security Features

### PKCE (Proof Key for Code Exchange)
- Prevents authorization code interception attacks
- No client secrets required for CLI applications
- Industry standard for secure OAuth2 flows

### Token Management
- Access tokens stored securely in system keyring
- Automatic token refresh when possible
- Secure logout with provider notification

### Permission Enforcement
- gRPC interceptors validate all requests
- Database-backed permission checks
- Granular resource-level access control

### Admin Email Security
- Case-insensitive email matching
- Whitespace trimming for robustness
- Environment-based configuration (no hardcoded admins)

## Backward Compatibility

The system maintains full backward compatibility:

- **Unauthenticated usage**: Still works for users who don't need auth
- **Auto-detection**: System automatically detects if auth is configured
- **Graceful degradation**: Falls back to permissionless mode when auth is disabled

```bash
# Works without authentication
rocketship run -f test.yaml

# Also works with authentication (when configured)
rocketship auth login
rocketship run -f test.yaml
```

## Integration Examples

### CI/CD Pipeline

```bash
# Generate API token for CI
rocketship token create ci-pipeline \
  --team backend-team \
  --permissions "tests:write,workflows:write" \
  --expires-in 90d

# Use in CI (token has team's repository access)
export ROCKETSHIP_API_TOKEN="rs_..."
rocketship run -f .github/workflows/test.yaml
```

### GitHub Integration

```bash
# Add repository to team
rocketship team add-repo backend-team https://github.com/company/api

# Enable CODEOWNERS enforcement
rocketship repo configure https://github.com/company/api \
  --enforce-codeowners=true
```

## Monitoring & Observability

### Authentication Events
- User logins/logouts logged
- Permission violations tracked
- Token usage monitored

### Admin Actions
- Team creation/modification logged
- Permission changes tracked
- User role changes audited

## Troubleshooting

### Common Issues

1. **"Authentication not configured"**
   - Ensure `ROCKETSHIP_OIDC_ISSUER` and `ROCKETSHIP_OIDC_CLIENT_ID` are set
   - Verify OIDC provider is accessible

2. **"Permission denied"**
   - Check user's team memberships: `rocketship auth status`
   - Verify required permissions are assigned to user's teams

3. **"Invalid token"**
   - Token may be expired: `rocketship auth login`
   - Check OIDC provider is responding correctly

### Debug Mode

```bash
# Enable debug logging
export ROCKETSHIP_LOG=DEBUG
rocketship auth login
```

## Comparison with Other Systems

### vs. Buildkite
- **Similar**: Team-based permissions, granular controls
- **Different**: OIDC-agnostic (not tied to specific providers)

### vs. GitHub
- **Similar**: Repository-based access control
- **Different**: Test execution focus, not code hosting

### vs. Traditional RBAC
- **Similar**: Role and permission concepts
- **Different**: Resource-specific permissions, team-centric model

---

This authentication system provides enterprise-grade security while maintaining the simplicity and flexibility that makes Rocketship easy to adopt in any organization.