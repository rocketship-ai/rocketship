# Isolated Docker Environment for Git Worktrees

This setup allows multiple Claude Code instances to work in parallel using isolated Docker environments. Each git worktree gets its own set of containers with unique names and ports.

## Quick Start

1. **Create a git worktree:**
   ```bash
   git worktree add ../my-feature-branch -b my-feature
   cd ../my-feature-branch
   ```

2. **Setup isolated environment:**
   ```bash
   .docker/setup-worktree-env.sh
   ```

3. **Start services:**
   ```bash
   .docker/start-services.sh
   ```

4. **Run tests:**
   ```bash
   .docker/docker-rocketship-local.sh run -f test.yaml
   ```

5. **Stop services when done:**
   ```bash
   .docker/stop-services.sh
   ```

## How It Works

- **Unique Project Names**: Each worktree gets containers named `rocketship-{worktree-name}-*`
- **Port Isolation**: Each worktree uses different ports based on a hash of the worktree name
- **Shared Network**: All instances use the same `temporal-network` for simplicity
- **Environment Variables**: Each worktree gets its own `.env.local` file with unique values

## Files Generated

The setup script creates:

- `.docker/.env.local` - Environment variables for this worktree
- `.docker/docker-compose.override.yml` - Container names and port mappings
- `.docker/start-services.sh` - Start script that loads both .env files
- `.docker/stop-services.sh` - Stop script for cleanup
- `.docker/docker-rocketship-local.sh` - CLI wrapper for this environment

## Port Allocation

Each worktree gets unique ports based on a hash of the worktree name:

- **Temporal UI**: 8080 + offset
- **Engine**: 7700 + offset  
- **Engine Metrics**: 7701 + offset
- **PostgreSQL Test**: 5433 + offset
- **MySQL Test**: 3307 + offset

## Troubleshooting

**Engine connection errors:**
- Make sure services are running: `.docker/start-services.sh`
- Check container names: `docker ps | grep rocketship-{worktree-name}`

**Port conflicts:**
- Each worktree automatically gets different ports
- Check assigned ports in the setup script output

**Environment variable issues:**
- The start script loads both `.env` and `.env.local`
- All Temporal variables are included in `.env.local`

## Example Workflow

```bash
# Create worktree for bug fixes
git worktree add ../rocketship-bugs -b fix-validation

# Switch to worktree
cd ../rocketship-bugs

# Setup isolated environment  
.docker/setup-worktree-env.sh

# Start services
.docker/start-services.sh

# Run tests
.docker/docker-rocketship-local.sh run -af examples/simple-http/rocketship.yaml

# Work on fixes...

# Stop services when done
.docker/stop-services.sh
```

This allows multiple developers or Claude instances to work on different features simultaneously without container conflicts.