# Enterprise Self-Hosting Guide

This guide provides a comprehensive walkthrough for enterprise administrators to self-host Rocketship with complete authentication and role-based access control.

## Overview

Rocketship is an open-source testing framework that enterprises can self-host to run automated tests with proper authentication, team management, and access controls. This guide covers the complete setup process from infrastructure to user management.

## Architecture

When self-hosted, Rocketship consists of these components:

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│   Users/Teams   │────▶│  Rocketship  │────▶│  Test APIs  │
│  (OIDC Login)   │     │   Platform   │     │  & Browser  │
└─────────────────┘     └──────────────┘     └─────────────┘
                               │
┌─────────────────┐            │              ┌─────────────┐
│    CI/CD Bot    │────────────┘         ┌───▶│   Reports   │
│  (API Tokens)   │                      │    └─────────────┘
└─────────────────┘     ┌──────────────┐ │
                        │   Temporal   │─┘
                        │  (Workflow)  │
                        └──────────────┘
```

**Core Components:**
- **Engine**: gRPC API server that orchestrates test execution
- **Worker**: Executes tests using various plugins (HTTP, Browser, SQL, etc.)
- **Temporal**: Workflow orchestration and durability
- **PostgreSQL**: Stores authentication data and Temporal state
- **External OIDC**: Your existing identity provider (Auth0, Okta, Azure AD, etc.)

## Prerequisites

### Docker Deployment
- Docker and Docker Compose
- DNS configuration capability
- SSL certificates for production
- Existing OIDC provider (Auth0, Okta, Azure AD, Google Workspace, etc.)
- PostgreSQL database (optional - included in stack)

### Kubernetes Deployment (Recommended for Production)
- Kubernetes cluster (1.19+)
- Helm 3.8+
- kubectl configured
- NGINX Ingress Controller
- cert-manager (optional, for automatic TLS)
- Persistent Volume support

## Step-by-Step Setup

Choose your deployment method based on your infrastructure:

### Option A: Kubernetes Deployment (Production Recommended)

#### 1. Infrastructure Preparation

Clone the repository and prepare Helm chart:

```bash
git clone https://github.com/rocketship-ai/rocketship.git
cd rocketship/helm/rocketship

# Add required Helm repositories
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

# Install dependencies
helm dependency update
```

**Purpose**: Enterprise-grade Kubernetes deployment with high availability
**Result**: Scalable, production-ready infrastructure

### Option B: Docker Deployment

#### 1. Infrastructure Preparation

Clone the repository and navigate to the Docker configuration:

```bash
git clone https://github.com/rocketship-ai/rocketship.git
cd rocketship/.docker
```

**Purpose**: Get the complete Rocketship platform with authentication support
**Result**: Local copy with Docker multi-stack environment

### 2. OIDC Provider Setup

#### Step 2.1: Create Rocketship Application in Your Identity Provider

**For Okta (Enterprise):**

1. **Access Okta Admin Console**
   - Sign in to your Okta organization with administrator account
   - Navigate to **Applications > Applications**

2. **Create New App Integration**
   - Click **Create App Integration**
   - Select **OIDC - OpenID Connect**
   - Select **Web Application** (for enhanced security)

3. **Configure Application Settings**
   ```
   App name: Rocketship Testing Platform
   Grant type: Authorization Code
   Sign-in redirect URIs:
     - https://rocketship.your-enterprise.com/callback
     - http://localhost:8000/callback (for CLI development)
   Sign-out redirect URIs:
     - https://rocketship.your-enterprise.com/logout
     - http://localhost:8000/logout
   ```

4. **Configure Group Claims** (Important for RBAC)
   - Go to **Sign On** tab → **OpenID Connect ID Token**
   - Set **Groups claim type**: Filter
   - Set **Groups claim filter**: groups Matches regex `.*`

5. **Assign Users/Groups**
   - Go to **Assignments** tab
   - Assign relevant groups or users who need Rocketship access

6. **Copy Credentials**
   - Note the **Client ID** and **Client Secret** from the General tab

**For Azure AD:**

1. **Access Azure Portal**
   - Go to **Azure Active Directory > App registrations**

2. **Create New Registration**
   - Click **New registration**
   - Name: `Rocketship Testing Platform`
   - Redirect URI: `Web` → `https://rocketship.your-enterprise.com/callback`

3. **Configure Authentication**
   - Go to **Authentication** section
   - Add additional redirect URIs:
     - `http://localhost:8000/callback` (for CLI)
   - Under **Implicit grant and hybrid flows**: Check **ID tokens**

4. **Create Client Secret**
   - Go to **Certificates & secrets** → **Client secrets**
   - Click **New client secret**
   - Copy the **Value** (not the Secret ID)

5. **API Permissions**
   - Go to **API permissions**
   - Ensure `openid`, `profile`, `email` permissions are granted

**For Auth0:**

1. **Access Auth0 Dashboard**
   - Go to **Applications** in Auth0 Dashboard

2. **Create Application**
   - Click **Create Application**
   - Name: `Rocketship Testing Platform`
   - Type: **Single Page Applications** (for development/testing)

3. **Configure Settings**
   - **Allowed Callback URLs**: `http://localhost:8000/callback`
   - **Allowed Logout URLs**: `http://localhost:8000/logout`
   - **Allowed Web Origins**: `http://localhost:8000`

4. **Copy Credentials**
   - Note the **Client ID** (no client secret needed for SPA)

#### Step 2.2: Configure Rocketship Environment

**For Kubernetes Deployment:**

Create Kubernetes secrets for OIDC configuration:

```bash
# Create namespace
kubectl create namespace rocketship

# Create OIDC secret
kubectl create secret generic rocketship-oidc-secret \
  --from-literal=issuer="https://your-enterprise.okta.com" \
  --from-literal=client-id="0oa1234567890abcdef" \
  --from-literal=client-secret="your-client-secret-from-okta" \
  --namespace rocketship

# Create custom values file
cat > values-production.yaml <<EOF
auth:
  oidc:
    existingSecret: "rocketship-oidc-secret"
  adminEmails: "admin@your-enterprise.com,devops@your-enterprise.com"

tls:
  enabled: true
  domain: "rocketship.your-enterprise.com"
  certificate:
    existingSecret: "rocketship-tls-secret"  # Create this with your certificates

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: rocketship.your-enterprise.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: rocketship-tls-secret
      hosts:
        - rocketship.your-enterprise.com
EOF
```

**For Docker Deployment:**

Set the OIDC configuration based on your provider:

```bash
# For Okta (Production):
export ROCKETSHIP_OIDC_ISSUER="https://your-enterprise.okta.com"
export ROCKETSHIP_OIDC_CLIENT_ID="0oa1234567890abcdef"
export ROCKETSHIP_OIDC_CLIENT_SECRET="your-client-secret-from-okta"
export ROCKETSHIP_ADMIN_EMAILS="admin@your-enterprise.com,devops@your-enterprise.com"

# For Azure AD (Production):
export ROCKETSHIP_OIDC_ISSUER="https://login.microsoftonline.com/your-tenant-id/v2.0"
export ROCKETSHIP_OIDC_CLIENT_ID="your-azure-client-id"
export ROCKETSHIP_OIDC_CLIENT_SECRET="your-azure-client-secret"
export ROCKETSHIP_ADMIN_EMAILS="admin@your-enterprise.com"

# For Auth0 (Development/Testing):
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-auth0-client-id"
export ROCKETSHIP_OIDC_CLIENT_SECRET=""  # Empty for SPA mode
export ROCKETSHIP_ADMIN_EMAILS="admin@your-enterprise.com"
```

**Security Notes:**
- **Production**: Use Web Application type with client secret for enhanced security
- **Development**: SPA type with PKCE is acceptable for testing
- **Client Secret**: Store securely (Kubernetes secrets, environment files, etc.)

**Purpose**: Integrate with enterprise identity management
**Result**: Single sign-on for all enterprise users with proper security

### 3. Database Configuration

#### Option A: Use Included PostgreSQL (Default)

```bash
# Automatic configuration for auth database:
ROCKETSHIP_DB_HOST=auth-postgres
ROCKETSHIP_DB_PORT=5432
ROCKETSHIP_DB_NAME=auth
ROCKETSHIP_DB_USER=authuser
ROCKETSHIP_DB_PASSWORD=authpass
```

#### Option B: Enterprise Database (Production)

```bash
# Configure for production database:
export ROCKETSHIP_DB_HOST=postgres.company.com
export ROCKETSHIP_DB_PORT=5432
export ROCKETSHIP_DB_NAME=rocketship_auth
export ROCKETSHIP_DB_USER=rocketship_user
export ROCKETSHIP_DB_PASSWORD=secure-enterprise-password
```

**Purpose**: Store user permissions, teams, repositories, and API tokens
**Result**: Persistent authentication data in enterprise infrastructure

### 4. Network and DNS Setup

Configure external access for production:

```bash
# Production domains:
# rocketship.company.com → Engine (port 7700)
# temporal.company.com → Temporal UI (port 8080)  

# Load balancer configuration:
upstream rocketship_engine {
    server rocketship-engine:7700;
}

upstream temporal_ui {
    server temporal-ui:8080;
}
```

**Purpose**: Secure, accessible endpoints for enterprise users
**Result**: Production-ready HTTPS access

### 5. Initialize the Platform

**For Kubernetes Deployment:**

Deploy using Helm with enterprise configuration:

```bash
# Create TLS secret with your certificates
kubectl create secret tls rocketship-tls-secret \
  --cert=path/to/your/certificate.crt \
  --key=path/to/your/private.key \
  --namespace rocketship

# Deploy Rocketship with production values
helm install rocketship ./helm/rocketship \
  -f values-production.yaml \
  --namespace rocketship

# Verify deployment
kubectl get pods -n rocketship
kubectl get services -n rocketship
kubectl get ingress -n rocketship

# Check application logs
kubectl logs -n rocketship deployment/rocketship-engine
kubectl logs -n rocketship deployment/rocketship-worker
```

**For Docker Deployment:**

Start the complete authentication-enabled stack:

```bash
# Initialize environment for your deployment
./rocketship init

# Start all services:
./rocketship start

# Verify all services are healthy:
./rocketship status
```

**Services started:**
- Temporal (workflow engine)
- PostgreSQL (Temporal database)
- Elasticsearch (Temporal visibility)
- Engine (Rocketship API)
- Worker (test execution)
- Auth PostgreSQL (authentication data)
- Test databases (PostgreSQL, MySQL)

**Purpose**: Deploy the complete platform
**Result**: Full Rocketship environment with authentication

### 6. First Admin Setup

Authenticate and verify admin access:

```bash
# Ensure environment variables are set from step 2
echo $ROCKETSHIP_OIDC_ISSUER
echo $ROCKETSHIP_ADMIN_EMAILS

# Login as admin (opens browser for OIDC flow):
rocketship auth login

# Verify admin status:
rocketship auth status
```

**Expected Output:**
```
Status: Authenticated
User: John Doe (john@company.com)
Subject: user_id_from_oidc_provider
Admin role: Yes
```

**Purpose**: Bootstrap first authenticated administrator
**Result**: Admin user with full platform access

### 7. Team and Permission Management

Create organizational structure using Buildkite-inspired permissions:

```bash
# Create teams for different groups:
rocketship team create "QA Engineering"
rocketship team create "Backend Team"  
rocketship team create "Frontend Team"
rocketship team create "DevOps Team"

# Add team members with specific permissions:
rocketship team add-member "QA Engineering" john.doe@company.com \
  --role admin \
  --permissions "tests:read,tests:write,tests:manage,workflows:read,workflows:write,team:members:manage"

rocketship team add-member "Backend Team" sarah.smith@company.com \
  --role member \
  --permissions "tests:read,tests:write,workflows:read,workflows:write"

# Add a senior developer with repository management:
rocketship team add-member "Backend Team" lead.dev@company.com \
  --role admin \
  --permissions "tests:*,workflows:*,repositories:read,repositories:write,team:members:read"
```

**Available Permissions:**
- **Test Permissions**: `tests:read`, `tests:write`, `tests:manage`
- **Workflow Permissions**: `workflows:read`, `workflows:write`, `workflows:manage`
- **Repository Permissions**: `repositories:read`, `repositories:write`, `repositories:manage`
- **Team Management**: `team:members:read`, `team:members:write`, `team:members:manage`

**Purpose**: Organize users by responsibility with granular access control
**Result**: Role-based access control aligned with team structure

### 8. CI/CD Integration

Configure automated test execution:

```yaml
# GitHub Actions example (.github/workflows/tests.yml):
name: Automated Testing
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Run Rocketship Tests
        env:
          ROCKETSHIP_API_TOKEN: ${{ secrets.ROCKETSHIP_TOKEN }}
        run: |
          # Install Rocketship CLI
          curl -sSL https://get.rocketship.sh | bash
          
          # Run tests against production engine
          rocketship run -f tests/api-integration.yaml \
            --engine https://rocketship.company.com
```

**API Token Creation:**
```bash
# Generate API token for CI/CD:
rocketship token create "CI-Pipeline" \
  --team "Backend Team" \
  --permissions "tests:write,workflows:write" \
  --expires 90d
```

**Purpose**: Automate test execution in development workflows
**Result**: Continuous testing with proper authentication

### 9. Production Monitoring

Set up observability and scaling:

```bash
# View real-time workflow execution:
# https://temporal.company.com

# Monitor engine metrics:
curl http://rocketship.company.com:7701/metrics

# Scale workers for parallel execution:
docker-compose scale worker=10

# View test execution logs:
./rocketship logs worker
```

**Metrics Available:**
- Test execution rates
- Success/failure ratios  
- Plugin usage statistics
- Resource utilization

**Purpose**: Monitor platform health and performance
**Result**: Scalable, observable test execution platform

## Security Features

### Authentication & Authorization
- **PKCE OAuth2 flow** with enterprise OIDC providers
- **Buildkite-inspired RBAC** with granular permissions
- **Admin email-based setup** for initial administrators
- **API token management** with expiration and rotation

### Audit & Compliance
- **Complete audit trail** of all test executions
- **User attribution** for all actions
- **Token usage tracking** with last-used timestamps
- **Permission change logging**

### Data Protection
- **Encrypted token storage** using secure hashing (SHA-256)
- **Environment variable isolation** between teams
- **Network segmentation** with proper container networking
- **No hardcoded secrets** - environment-based configuration

## Enterprise Usage Patterns

### Development Teams
```bash
# Developer workflow:
rocketship auth login                           # OIDC authentication
rocketship validate tests/feature-tests.yaml   # Validate test syntax
rocketship run -f tests/feature-tests.yaml     # Execute tests locally
```

### QA Engineers  
```bash
# QA workflow with team permissions:
rocketship team list                            # View accessible teams
rocketship run -f tests/regression-suite.yaml  # Run comprehensive tests
rocketship list runs                           # View test history
```

### DevOps/Platform Teams
```bash
# Platform management:
rocketship team create "New Project Team"       # Onboard new teams
rocketship token create "Deployment-Bot"       # Create service tokens
```

## Troubleshooting

### Authentication Issues
```bash
# Check OIDC configuration:
curl -f $ROCKETSHIP_OIDC_ISSUER/.well-known/openid_configuration

# Verify user permissions:
rocketship auth status

# Debug authentication:
export ROCKETSHIP_LOG=DEBUG
rocketship auth login
```

### Platform Health
```bash
# Check all services:
./rocketship status

# View service logs:
./rocketship logs engine
./rocketship logs worker  

# Test engine connectivity:
curl -f http://localhost:7700/health
```

### Database Issues
```bash
# Check auth database:
docker exec -it rocketship-auth-postgres-1 \
  psql -U authuser -d auth -c "\dt"

# Verify schema:
docker exec -it rocketship-auth-postgres-1 \
  psql -U authuser -d auth -c "\d users"
```

## Scaling Considerations

### Horizontal Scaling

**Kubernetes (Recommended):**
```bash
# Scale workers for increased throughput
kubectl scale deployment rocketship-worker --replicas=10 -n rocketship

# Scale engines for high availability
kubectl scale deployment rocketship-engine --replicas=3 -n rocketship

# Enable horizontal pod autoscaling
kubectl autoscale deployment rocketship-worker \
  --cpu-percent=70 --min=3 --max=20 -n rocketship

# Configure resource requests and limits in values.yaml
helm upgrade rocketship ./helm/rocketship \
  --set rocketship.worker.replicas=5 \
  --set rocketship.engine.replicas=2 \
  --namespace rocketship
```

**Docker:**
- **Worker scaling**: Add more worker containers for parallel execution
- **Engine replicas**: Multiple engine instances behind load balancer
- **Database clustering**: PostgreSQL read replicas for high availability

### Resource Planning
- **Worker resources**: 100-500MB RAM per worker, 0.1-0.5 CPU cores
- **Engine resources**: 128-512MB RAM, 0.1-1.0 CPU cores
- **Storage**: 10-50GB for Temporal history, 1-5GB for auth data

### Performance Optimization
- **Test parallelization**: Configure appropriate worker counts
- **Resource limits**: Set container resource constraints
- **Network optimization**: Use high-speed networking between components

## Maintenance

### Regular Tasks
```bash
# Update Rocketship to latest version:
git pull origin main
./rocketship stop
docker-compose pull
./rocketship start

# Rotate API tokens:
rocketship token list
rocketship token create "new-token" --team "team-name"
```

### Backup Procedures
```bash
# Backup authentication database:
docker exec rocketship-auth-postgres-1 \
  pg_dump -U authuser auth > auth-backup.sql

# Backup Temporal database:  
docker exec rocketship-postgresql-1 \
  pg_dump -U temporal temporal > temporal-backup.sql
```

## Migration from Existing Systems

### From Manual Testing
1. **Inventory existing test cases** and convert to YAML format
2. **Set up teams** matching current organizational structure  
3. **Migrate test data** and environment configurations
4. **Train teams** on Rocketship workflow and CLI usage

### From Other Testing Platforms
1. **Export test definitions** from existing tools
2. **Convert to Rocketship YAML format** using available plugins
3. **Migrate user access patterns** to Rocketship teams
4. **Parallel running** during transition period

## Next Steps

After successful deployment:

1. **Team Onboarding**: Train teams on Rocketship workflows
2. **Test Migration**: Convert existing tests to Rocketship format  
3. **CI/CD Integration**: Integrate with existing development workflows
4. **Monitoring Setup**: Configure alerting and dashboards
5. **Documentation**: Create team-specific testing guidelines

## Related Documentation

- [Authentication & RBAC Guide](../../AUTH_README.md) - Complete authentication documentation
- [Command Reference](./reference/rocketship.md) - Complete CLI documentation
- [Examples](./examples.md) - Test specification examples
- [YAML Reference](./yaml-reference/plugin-reference.md) - Complete syntax reference

This enterprise setup provides a complete, secure, and scalable testing platform that integrates seamlessly with existing enterprise infrastructure and workflows.