# Rocketship Multi-Stack Docker Environment

This directory provides a **completely automated multi-stack Docker environment** for Rocketship development. It's designed specifically for **git worktree workflows** where each worktree gets its own isolated Docker environment with **zero configuration required**.

> **🔐 Authentication Support**: This environment now includes full authentication capabilities using external OIDC providers (Auth0, Okta, Azure AD, etc.). See the [Authentication Setup](#authentication-setup) section below.

## 🎯 Key Features

- **🔍 Auto-Discovery**: Automatically detects your current git worktree/branch
- **🔢 Dynamic Ports**: Calculates unique ports to prevent conflicts between environments  
- **🚀 Zero Config**: No manual environment setup required
- **🌐 Complete Isolation**: Each worktree gets separate networks, volumes, and containers
- **⚡ Simple Commands**: Single CLI that handles everything automatically

## 🚀 Quick Start for Git Worktrees

### Step 1: Create a Git Worktree

```bash
# From main rocketship directory
git worktree add ../rocketship-feature-xyz

# Navigate to your worktree
cd ../rocketship-feature-xyz
```

### Step 2: Initialize Isolated Environment

```bash
# Auto-initialize environment for this worktree
./.docker/rocketship init
```

This command will:
- Auto-detect your branch/worktree name
- Calculate unique ports (no conflicts!)
- Generate environment configuration
- Set up isolated Docker stack

### Step 3: Start and Use Your Environment

```bash
# Start your isolated stack
./.docker/rocketship start

# Use rocketship CLI directly with profiles
rocketship profile list                    # See available profiles
rocketship run -f test.yaml                # Run tests
rocketship team list                       # Manage teams

# Stop when done
./.docker/rocketship stop
```

## 🏗️ How It Works

### Auto-Discovery Magic

The system automatically:

1. **Detects your current git branch** (e.g., `feature-xyz`)
2. **Creates a stack name** (e.g., `rocketship-feature-xyz`)
3. **Calculates unique ports** using hash-based allocation
4. **Generates environment** with zero conflicts

### Port Allocation Example

If you have multiple worktrees:

**Worktree 1** (`feature-api`):
- Stack Name: `rocketship-feature-api`
- Temporal UI: `http://localhost:8180`
- Engine API: `localhost:7800`

**Worktree 2** (`feature-ui`):
- Stack Name: `rocketship-feature-ui`  
- Temporal UI: `http://localhost:9280`
- Engine API: `localhost:8900`

**No conflicts!** Each gets its own port range automatically.

## 📋 Available Commands

### Essential Docker Commands
```bash
./.docker/rocketship init           # Initialize stack for current worktree
./.docker/rocketship start          # Start the current stack
./.docker/rocketship stop           # Stop the current stack
./.docker/rocketship logs [service] # Show recent logs (never hangs)
./.docker/rocketship clean          # Stop and remove all containers and volumes
```

### Use Rocketship CLI Directly
After starting your stack, use the rocketship CLI with profiles:
```bash
rocketship profile list             # See available profiles
rocketship auth login               # Authenticate if needed
rocketship run -f test.yaml         # Run tests
rocketship team list                # Manage teams
rocketship validate test.yaml       # Validate test files
rocketship list                     # List test runs
rocketship get <run-id>             # Get test run details
```

### Removed Commands (Use Direct CLI)
These commands have been removed to simplify the Docker script:
```bash
# Old → New
restart  → ./.docker/rocketship stop && ./.docker/rocketship start
status   → docker ps | grep rocketship
info     → rocketship profile list
validate → rocketship validate <file>
run      → rocketship run [options]
list     → rocketship list
get      → rocketship get <run-id>
```

## 🎯 Git Worktree Workflow Examples

### Scenario 1: Single Developer, Multiple Features

```bash
# Main development
cd rocketship
./.docker/rocketship init && ./.docker/rocketship start

# Work on API feature in parallel
git worktree add ../rocketship-api-enhancement
cd ../rocketship-api-enhancement
./.docker/rocketship init && ./.docker/rocketship start

# Both environments running simultaneously with different ports!
```

### Scenario 2: Team Development

```bash
# Developer A - Feature Branch
git worktree add ../rocketship-user-auth
cd ../rocketship-user-auth
./.docker/rocketship init
./.docker/rocketship start
# Gets ports: 8xxx range

# Developer B - Different Feature  
git worktree add ../rocketship-payment-flow
cd ../rocketship-payment-flow
./.docker/rocketship init
./.docker/rocketship start
# Gets ports: 9xxx range (automatically different!)
```

### Scenario 3: Claude Code Agents

Each Claude Code agent working in a different worktree gets completely isolated environments:

```bash
# Agent 1 Prompt:
# "You are working in worktree ../rocketship-feature-x"
# "Run: ./.docker/rocketship init && ./.docker/rocketship start"

# Agent 2 Prompt:  
# "You are working in worktree ../rocketship-feature-y"
# "Run: ./.docker/rocketship init && ./.docker/rocketship start"

# Zero conflicts, complete isolation!
```

## 🔧 Technical Details

### Stack Naming Convention

- **Git Branch**: `feature/user-auth` → **Stack Name**: `rocketship-feature-user-auth`
- **Worktree Dir**: `rocketship-api-v2` → **Stack Name**: `rocketship-rocketship-api-v2`
- **Special chars** are automatically converted to hyphens for Docker compatibility

### Port Calculation Algorithm

```bash
# Simplified algorithm:
hash = checksum(stack_name)
offset = (hash % 50) * 100
temporal_port = 7233 + offset
engine_port = 7700 + offset
# ... etc for all services
```

This ensures:
- **50 possible port ranges** (0-4900 offset)
- **100 ports per range** (enough for all services)
- **Deterministic allocation** (same branch = same ports)
- **Zero conflicts** between different stacks

### File Structure

```
.docker/
├── rocketship                       # Main CLI (auto-detects everything)
├── init-stack.sh                    # Stack initialization script
├── stack-utils.sh                   # Shared utilities and logic
├── docker-compose.yaml              # Parameterized compose file
├── Dockerfile.cli                   # CLI container image
├── Dockerfile.engine                # Engine container image
├── Dockerfile.worker                # Worker container image
├── templates/
│   └── .env.template               # Environment file template
└── .env.{stack-name}              # Generated environment files
```

## 🐛 Troubleshooting

### "Stack not initialized"
```bash
# Solution: Initialize first
./.docker/rocketship init
```

### "Stack not running"
```bash
# Solution: Start the stack
./.docker/rocketship start
```

### "Port already in use"
This should never happen with the auto-allocation system, but if it does:
```bash
# Check what's using the port
lsof -i :PORT_NUMBER

# Or force clean and restart
./.docker/rocketship clean
./.docker/rocketship start
```

### "Docker not running"
```bash
# Start Docker Desktop or Docker daemon
# Then retry your command
```

## 🧹 Cleanup

### Clean Single Environment
```bash
# Stop and remove containers/volumes for current worktree
./.docker/rocketship clean
```

### Clean All Environments
```bash
# Stop all rocketship containers
docker stop $(docker ps -q --filter "name=rocketship-")

# Remove all rocketship containers and volumes
docker system prune -f
docker volume prune -f
```

## 🎉 Benefits for Development

### For Individual Developers
- **Parallel Development**: Work on multiple features simultaneously
- **Clean Separation**: No interference between different features
- **Quick Switching**: Each worktree maintains its own state

### For Teams
- **Zero Conflicts**: Everyone gets unique ports automatically
- **Easy Onboarding**: New team members just run `init` and `start`
- **Consistent Environments**: Same setup across all machines

### For Claude Code Agents
- **Perfect Isolation**: Each agent has its own complete environment
- **Auto-Configuration**: Agents can set up environments autonomously
- **Conflict-Free**: Multiple agents can work simultaneously

## 🔗 Integration with Main Workflow

This multi-stack system integrates seamlessly with the standard Rocketship development workflow:

1. **Create worktree** for your feature/fix
2. **Initialize environment** with `./.docker/rocketship init`
3. **Develop and test** using standard rocketship commands
4. **Commit and push** your changes
5. **Clean up** with `./.docker/rocketship clean` when done

The isolated environment ensures your development doesn't interfere with other work and provides a clean, reproducible testing environment for every feature.

## 🔐 Authentication Setup

### For Testing with External OIDC Providers

If you want to test authentication features, configure environment variables before starting your stack:

```bash
# Example: Auth0 Configuration
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-auth0-client-id"
export ROCKETSHIP_ADMIN_EMAILS="your-email@gmail.com"

# Example: Enterprise Okta Configuration
export ROCKETSHIP_OIDC_ISSUER="https://your-company.okta.com/oauth2/default"
export ROCKETSHIP_OIDC_CLIENT_ID="your-okta-client-id"
export ROCKETSHIP_ADMIN_EMAILS="admin@company.com,devops@company.com"

# Then start your stack
./.docker/rocketship start

# Test authentication
rocketship auth login
rocketship auth status
```

### Authentication Features

When authentication is configured, your stack includes:
- **PostgreSQL auth database** for user and team management
- **PKCE OAuth2 flow** for secure CLI authentication
- **Buildkite-inspired RBAC** with teams and granular permissions
- **Admin API** for team management
- **Backward compatibility** - works without auth too

### Authentication Commands

```bash
# Authentication
rocketship auth login          # Login via OIDC provider
rocketship auth status         # Show current user
rocketship auth logout         # Logout

# Team Management (for admins)
rocketship team create my-team
rocketship team add-member my-team user@company.com member \
  --permissions "tests:read,tests:write,workflows:read"
rocketship team list
```

For complete authentication documentation, see [AUTH_README.md](../AUTH_README.md).

### Without Authentication

If you don't configure authentication environment variables, the system works exactly as before - no authentication required, full functionality available.

---

**Ready to get started?** Just `cd` to any rocketship worktree and run `./.docker/rocketship init`! 🚀