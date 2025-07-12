# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

Rocketship is an open-source testing framework for browser and API testing that uses Temporal for durable execution. The system is built with Go and follows a plugin-based architecture.

There are 3 "server" components that make up the Rocketship system: Temporal, Engine, and Worker. The CLI is meant to communicate with the system via the engine. There are two ways to run Rocketship:

1. The self-hosted solution calls for spinning up all of these server components separately using their Dockerfiles and having the CLI connect remotely.
2. The local solution is much faster and has the CLI just run all of the server components locally on separate processes.

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

## Running Tests

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

## Running Tests that have the SQL plugin

Make sure to spin up the sql containers before running the tests.

```bash
docker-compose -f .docker/docker-compose.yaml up postgres-test mysql-test -d
```

Run the test(s) with sql plugins

```bash
rocketship run -af examples/sql-testing/rocketship.yaml
```

Spin down the containers when you're done

```bash
docker-compose -f .docker/docker-compose.yaml down
```

## Running Tests with Browser Plugin

The browser plugin requires Python 3.11+ and browser-use installed:

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
