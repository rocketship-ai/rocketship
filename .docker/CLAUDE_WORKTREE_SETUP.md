# Instructions for Claude Code Instance

You are working in a git worktree of the Rocketship project. To ensure you have an isolated Docker environment that doesn't conflict with other Claude instances, please follow these steps:

## Initial Setup (Run Once)

1. First, run the setup script to create your isolated environment:
```bash
./docker/setup-worktree-env.sh
```

This will:
- Generate unique container names based on your worktree name
- Assign unique ports to avoid conflicts
- Create local configuration files

2. Start your isolated Docker environment:
```bash
cd .docker
docker-compose up -d
```

3. Verify everything is running:
```bash
docker ps | grep $(basename $(pwd))
```

## Using Your Isolated Environment

### Running Tests
Use the generated wrapper script:
```bash
.docker/docker-rocketship-local.sh run -f test.yaml
.docker/docker-rocketship-local.sh list runs
```

### Accessing Services
Your setup script will show you the unique ports assigned to your instance:
- Temporal UI: Check the output from setup script for your unique port
- Engine: Your unique engine port
- Test databases: Your unique PostgreSQL and MySQL ports

### Important Notes
- **Never use the default ports** - always use your worktree-specific ports
- **Your container names include your worktree name** to ensure uniqueness
- **Each worktree has its own network** to prevent cross-contamination
- **Your data is isolated** - other Claude instances cannot see your test runs

### Checking Your Configuration
```bash
cat .docker/.env.local  # Shows your unique ports and project name
```

### Cleanup (When Done)
```bash
cd .docker
docker-compose down -v  # Removes containers and volumes
```

## Example Workflow

```bash
# 1. Setup (once)
./docker/setup-worktree-env.sh

# 2. Start services
cd .docker && docker-compose up -d

# 3. Run tests
cd .. # back to project root
.docker/docker-rocketship-local.sh run -f examples/simple-http/rocketship.yaml

# 4. Check results
.docker/docker-rocketship-local.sh list runs
```

Remember: Your environment is completely isolated from other Claude instances working on different worktrees!