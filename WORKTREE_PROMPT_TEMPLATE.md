# Prompt for Claude Code Instance in Git Worktree

You are working in a git worktree called `rocketship-bugs` on branch `top-level-var-patches`. 

## Your Task
Fix the following bug: "Don't make top-level vars key be required and completely remove the top-level version from the spec"

This means:
1. The `vars` key at the top level of Rocketship YAML files should be optional, not required
2. The `version` field should be completely removed from the spec (no longer required or used)

## Initial Setup
Before starting work, you MUST set up your isolated Docker environment:

```bash
# Run this first to create your isolated environment
./docker/setup-worktree-env.sh

# Then start your services
cd .docker && docker-compose up -d

# Verify everything is running
docker ps | grep rocketship-bugs
```

After setup, you'll have your own isolated:
- Temporal server
- Rocketship engine and worker
- Test databases
- Unique ports that won't conflict with other instances

## Development Workflow
1. Use your worktree-specific CLI wrapper for testing:
   ```bash
   .docker/docker-rocketship-local.sh run -f examples/simple-http/rocketship.yaml
   ```

2. After making changes to Go code, rebuild:
   ```bash
   cd .docker
   docker-compose build engine worker
   docker build -f Dockerfile.cli -t rocketship-bugs-cli:latest ..
   docker-compose restart engine worker
   ```

3. Run tests to verify your changes:
   ```bash
   make test
   make lint
   ```

## Key Files to Investigate
- JSON schema definitions (likely in `internal/` or schema files)
- YAML parser/validator code
- Any test files that might be asserting these fields are required
- Example YAML files that use `version` or `vars`

## Submission Process
After making your changes and verifying they work locally:

1. **Commit your changes**:
   ```bash
   git add .
   git commit -m "fix: make vars optional and remove version requirement from spec"
   git push origin top-level-var-patches
   ```

2. **Create a Pull Request**:
   ```bash
   # Use GitHub MCP to create the PR
   gh pr create --title "Fix: Make vars optional and remove version requirement" --body "Fixes the bug where vars key was required and removes version field from spec completely"
   ```

3. **Monitor CI Status**:
   You MUST monitor the GitHub PR check workflow and ensure it passes. Use the GitHub MCP server to:
   
   ```bash
   # Poll the workflow status every minute until it completes
   # Get your PR number first, then monitor the workflow runs
   # Example workflow:
   # 1. List your PRs to get the PR number
   # 2. Get workflow runs for your branch
   # 3. Check the status every minute
   # 4. If it fails, check the logs and fix the issues
   # 5. Repeat until CI passes
   ```

   Keep polling until:
   - ✅ All checks pass (CI is green)
   - ❌ If any checks fail, investigate the logs, fix the issues, and push again

## Important Notes
- You're in an isolated environment - your containers won't interfere with other Claude instances
- Your Temporal UI will be on a unique port (shown after running setup script)
- **CRITICAL**: Do not consider the task complete until the GitHub CI passes
- Use the GitHub MCP server extensively to monitor workflow status
- If CI fails, debug using the workflow logs and fix the issues
- The PR should only be considered ready when all GitHub checks are green

Please investigate the codebase, make the necessary changes, test thoroughly, create a PR, and monitor CI until it passes.