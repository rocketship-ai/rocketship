# Rocketship Authentication & RBAC System

This document describes the authentication and Role-Based Access Control (RBAC) system implemented for Rocketship.

## Overview

The authentication system provides:
- **PKCE OAuth2 flow** for both CLI and web UI
- **RBAC model** with organizations, teams, and permissions
- **GitHub integration** for repository management and CODEOWNERS
- **API token system** for CI/CD integration
- **Backward compatibility** - unauthenticated usage still works

## Architecture

### Components

1. **Auth Package** (`internal/auth/`): Core authentication with PKCE flow
2. **RBAC Package** (`internal/rbac/`): Permission enforcement and data models
3. **GitHub Package** (`internal/github/`): GitHub API integration
4. **Tokens Package** (`internal/tokens/`): API token management
5. **Database Package** (`internal/database/`): PostgreSQL connection and migrations
6. **Web UI** (`web/`): React-based web interface

### Database Schema

```sql
-- Users (from OIDC)
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,           -- OIDC subject
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    is_admin BOOLEAN DEFAULT false,        -- Organization-level admin
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_login TIMESTAMPTZ
);

-- Teams
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Team Members
CREATE TABLE team_members (
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'member')),
    permissions TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

-- Repositories
CREATE TABLE repositories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url VARCHAR(500) UNIQUE NOT NULL,
    github_installation_id BIGINT,
    enforce_codeowners BOOLEAN DEFAULT false,
    codeowners_cache JSONB,
    codeowners_cached_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Team Repository Access
CREATE TABLE team_repositories (
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    repository_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (team_id, repository_id)
);

-- API Tokens
CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    permissions TEXT[] NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by VARCHAR(255) REFERENCES users(id)
);
```

## Setup Instructions

### 1. Database Setup

```bash
# Install PostgreSQL 15+
# Create database and user
createdb rocketship
createuser -P rocketship

# Run migrations
make install
rocketship migrate up
```

### 2. OIDC Configuration

Set up your OIDC provider (e.g., Keycloak) and configure environment variables:

```bash
export ROCKETSHIP_OIDC_ISSUER="https://your-keycloak/realms/rocketship"
export ROCKETSHIP_OIDC_CLIENT_ID="rocketship"
export ROCKETSHIP_OIDC_CLIENT_SECRET="your-secret"
export ROCKETSHIP_OIDC_ADMIN_GROUP="rocketship-admins"
```

### 3. Database Configuration

```bash
export ROCKETSHIP_DB_HOST="localhost"
export ROCKETSHIP_DB_PORT="5432"
export ROCKETSHIP_DB_NAME="rocketship"
export ROCKETSHIP_DB_USER="rocketship"
export ROCKETSHIP_DB_PASSWORD="your-password"
export ROCKETSHIP_DB_SSLMODE="disable"
```

### 4. Web UI Configuration

```bash
# In web/.env
REACT_APP_OIDC_ISSUER="https://your-keycloak/realms/rocketship"
REACT_APP_OIDC_CLIENT_ID="rocketship"
```

## Usage

### CLI Authentication

```bash
# Login
rocketship auth login

# Check status
rocketship auth status

# Logout
rocketship auth logout
```

### Team Management

```bash
# Create team
rocketship team create "DevOps Team"

# Add member
rocketship team add-member "DevOps Team" user@example.com --role=admin --permissions=test_runs,repository_mgmt

# Add repository
rocketship team add-repo "DevOps Team" https://github.com/org/repo
```

### API Token Management

```bash
# Create token
rocketship token create "CI Token" --team="DevOps Team" --permissions=test_runs --expires=2024-12-31

# List tokens
rocketship token list

# Revoke token
rocketship token revoke <token-id>
```

### Web UI

1. Navigate to `http://localhost:3000`
2. Click "Sign in with OIDC"
3. Complete authentication flow
4. Access dashboard and management features

## Permission Model

### Roles

- **Organization Admin**: Full access to everything
- **Team Admin**: Can manage team members and repositories
- **Team Member**: Access based on assigned permissions

### Permissions

- `test_runs`: Can run tests
- `repository_mgmt`: Can manage repositories
- `team_mgmt`: Can manage team members
- `user_mgmt`: Can manage users (org admins only)

### CODEOWNERS Integration

When `enforce_codeowners` is enabled for a repository:

1. System fetches CODEOWNERS file from GitHub
2. Caches the parsed rules
3. Checks if user/team owns the path being tested
4. Grants/denies access based on ownership

## API Token Usage

### In CI/CD Pipelines

```bash
# Export token
export ROCKETSHIP_API_TOKEN="your-token-here"

# Use with CLI
rocketship run -f test.yaml
```

### Token Format

API tokens are:
- 64-character hex strings
- Hashed with SHA-256 for storage
- Scoped to specific team and permissions
- Optional expiration dates

## Security Features

### CLI Security

- **System keyring**: Tokens stored in OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- **PKCE flow**: Prevents authorization code interception
- **Token refresh**: Automatic token refresh

### Web Security

- **HttpOnly cookies**: Secure token storage
- **SameSite strict**: CSRF protection
- **Secure flag**: HTTPS-only cookies
- **PKCE flow**: Same as CLI

### API Security

- **Token hashing**: Tokens are hashed with SHA-256
- **Scoped permissions**: Tokens limited to specific permissions
- **Usage tracking**: Last used timestamps
- **Expiration**: Optional token expiration

## Backward Compatibility

The system maintains backward compatibility:

1. **Authentication detection**: Checks for OIDC configuration
2. **Graceful degradation**: Falls back to unauthenticated mode
3. **Optional enforcement**: RBAC only applies when configured
4. **CLI compatibility**: All existing commands work without auth

## Troubleshooting

### Common Issues

1. **"Authentication not configured"**
   - Check OIDC environment variables
   - Verify database connection

2. **"Token expired"**
   - Run `rocketship auth login` to refresh

3. **"Permission denied"**
   - Check team memberships and permissions
   - Verify repository access

4. **"CODEOWNERS not found"**
   - Ensure repository has CODEOWNERS file
   - Check GitHub integration

### Debug Commands

```bash
# Check auth status
rocketship auth status

# Test database connection
rocketship migrate version

# Debug with verbose logging
ROCKETSHIP_LOG=DEBUG rocketship run -f test.yaml
```

## Development

### Adding New Permissions

1. Add permission to `internal/rbac/types.go`
2. Update CLI commands in `internal/cli/`
3. Add to web UI permission lists
4. Update documentation

### Adding New Endpoints

1. Add gRPC method to `proto/engine.proto`
2. Implement in `internal/orchestrator/`
3. Add authentication check if needed
4. Update web UI if applicable

## Testing

### Unit Tests

```bash
go test ./internal/auth/...
go test ./internal/rbac/...
go test ./internal/tokens/...
```

### Integration Tests

```bash
# Start test database
docker-compose up -d postgres-test

# Run integration tests
go test -tags=integration ./...
```

### Web UI Tests

```bash
cd web
npm test
```

## Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
# Build steps...

FROM alpine:latest
# Runtime setup...
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rocketship
spec:
  # Deployment configuration...
  template:
    spec:
      containers:
      - name: rocketship
        env:
        - name: ROCKETSHIP_OIDC_ISSUER
          value: "https://your-keycloak/realms/rocketship"
        # Other environment variables...
```

## Monitoring

### Metrics

- Authentication success/failure rates
- Token usage statistics
- Permission check counts
- CODEOWNERS cache hit rates

### Logs

- Authentication events
- Permission violations
- Token creation/revocation
- RBAC enforcement decisions

## Future Enhancements

1. **Multi-organization support**: Multiple isolated organizations
2. **Advanced CODEOWNERS**: Regex patterns, inheritance rules
3. **Audit logs**: Comprehensive activity tracking
4. **SSO integration**: SAML, Azure AD, Google Workspace
5. **API rate limiting**: Token-based rate limiting
6. **Mobile app**: React Native mobile interface

## Contributing

1. Fork the repository
2. Create feature branch
3. Add tests for new functionality
4. Update documentation
5. Submit pull request

## License

This implementation is part of the Rocketship project and follows the same license terms.