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

# 4. Test your changes (rebuilds images automatically if needed)
./.docker/rocketship run -f examples/simple-http/rocketship.yaml

# 5. Run additional tests
./.docker/rocketship run -f examples/complex-http/rocketship.yaml

# 6. Check test history
./.docker/rocketship list

# 7. Stop stack when done
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

### Troubleshooting Docker Environment

**Stack not initialized**: Run `./.docker/rocketship init` first

**Port conflicts**: The auto-allocation system prevents this, but if it happens:
- Check what's using the port: `lsof -i :PORT_NUMBER`
- Clean and restart: `./.docker/rocketship clean && ./.docker/rocketship start`

**Images not updating**: The system rebuilds automatically when source changes, but you can force rebuild:
- Stop stack: `./.docker/rocketship stop`
- Start again: `./.docker/rocketship start`

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
