# AGENTS.md

This file provides guidance to AI coding agents (including this Codex CLI assistant) when working with code in this repository.

> **Versioning note:** Rocketship is still pre-1.0. There is **no backwards-compatibility requirement** for any interface, schema, or behaviour. Optimise for the current epic even if it means breaking past behaviour; do not preserve legacy code paths for compatibility unless the user explicitly asks.

## Rocketship Cloud v1 Snapshot (for agents)

- Product focus: hosted cloud with GitHub SSO (device flow for CLI, OAuth for web) backed by our auth-broker that mints Rocketship JWTs. Engine + worker still run tests via Temporal.
- Tenancy: **Org → Project**. Projects reference repo URL, default branch, and `path_scope` globs for mono-repo isolation. No “workspace” layer.
- Roles: project-level **Read** (view only) and **Write** (run/edit). Org Admins inherit Write on all projects. Tokens must carry explicit roles; missing roles are rejected.
- Git-as-SoT: UI/CLI can run uncommitted edits immediately (flagged as `config_source=uncommitted`). Approvals/merges happen in GitHub; Rocketship can optionally open PRs or commits if the user has push rights.
- Tokens: user JWT + refresh issued by broker; CI tokens are opaque secrets scoped per project with explicit permissions + TTL. Engine tags runs with `initiator`, `environment`, `config_source`, `commit_sha`/`bundle_sha` for auditability.
- Auth broker persists orgs/users/memberships in Postgres. Fresh logins return `pending` roles until the user creates or joins an org via `POST /api/orgs`.
- Guardrails: enforce path scopes, reject unknown RPCs in auth, clarify uncommitted runs, prefer minikube Helm flow for reproducible clusters.

## Architecture Overview

Rocketship is an open-source testing framework for browser and API testing that uses Temporal for durable execution. The system is built with Go and follows a plugin-based architecture.

There are 3 "server" components that make up the Rocketship system: Temporal, Engine, and Worker. The CLI is meant to communicate with the engine. There are three ways to run Rocketship:

1. **Minikube stack** (RECOMMENDED FOR AGENTS): `scripts/install-minikube.sh` provisions Temporal + Rocketship inside an isolated cluster per branch.
2. **Self-hosted cluster**: Deploy the Helm charts to your own Kubernetes environment and connect the CLI remotely.
3. **Local processes**: Use `rocketship start server` / `rocketship run -af` for quick experiments without Kubernetes.

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

## 🚀 Minikube Environment (RECOMMENDED)

The legacy `.docker/rocketship` wrapper has been replaced by a maintained Minikube workflow. Use it to get an isolated Temporal + Rocketship stack per branch.

### Quick Start

```bash
scripts/install-minikube.sh
```

The script initializes (or reuses) a Minikube profile, builds fresh engine/worker images inside the cluster, installs Temporal, registers the workflow namespace, and deploys the Rocketship chart.

### Customisation

Environment variables you can override:

| Variable                      | Default      | Description                                   |
| ----------------------------- | ------------ | --------------------------------------------- |
| `MINIKUBE_PROFILE`            | `rocketship` | Minikube profile name                         |
| `ROCKETSHIP_NAMESPACE`        | `rocketship` | Namespace for Rocketship deployments          |
| `TEMPORAL_NAMESPACE`          | `rocketship` | Namespace for the Temporal release            |
| `TEMPORAL_WORKFLOW_NAMESPACE` | `rocketship` | Temporal logical namespace used by Rocketship |
| `ROCKETSHIP_RELEASE`          | `rocketship` | Helm release for Rocketship                   |
| `TEMPORAL_RELEASE`            | `temporal`   | Helm release for Temporal                     |

### Usage Workflow

```bash
# 1. Provision / update the stack
scripts/install-minikube.sh

# 2. Inspect resources
kubectl get pods -n rocketship

# 3. Port-forward when running CLI commands locally
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube

# 4. Run tests
rocketship run -af examples/simple-http/rocketship.yaml

# 5. Tear down when finished
helm uninstall rocketship temporal -n rocketship
kubectl delete namespace rocketship
minikube delete -p rocketship
```

Always include a unique `X-Test-Session` header when calling shared services like `https://tryme.rocketship.sh` to avoid cross-test contamination.

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

- **Minikube stack:** run `scripts/install-minikube.sh`, port-forward the engine, then execute `rocketship run -af examples/sql-testing/rocketship.yaml`.
- **Standalone Docker containers:**

  ```bash
  docker run --rm -d --name rocketship-postgres     -e POSTGRES_PASSWORD=testpass     -e POSTGRES_DB=testdb     -p 5433:5432     postgres:13

  docker run --rm -d --name rocketship-mysql     -e MYSQL_ROOT_PASSWORD=testpass     -e MYSQL_DATABASE=testdb     -p 3306:3306     mysql:8.0
  ```

  Update DSNs accordingly and stop the containers after testing.

## Running Tests with Browser Plugin

When using Minikube, the worker image built by `scripts/install-minikube.sh` already contains the Python and Playwright dependencies. After the script completes, port-forward the engine and run:

```bash
rocketship run -af examples/browser-testing/rocketship.yaml
```

For manual local setups, install the dependencies once:

```bash
pip install browser-use playwright langchain-openai langchain-anthropic
playwright install chromium
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
