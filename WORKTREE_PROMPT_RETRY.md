# Prompt for Claude Code Instance - Retry Key Enhancement

You are working in a git worktree called `rocketship-workflow-enhancements` on branch `retry-key-for-activities`. 

## Your Task
Create a plugin-agnostic "retry" key that maps to Temporal activity retry policies.

**Requirements:**
1. Add a `retry` key that can be used on ANY plugin step in Rocketship tests
2. Map this retry configuration to Temporal activity retry options
3. The retry policy should be plugin-agnostic (works with http, delay, sql, etc.)
4. Create an integration test in `/examples` that demonstrates this functionality
5. Ensure the integration test passes in CI

**Example of desired functionality:**
```yaml
version: "v1.0.0"
name: "Retry Policy Test"
tests:
  - name: "Test with retry policy"
    steps:
      - name: "HTTP request with retry"
        plugin: "http"
        config:
          method: "GET"
          url: "https://httpbin.org/status/500"  # Will fail
        retry:
          initial_interval: "1s"
          maximum_interval: "10s"
          maximum_attempts: 3
          backoff_coefficient: 2.0
```

## Initial Setup
Before starting work, you MUST set up your isolated Docker environment:

```bash
# Run this first to create your isolated environment
./docker/setup-worktree-env.sh

# Then start your services
cd .docker && docker-compose up -d

# Verify everything is running
docker ps | grep rocketship-workflow-enhancements
```

After setup, you'll have your own isolated:
- Temporal server
- Rocketship engine and worker
- Test databases
- Unique ports that won't conflict with other instances

## Development Workflow
1. Use your worktree-specific CLI wrapper for testing:
   ```bash
   .docker/docker-rocketship-local.sh run -f examples/retry-policy/rocketship.yaml
   ```

2. After making changes to Go code, rebuild:
   ```bash
   cd .docker
   docker-compose build engine worker
   docker build -f Dockerfile.cli -t rocketship-workflow-enhancements-cli:latest ..
   docker-compose restart engine worker
   ```

3. Run tests to verify your changes:
   ```bash
   make test
   make lint
   ```

## Key Areas to Investigate
- **DSL/Schema**: Where step configuration is defined (likely `internal/dsl/`)
- **Temporal Integration**: How activities are currently executed (look for activity options)
- **Plugin Interface**: The common plugin execution pattern (`internal/plugins/`)
- **Workflow Orchestration**: Where activities are started with retry policies
- **Example Structure**: Follow patterns from existing examples in `/examples`

## Technical Implementation Hints
1. **Retry Policy Structure**: Look at Temporal's `RetryPolicy` struct:
   - `InitialInterval`
   - `MaximumInterval` 
   - `MaximumAttempts`
   - `BackoffCoefficient`
   - `NonRetryableErrorTypes`

2. **Integration Points**: 
   - Add retry field to step schema
   - Update workflow activity execution to use retry policies
   - Ensure it works across all plugin types

3. **Testing Strategy**:
   - Create example that intentionally fails initially
   - Verify retry behavior with different policies
   - Test with multiple plugin types

## Integration Test Requirements
Create `/examples/retry-policy/` with:
- `rocketship.yaml` - Demonstrates retry functionality
- `README.md` - Explains the retry feature
- Test should include:
  - Steps that fail initially but succeed on retry
  - Different retry configurations
  - Multiple plugin types (http, delay, etc.)
  - Assertions that verify retry behavior worked

## Submission Process
After making your changes and verifying they work locally:

1. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add plugin-agnostic retry key for temporal activity retry policies

   - Add retry configuration to step schema
   - Implement retry policy mapping to temporal activities
   - Create integration test demonstrating functionality
   - Support for all plugin types"
   git push origin retry-key-for-activities
   ```

2. **Create a Pull Request**:
   ```bash
   # Use GitHub MCP to create the PR
   gh pr create --title "feat: Add plugin-agnostic retry key for activities" --body "Implements retry configuration for any plugin step that maps to Temporal activity retry policies.

   Features:
   - Plugin-agnostic retry key for all step types
   - Maps to Temporal RetryPolicy options
   - Integration test with examples
   - Backward compatible

   Fixes: Adds retry capability for improved reliability"
   ```

3. **Monitor CI Status**:
   You MUST monitor the GitHub PR check workflow and ensure it passes. Use the GitHub MCP server to:
   
   - Poll workflow status every minute until completion
   - Get your PR number and monitor workflow runs for your branch
   - Check status continuously and fix any CI failures
   - Pay special attention to the examples integration test

   Keep polling until:
   - ✅ All checks pass (CI is green)
   - ✅ Your new integration test passes in CI
   - ❌ If any checks fail, investigate logs, fix issues, and push again

## Important Notes
- You're in an isolated environment - your containers won't interfere with other instances
- Your Temporal UI will be on a unique port (shown after setup script)
- **CRITICAL**: The integration test MUST pass in CI for the task to be complete
- Use the GitHub MCP server extensively to monitor workflow status
- Test with multiple plugin types to ensure it's truly plugin-agnostic
- Consider edge cases: malformed retry configs, missing values, etc.
- The retry feature should be backward compatible (existing tests shouldn't break)

Please investigate the codebase, implement the retry functionality, create comprehensive integration tests, and monitor CI until it passes.