# Rocketship Multi-Stack Docker Environment

This directory provides a **completely automated multi-stack Docker environment** for Rocketship development. It's designed specifically for **git worktree workflows** where each worktree gets its own isolated Docker environment with **zero configuration required**.

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

# Run tests in your environment
./.docker/rocketship run -f test.yaml

# Check status
./.docker/rocketship status

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

### Environment Management
```bash
./rocketship init                    # Initialize stack for current worktree
./rocketship start                   # Start the current stack
./rocketship stop                    # Stop the current stack
./rocketship restart                 # Restart the current stack
./rocketship status                  # Show status of current stack
./rocketship info                    # Show detailed stack information
./rocketship logs [service]          # Show logs (optionally for specific service)
./rocketship clean                   # Stop and remove all containers and volumes
```

### Test Commands
```bash
./rocketship validate <file>         # Validate test file
./rocketship run [options]           # Run tests (pass options to rocketship CLI)
./rocketship list                    # List test runs
./rocketship get <run-id>            # Get test run details
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

---

**Ready to get started?** Just `cd` to any rocketship worktree and run `./.docker/rocketship init`! 🚀