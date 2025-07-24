# Docker Development Environment

The Rocketship Docker environment is designed for **developers, contributors, and AI agents** working on Rocketship itself. It provides completely isolated development environments using git worktrees with **zero configuration** and **dynamic port allocation**.

## Overview

This development environment enables:

- 🔍 **Auto-Discovery**: Automatically detects your git worktree/branch
- 🔢 **Dynamic Ports**: Calculates unique ports to prevent conflicts
- 🚀 **Zero Config**: No manual environment setup required  
- 🌐 **Complete Isolation**: Each worktree gets separate networks, volumes, containers
- 🤖 **AI Agent Ready**: Perfect for parallel AI agents working independently
- ⚡ **Simple Commands**: Single CLI that handles everything automatically

## Perfect For

### 👨‍💻 **Developers & Contributors**
- Working on multiple Rocketship features simultaneously
- Testing changes in isolation
- Contributing to the Rocketship codebase

### 🤖 **AI Coding Agents**
- Claude Code agents working on different features
- Parallel development without conflicts
- Autonomous environment setup and management

### 🔬 **Testing & Experimentation**
- Trying different approaches in isolation
- A/B testing features
- Rapid prototyping

## Quick Start for Git Worktrees

### Step 1: Create a Git Worktree

```bash
# From main rocketship directory
git worktree add ../rocketship-feature-xyz

# Navigate to your worktree
cd ../rocketship-feature-xyz
```

### Step 2: Initialize Your Isolated Environment

```bash
# Auto-initialize environment for this worktree (zero config!)
./.docker/rocketship init
```

This command automatically:
- 🔍 **Detects your branch/worktree name** (e.g., `feature-xyz`)
- 🎯 **Creates unique stack name** (e.g., `rocketship-feature-xyz`)
- 🔢 **Calculates unique ports** using hash-based allocation
- ⚙️ **Generates environment configuration** with zero conflicts
- 🐳 **Sets up isolated Docker stack**

### Step 3: Start and Develop

```bash
# Start your isolated stack
./.docker/rocketship start

# Your environment is ready! Use rocketship CLI directly:
rocketship run -f examples/simple-http/rocketship.yaml
rocketship team create "My Team"
rocketship auth login
rocketship validate test.yaml

# Stop when done
./.docker/rocketship stop
```

## How Auto-Discovery Works

The system provides **magical zero-configuration setup** by automatically:

### 1. Stack Naming
- **Git Branch**: `feature/user-auth` → **Stack**: `rocketship-feature-user-auth`
- **Worktree Dir**: `rocketship-api-v2` → **Stack**: `rocketship-rocketship-api-v2`
- **Special chars** converted to hyphens for Docker compatibility

### 2. Port Allocation Algorithm
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
- **Deterministic allocation** (same worktree = same ports always)
- **Zero conflicts** between different worktrees

### 3. Port Allocation Example

**Worktree 1** (`feature-api`):
- Stack: `rocketship-feature-api`
- Temporal UI: `http://localhost:8180`
- Engine API: `localhost:7800`
- All services get unique ports automatically

**Worktree 2** (`feature-ui`):
- Stack: `rocketship-feature-ui`
- Temporal UI: `http://localhost:9280`  
- Engine API: `localhost:8900`
- Completely different ports, zero conflicts!

## Essential Commands

### Docker Environment Management
```bash
./.docker/rocketship init           # Initialize stack for current worktree
./.docker/rocketship start          # Start the current stack
./.docker/rocketship stop           # Stop the current stack
./.docker/rocketship logs [service] # Show logs (never hangs)
./.docker/rocketship clean          # Stop and remove containers/volumes
```

### Use Rocketship CLI Directly
After starting your stack, use the rocketship CLI normally:
```bash
rocketship run -f test.yaml         # Run tests
rocketship team create "My Team"    # Create teams
rocketship auth login               # Authenticate
rocketship validate test.yaml       # Validate tests
rocketship list                     # List test runs
rocketship get <run-id>             # Get test details
```

The CLI automatically connects to your isolated environment!

## Development Scenarios

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
# No configuration needed, no conflicts possible
```

### Scenario 2: Team Development

```bash
# Developer A - Feature Branch
git worktree add ../rocketship-user-auth
cd ../rocketship-user-auth
./.docker/rocketship init && ./.docker/rocketship start
# Gets ports: 8xxx range automatically

# Developer B - Different Feature  
git worktree add ../rocketship-payment-flow
cd ../rocketship-payment-flow
./.docker/rocketship init && ./.docker/rocketship start
# Gets ports: 9xxx range automatically (no conflicts!)
```

### Scenario 3: AI Coding Agents (Claude Code)

Each Claude Code agent working in a different worktree gets completely isolated environments:

```bash
# Agent 1 Prompt:
# "You are working in worktree ../rocketship-feature-authentication"
# "Run: ./.docker/rocketship init && ./.docker/rocketship start"

# Agent 2 Prompt:  
# "You are working in worktree ../rocketship-feature-browser-plugin"
# "Run: ./.docker/rocketship init && ./.docker/rocketship start"

# Zero conflicts, complete isolation, autonomous setup!
```

## Architecture Deep Dive

### Complete Isolation Through:

1. **Container Naming**: Different `COMPOSE_PROJECT_NAME` per stack
2. **Network Isolation**: Each stack has its own Docker network
3. **Port Mapping**: Non-overlapping port ranges prevent conflicts
4. **Volume Separation**: Independent data volumes per stack
5. **Session Isolation**: Different test session headers prevent data interference

### Services in Each Environment

Every worktree gets its own complete Rocketship infrastructure:

**Temporal Infrastructure**:
- `temporal-postgresql-1`: Database for Temporal
- `temporal-elasticsearch-1`: Search index  
- `temporal-temporal-1`: Main Temporal server
- `temporal-temporal-ui-1`: Temporal Web UI
- `temporal-temporal-admin-tools-1`: Admin tools

**Rocketship Core**:
- `temporal-engine-1`: Rocketship gRPC engine
- `temporal-worker-1`: Rocketship Temporal worker
- Built-in CLI connects automatically

**Test Databases** (optional):
- `temporal-postgres-test-1`: PostgreSQL test database
- `temporal-mysql-test-1`: MySQL test database

**Authentication** (optional):
- `temporal-auth-postgres-1`: Authentication database
- Full OIDC integration with external providers

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
├── dynamicconfig/                   # Temporal configuration
├── test-db-init/                    # Database initialization scripts
├── auth-db-init/                    # Authentication database setup
└── .env.{stack-name}              # Generated environment files per worktree
```

## Development Workflow

### 1. Making Code Changes

```bash
# Modify code in your worktree
vim internal/plugins/http/plugin.go

# Rebuild and restart your environment
./.docker/rocketship stop
./.docker/rocketship start

# Test your changes
rocketship run -f examples/simple-http/rocketship.yaml
```

### 2. Authentication Development

Configure authentication for testing auth features:

```bash
# Set OIDC environment variables before starting
export ROCKETSHIP_OIDC_ISSUER="https://your-tenant.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="your-auth0-client-id"
export ROCKETSHIP_ADMIN_EMAILS="your-email@gmail.com"

# Start stack with authentication enabled
./.docker/rocketship start

# Test authentication features
rocketship auth login
rocketship auth status
rocketship team create "Engineering"
```

### 3. Plugin Development

Perfect for developing new plugins:

```bash
# Create worktree for plugin development
git worktree add ../rocketship-kafka-plugin
cd ../rocketship-kafka-plugin

# Initialize isolated environment
./.docker/rocketship init
./.docker/rocketship start

# Develop plugin with complete isolation
# Edit internal/plugins/kafka/plugin.go
# Test without affecting other development
```

## AI Agent Integration

### Claude Code Agent Prompts

When creating Claude Code agents for Rocketship development:

```
You are a Claude Code agent working on Rocketship development.

**Your Environment:**
- Working directory: [WORKTREE_PATH]
- Project: Rocketship development (Go codebase)
- Focus: [SPECIFIC_FEATURE_OR_BUG]

**Environment Setup:**
1. Your worktree is automatically isolated with unique ports
2. Run `./.docker/rocketship init` to initialize your environment
3. Run `./.docker/rocketship start` to start your isolated stack
4. Use `rocketship` CLI commands directly after stack starts

**Development Workflow:**
1. Make code changes to Go source files
2. Rebuild: `./.docker/rocketship stop && ./.docker/rocketship start`
3. Test: `rocketship run -f examples/[relevant-example]/rocketship.yaml`
4. Validate: `rocketship validate test.yaml`

**Testing:**
- Use `rocketship run -f examples/simple-http/rocketship.yaml` for basic testing
- Use `rocketship run -f examples/sql-testing/rocketship.yaml` for database testing
- Check logs: `./.docker/rocketship logs engine` or `./.docker/rocketship logs worker`

**Your Isolation:**
- Your ports are automatically unique (no conflicts with other agents)
- Your data is completely isolated from other development
- You can work autonomously without affecting other agents

**Key Commands:**
- `./.docker/rocketship init` - Initialize your environment
- `./.docker/rocketship start` - Start services
- `./.docker/rocketship stop` - Stop services
- `./.docker/rocketship clean` - Clean up completely
- `rocketship run -f test.yaml` - Run tests
- `rocketship validate test.yaml` - Validate test files
```

### Multi-Agent Coordination

Multiple agents can work simultaneously:

```bash
# Agent 1: Authentication feature
cd ../rocketship-auth-feature
./.docker/rocketship init && ./.docker/rocketship start
# Gets unique port range: 8xxx

# Agent 2: Browser plugin enhancement  
cd ../rocketship-browser-plugin
./.docker/rocketship init && ./.docker/rocketship start
# Gets unique port range: 9xxx

# Agent 3: SQL plugin bugfix
cd ../rocketship-sql-bugfix
./.docker/rocketship init && ./.docker/rocketship start
# Gets unique port range: 10xxx

# All agents work independently with zero conflicts!
```

## Debugging and Troubleshooting

### View Service Logs

```bash
# Engine logs
./.docker/rocketship logs engine

# Worker logs
./.docker/rocketship logs worker

# All service logs
./.docker/rocketship logs

# Follow logs in real-time
./.docker/rocketship logs engine -f
```

### Access Service UIs

Each worktree gets its own UI endpoints:

```bash
# Check your stack's ports (view running containers)
docker ps | grep rocketship

# Access Temporal UI (port varies by worktree)
# Example: http://localhost:8180 or http://localhost:9280
```

### Common Issues

**"Template file not found":**
```bash
# Known issue: Missing .env template file
# This affects the init-stack.sh script which expects templates/.env.template
# Workaround: Use existing .env files as reference or create template manually
```

**"Stack not initialized":**
```bash
# Solution: Initialize first
./.docker/rocketship init
```

**"Stack not running":**
```bash
# Solution: Start the stack
./.docker/rocketship start
```

**"Docker not running":**
```bash
# Start Docker Desktop or Docker daemon
# Then retry your command
```

**Port conflicts (should never happen):**
```bash
# Nuclear option - clean and restart
./.docker/rocketship clean
./.docker/rocketship init
./.docker/rocketship start
```

## Advanced Usage

### Multiple Worktrees Management

```bash
# List all your worktrees
git worktree list

# Clean up finished worktrees
git worktree remove ../rocketship-completed-feature
./.docker/rocketship clean  # From the worktree before removing
```

### Performance Tuning

```bash
# Check resource usage across all stacks
docker stats

# Scale workers in your environment (edit docker-compose.yaml)
# Then restart: ./.docker/rocketship stop && ./.docker/rocketship start
```

### Session Isolation

When testing against external services, use session headers:

```yaml
# In your test files
steps:
  - name: "Test with isolation"
    plugin: http
    config:
      url: "https://api.example.com/users"
      headers:
        X-Test-Session: "worktree-feature-xyz"  # Unique per worktree
```

## Benefits for Development

### For Individual Developers
- **Parallel Development**: Work on multiple features simultaneously
- **Clean Separation**: No interference between different features
- **Quick Switching**: Each worktree maintains its own state
- **Safe Experimentation**: Break things without affecting other work

### For Teams  
- **Zero Conflicts**: Everyone gets unique ports automatically
- **Easy Onboarding**: New team members just run `init` and `start`
- **Consistent Environments**: Same setup across all machines
- **Autonomous Setup**: No coordination needed between team members

### For AI Coding Agents
- **Perfect Isolation**: Each agent has its own complete environment
- **Auto-Configuration**: Agents can set up environments autonomously  
- **Conflict-Free**: Multiple agents can work simultaneously
- **Zero Dependencies**: No coordination needed between agents

## Cleanup

### Clean Single Environment
```bash
# Stop and remove containers/volumes for current worktree
./.docker/rocketship clean
```

### Clean All Environments
```bash
# Stop all rocketship containers everywhere
docker stop $(docker ps -q --filter "name=rocketship-")

# Remove all rocketship containers and volumes
docker system prune -f
docker volume prune -f
```

## Integration with Main Workflow

This multi-stack system integrates seamlessly with standard Rocketship development:

1. **Create worktree** for your feature/fix
2. **Initialize environment** with `./.docker/rocketship init` 
3. **Develop and test** using standard rocketship commands
4. **Commit and push** your changes
5. **Clean up** with `./.docker/rocketship clean` when done

The isolated environment ensures your development doesn't interfere with other work and provides a clean, reproducible testing environment for every feature.

**Ready to get started?** Just `cd` to any rocketship worktree and run `./.docker/rocketship init`! 🚀