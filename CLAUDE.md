# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

Rocketship is an open-source testing framework for browser and API testing that uses Temporal for durable execution. The system is built with Go and follows a plugin-based architecture.

There are 3 "server" components that make up the Rocketship system: Temporal, Engine, and Worker. The CLI is meant to communicate with the system via the engine. There are three ways to run Rocketship:

1. **Docker Multi-Stack Environment** (RECOMMENDED FOR AGENTS): Isolated Docker environments with auto-port allocation
2. The self-hosted solution calls for spinning up all of these server components separately using their Dockerfiles and having the CLI connect remotely.
3. The local solution is much faster and has the CLI just run all of the server components locally on separate processes.

**Key Components:**

- **CLI (`cmd/rocketship/`)**: Main entry point that wraps the engine and worker binaries
- **Engine (`cmd/engine/`)**: gRPC server that orchestrates test execution via Temporal workflows
- **Worker (`cmd/worker/`)**: Temporal worker that executes test workflows using plugins
- **Plugins (`internal/plugins/`)**: Extensible system for different protocols (HTTP, delay, AWS services)
- **DSL Parser (`internal/dsl/`)**: Parses YAML test specifications into executable workflows
- **Orchestrator (`internal/orchestrator/`)**: Engine implementation that manages test runs and streaming logs

**Test Flow:**

1. YAML spec is parsed by DSL parser
2. Engine creates Temporal workflows for each test
3. Worker executes test steps using appropriate plugins
4. Results are streamed back via gRPC to CLI

## Development Commands

### Build and Install

```bash
make install        # Build CLI with embedded binaries and install CLI to $GOPATH/bin
```

### Testing and Quality

```bash
make lint && make test    # lint and test
```

### Embedded Binaries

The CLI embeds engine and worker binaries. Always run `make install` after modifying engine/worker code.

### Protocol Buffers

```bash
make proto          # Regenerate protobuf code from proto/engine.proto
```

### Documentation

```bash
make docs-serve     # Start local documentation server
make docs           # Build documentation
```

## Debugging and Logging

### Debug Logging

All processes (CLI, engine, worker) use unified structured logging from `internal/cli/logging.go`:

```bash
ROCKETSHIP_LOG=DEBUG rocketship run -af test.yaml    # Full debug output
ROCKETSHIP_LOG=INFO rocketship run -af test.yaml     # Info level (default)
ROCKETSHIP_LOG=ERROR rocketship run -af test.yaml    # Errors only
```

Debug logging shows:

- Process lifecycle (start, stop, cleanup)
- Temporal connections and workflow execution
- Plugin execution details
- gRPC server initialization

DEBUG LOGGING IS EXTREMELY USEFUL DURING DEVELOPMENT.

### Development Workflow

1. **Make code changes** to engine/worker/CLI
2. **Rebuild binaries**: `make install`
3. **Test with debug logging**: `ROCKETSHIP_LOG=DEBUG rocketship run -af examples/simple-http/rocketship.yaml`
4. **Run lint and test suite**: `make lint && make test`

### Local Development Binary Usage

The system automatically uses local development binaries from `internal/embedded/bin/` when available, avoiding GitHub downloads. This makes iterative development faster.

### Common Development Tasks

```bash
# Quick test with debug output
ROCKETSHIP_LOG=DEBUG rocketship run -af examples/simple-http/rocketship.yaml

# Background server for iterative testing
ROCKETSHIP_LOG=DEBUG rocketship start server --background
rocketship run --f test.yaml
rocketship stop server

# Validate YAML changes
rocketship validate test.yaml
```

## Test Specifications

Tests are defined in YAML files with this structure:

- `name`: Test suite name
- `tests[]`: Array of test cases
- `tests[].steps[]`: Sequential steps within a test
- `steps[].plugin`: Plugin to use (http, delay, aws/\*)
- `steps[].assertions[]`: Validation rules
- `steps[].save[]`: Variable extraction for step chaining

## Plugin System

Plugins implement the `Plugin` interface in `internal/plugins/plugin.go`. Each plugin has:

- `Execute()`: Main execution logic
- `Parse()`: Configuration parsing
- Plugin-specific types in separate files

Plugins include: HTTP, delay, AWS (S3, SQS, DynamoDB), SQL, log, script, agent, browser, supabase, etc.

### Variable Replacement in Plugins

**CRITICAL**: All plugins MUST use the central DSL template system for variable replacement to ensure consistency.

**DO NOT create custom variable replacement implementations.** Always use:

- `dsl.ProcessTemplate()` for runtime and environment variable processing
- `dsl.ProcessConfigVariablesRecursive()` for config variable processing in nested structures
- `dsl.TemplateContext` for providing runtime variables to the template system

**Supported Variable Types:**
- Config variables: `{{ .vars.variable_name }}` (processed by CLI before plugin execution)
- Runtime variables: `{{ variable_name }}` (processed by plugins using DSL system)
- Environment variables: `{{ .env.VARIABLE_NAME }}` (processed by DSL system)
- Escaped handlebars: `\{{ literal_handlebars }}` (handled by DSL system)

**Standard Implementation Pattern:**
```go
// Convert state to interface{} map for DSL compatibility
runtime := make(map[string]interface{})
for k, v := range state {
    runtime[k] = v
}

// Create template context with runtime variables
context := dsl.TemplateContext{
    Runtime: runtime,
}

// Use centralized template processing
result, err := dsl.ProcessTemplate(input, context)
if err != nil {
    return "", fmt.Errorf("template processing failed: %w", err)
}
```

This ensures all plugins handle variables identically and support all documented features including escaped handlebars and environment variables.

### Browser Plugin Notes

The browser plugin uses AI-driven web automation via the `browser-use` Python library. The Python script (`browser_automation.py`) is embedded into the binary using `go:embed` to ensure it's available at runtime.

## 🐳 Docker Multi-Stack Environment (RECOMMENDED)

**As a coding agent, you should ALWAYS use the isolated Docker environment instead of local binaries.** This ensures:

- ✅ **Complete isolation** from other agents
- ✅ **Unique ports** prevent conflicts  
- ✅ **Source code isolation** with tagged Docker images
- ✅ **Zero configuration** with auto-detection
- ✅ **Clean environments** for every git worktree

### Quick Start for Coding Agents

```bash
# Initialize your isolated environment (run once per worktree)
./.docker/rocketship init

# Start your stack
./.docker/rocketship start

# Run tests through your isolated environment
./.docker/rocketship run -f examples/simple-http/rocketship.yaml

# Check status of your stack
./.docker/rocketship status

# Stop when done
./.docker/rocketship stop
```

### How Auto-Detection Works

The system automatically:
1. **Detects your git branch** (e.g., `feature-api`)
2. **Creates unique stack name** (e.g., `rocketship-feature-api`) 
3. **Calculates unique ports** using hash-based allocation
4. **Builds tagged Docker images** (e.g., `rocketship-engine:feature-api`)
5. **Isolates everything** - networks, volumes, containers

### Stack Isolation Example

If multiple agents are working simultaneously:

**Agent 1** (branch: `feature-api`):
- Stack: `rocketship-feature-api`
- Temporal UI: `http://localhost:11880`
- Engine API: `localhost:11500`
- Docker Images: `rocketship-engine:feature-api`, `rocketship-worker:feature-api`

**Agent 2** (branch: `feature-ui`):
- Stack: `rocketship-feature-ui`  
- Temporal UI: `http://localhost:12780`
- Engine API: `localhost:12400`
- Docker Images: `rocketship-engine:feature-ui`, `rocketship-worker:feature-ui`

**Zero conflicts!** Each agent gets completely isolated infrastructure.

### Available Docker Commands

```bash
# Environment Management
./.docker/rocketship init        # Initialize stack for current git branch
./.docker/rocketship start       # Start the stack
./.docker/rocketship stop        # Stop the stack
./.docker/rocketship restart     # Restart the stack
./.docker/rocketship status      # Show status
./.docker/rocketship info        # Show detailed stack information
./.docker/rocketship logs        # Show logs for all services
./.docker/rocketship logs engine # Show logs for specific service
./.docker/rocketship clean       # Stop and remove all containers/volumes

# Test Commands (require running stack)
./.docker/rocketship validate <file>     # Validate test file
./.docker/rocketship run [options]       # Run tests
./.docker/rocketship list               # List test runs
./.docker/rocketship get <run-id>       # Get test run details
```

### Complete Development Workflow

```bash
# 1. Initialize your isolated environment
./.docker/rocketship init

# 2. Start the stack
./.docker/rocketship start

# 3. Make code changes to engine/worker/CLI
# ... edit files ...

# 4. CRITICAL: Force rebuild Docker images after code changes
# The system may not detect all changes - always force rebuild to be safe
./.docker/rocketship stop
docker rmi rocketship-engine:$(git branch --show-current) rocketship-worker:$(git branch --show-current) 2>/dev/null || true
./.docker/rocketship start

# 5. Test your changes
./.docker/rocketship run -f examples/simple-http/rocketship.yaml

# 6. Run additional tests
./.docker/rocketship run -f examples/complex-http/rocketship.yaml

# 7. Check test history
./.docker/rocketship list

# 8. Stop stack when done
./.docker/rocketship stop
```

### Integration with Test Session Isolation

When running HTTP tests, always use unique session headers to prevent interference with other agents:

```yaml
steps:
  - name: "Test with agent isolation"
    plugin: http
    config:
      url: "https://tryme.rocketship.sh/users"
      headers:
        X-Test-Session: "{{ .vars.agent_session_id }}"  # Use your unique session
```

### Docker Environment Critical Reminders

**🚨 ALWAYS REBUILD IMAGES AFTER CODE CHANGES:**
```bash
# Docker may not detect all source code changes, especially:
# - Changes to internal/ packages
# - Environment variable updates
# - Binary embedding changes

# ALWAYS force rebuild when you make changes:
./.docker/rocketship stop
docker rmi rocketship-engine:$(git branch --show-current) rocketship-worker:$(git branch --show-current) 2>/dev/null || true
./.docker/rocketship start
```

**🚨 ENVIRONMENT VARIABLE GOTCHAS:**
```bash
# Environment variables in .docker/.env.{branch} are loaded into Docker containers
# Changes to .env files require container restart (not just rebuild)
./.docker/rocketship restart

# Build and install CLI
make install
```

**🚨 PORT CONFUSION:**
```bash
# Docker engine runs on a different port than standalone (e.g., 12100 vs 7700)
# Always check your .docker/.env.{branch} file for the correct ENGINE_PORT
# Connect with: rocketship run -f test.yaml --engine localhost:{ENGINE_PORT}
```

**🚨 VOLUME MOUNT ISSUES:**
```bash
# Certificate permissions for HTTPS in Docker:
chmod -R 755 ~/.rocketship/certs/

# Docker Desktop on macOS may have file sharing restrictions
# Ensure ~/Projects or wherever rocketship is located is shared in Docker Desktop settings
```

### Troubleshooting Docker Environment

**Stack not initialized**: Run `./.docker/rocketship init` first

**Port conflicts**: The auto-allocation system prevents this, but if it happens:
- Check what's using the port: `lsof -i :PORT_NUMBER`
- Clean and restart: `./.docker/rocketship clean && ./.docker/rocketship start`

**Images not updating despite rebuild**: 
- Docker build cache issues: `docker system prune -f && ./.docker/rocketship start`
- Check if you're in the right git branch: `git branch --show-current`

**Container logs for debugging**:
```bash
# View logs for all services
./.docker/rocketship logs

# View logs for specific service  
./.docker/rocketship logs engine
./.docker/rocketship logs worker
```

**Complete environment reset**:
```bash
# Nuclear option - removes everything and starts fresh
./.docker/rocketship clean
docker system prune -f
./.docker/rocketship init
./.docker/rocketship start
```

**Docker build hanging**: If `./.docker/rocketship start` hangs during build:
- Use manual command: `cd .docker && docker-compose --env-file .env.add-auth up -d --build`
- Force clean rebuild: `./.docker/rocketship clean` then `./.docker/rocketship start`

### HTTPS/TLS Docker Setup Reminders

**🔐 HTTPS is WORKING in Docker Environment:**
- Engine serves HTTPS on port configured in `.docker/.env.{branch}` (e.g., ENGINE_PORT=12100)
- Self-signed certificates work perfectly for development/testing
- Authentication is integrated and working with HTTPS

**🔐 HTTPS Architecture:**
```bash
# Engine serves plain gRPC internally (no TLS)
./.docker/rocketship logs engine | grep "grpc server listening"
# Should see: "level=INFO msg="grpc server listening" port=:7700"

# TLS termination happens at ingress (enterprise pattern)
# CLI connects to HTTPS ingress using system CA certificates
rocketship run -f test.yaml --engine https://globalbank.rocketship.sh
```

**🔐 Let's Encrypt Known Issues:**
- HTTP-01 challenge FAILS with Cloudflare tunnels (use self-signed instead)
- DNS-01 challenge needs implementation for real domain certificates
- For production: Generate certificates externally and mount them

**🔐 Certificate Management in Docker:**
```bash
# Generate self-signed certificate (works immediately)
rocketship certs generate --domain globalbank.rocketship.sh --email your@email.com --local

# Fix certificate permissions for Docker
chmod -R 755 ~/.rocketship/certs/

# Certificates are automatically mounted into Docker containers at:
# /root/.rocketship/certs (read-only)
```

## Running Tests (Legacy Local Mode)

**⚠️ CODING AGENTS: Use the Docker environment above instead of these local commands.**

```bash
rocketship run -af test.yaml    # Auto-start local engine, run tests, auto-stop engine
rocketship start server -b      # Start engine locally in the background
rocketship run test.yaml        # Run against existing engine (defaults to localhost:7700)
rocketship stop server          # Stop local background engine
```

### Engine Dependencies for Commands

**Important**: Some commands require a running engine to communicate with:

- `rocketship get` - Requires running engine to fetch run details
- `rocketship list` - Requires running engine to list test runs
- `rocketship validate` - Works offline (no engine required)

**Workflow for using get/list commands:**

```bash
# Start server in background
rocketship start server -b

# Run tests (keeps engine running)
rocketship run test.yaml

# Now you can use get/list commands
rocketship list runs
rocketship get run <run-id>

# Stop server when done
rocketship stop server
```

**Auto mode (-a flag)**: Starts engine, runs tests, then shuts down engine automatically. Use this for simple test execution, but you won't be able to use get/list commands afterward since the engine stops.

## Running Tests with SQL Plugin

**Using Docker Environment (RECOMMENDED):**

The SQL test databases (PostgreSQL and MySQL) are automatically included in your isolated Docker stack:

```bash
# Start your stack (includes SQL databases)
./.docker/rocketship start

# Run SQL tests through your isolated environment
./.docker/rocketship run -f examples/sql-testing/rocketship.yaml

# Stop when done
./.docker/rocketship stop
```

**Legacy Local Mode (for reference only):**

```bash
docker-compose -f .docker/docker-compose.yaml up postgres-test mysql-test -d
rocketship run -af examples/sql-testing/rocketship.yaml
docker-compose -f .docker/docker-compose.yaml down
```

## Running Tests with Browser Plugin

**Using Docker Environment (RECOMMENDED):**

Browser dependencies are included in the Docker worker container:

```bash
# Start your stack
./.docker/rocketship start

# Run browser tests through your isolated environment
./.docker/rocketship run -f examples/browser-testing/rocketship.yaml

# Stop when done
./.docker/rocketship stop
```

**Legacy Local Mode (for reference only):**

```bash
# Install browser-use and its dependencies
pip install browser-use playwright langchain-openai langchain-anthropic

# Install Playwright browsers
playwright install chromium

# Run browser tests
rocketship run -af examples/browser-automation/rocketship.yaml
```

## Testing Against tryme Server

For testing purposes, there's a hosted test server at `tryme.rocketship.sh` that provides endpoints for testing HTTP requests. This server is useful for development and testing without requiring external services.

The tryme server features:
- Test CRUD operations for a resource type
- Resources are isolated based off a session header
- Resource cleanup is done hourly (every :00)

### Test Session Isolation

When multiple coding agents are testing simultaneously, use the `X-Test-Session` header to ensure complete isolation between test sessions:

```yaml
steps:
  - name: "Test with session isolation"
    plugin: http
    config:
      url: "https://tryme.rocketship.sh/users"
      method: "POST"
      headers:
        X-Test-Session: "unique-session-id-for-this-agent"
      body: |
        {
          "name": "Test User",
          "email": "test@example.com"
        }
```

**Important**: Each coding agent should use a unique value for the `X-Test-Session` header to prevent interference between concurrent test runs. This ensures that:

- Test data is isolated per session
- Concurrent agents don't affect each other's test results
- Each agent gets its own isolated test environment

Example session ID patterns:
- `agent-1-timestamp-hash`
- `worktree-name-uuid`
- `feature-branch-random-id`

This isolation is particularly important when using the Docker worktree setup where multiple agents may be testing simultaneously.

## 🔒 HTTPS/TLS Implementation Guide

### Certificate Management System

Rocketship includes a complete certificate management system supporting:
- **Self-signed certificates** for development/demo
- **Let's Encrypt certificates** for production 
- **Cloudflared tunnel integration** for local HTTPS validation

### Key Commands

```bash
# Generate self-signed certificate (works immediately)
rocketship certs generate --domain globalbank.rocketship.sh --self-signed

# Generate Let's Encrypt certificate with local tunnel
rocketship certs generate --domain globalbank.rocketship.sh --email admin@company.com --local

# Check certificate status
rocketship certs status

# Start server with HTTPS
rocketship start server --https --domain globalbank.rocketship.sh
```

### Docker HTTPS Configuration

To enable HTTPS in Docker environment:

1. **Generate certificate**: `rocketship certs generate --domain yourdomain --self-signed`
2. **Configure environment**: Add to `.docker/.env.{branch}`:
   ```bash
   ROCKETSHIP_TLS_ENABLED=true
   ROCKETSHIP_TLS_DOMAIN=yourdomain
   ROCKETSHIP_LOG=DEBUG  # For TLS debugging
   ```
3. **Fix certificate permissions**: `chmod -R 755 ~/.rocketship/certs/`
4. **Restart stack**: `./.docker/rocketship stop && ./.docker/rocketship start`

### Testing Environment Configuration  

**Critical for Testing**: TLS environment variables affect CLI tests. For `make install`:

```bash
# Disable TLS for build/test
unset ROCKETSHIP_TLS_ENABLED
unset ROCKETSHIP_TLS_DOMAIN
make install

# Re-enable for runtime
source test-env.sh  # Contains TLS configuration
```

### Docker Build Issues

**If Docker images don't reflect code changes**:
1. **Test fails**: CLI tests fail with TLS handshake errors when `ROCKETSHIP_TLS_ENABLED=true` is set during build
2. **Solution**: Temporarily unset TLS environment variables before `make install`
3. **Force rebuild**: Remove Docker images and restart:
   ```bash
   docker images | grep rocketship-engine | awk '{print $3}' | xargs docker rmi -f
   docker images | grep rocketship-worker | awk '{print $3}' | xargs docker rmi -f
   ./.docker/rocketship start
   ```

### Certificate Permissions

**Common issue**: Docker containers can't access certificates due to restrictive permissions.
**Solution**: `chmod -R 755 ~/.rocketship/certs/`

### TLS Debug Verification

Engine logs should show:
```
level=DEBUG msg="TLS environment check" raw_enabled=true enabled=true domain=yourdomain
level=INFO msg="loading TLS certificate" domain=yourdomain  
level=INFO msg="grpc server listening with TLS" port=:7700 domain=yourdomain
```

Client logs should show:
```
level=DEBUG msg="TLS enabled for gRPC client" domain=yourdomain
level=DEBUG msg="loading custom certificate for TLS connection" domain=yourdomain
```

### Let's Encrypt Known Issues

**HTTP-01 Challenge with Cloudflare**: Current implementation doesn't work with Cloudflare tunnels due to proxy interference.
**Workaround**: Use self-signed certificates for demos, implement DNS-01 challenge for production.

### Production Deployment

For production HTTPS with authentication:
1. Generate real domain certificate (Let's Encrypt or custom)
2. Configure Docker environment with TLS enabled
3. Set authentication environment variables
4. Mount certificate directory with proper permissions
5. Test complete flow: certificate → TLS → authentication → testing
