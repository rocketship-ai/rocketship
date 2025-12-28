# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> **Versioning note:** Rocketship remains a v0 product. There are **no backwards-compatibility guarantees**. Remove or break legacy behaviours whenever it simplifies the current work, unless the user explicitly requests otherwise.

## Rocketship Cloud v1 Snapshot (for agents)

- Hosted experience uses GitHub device flow/OAuth via the controlplane; controlplane issues Rocketship JWTs checked by the engine. Engine, worker, Temporal remain the execution core.
- Tenancy model is **Org → Project** (no workspaces). Projects capture repo URL, default branch, and `path_scope` globs to stay mono-repo friendly.
- RBAC: project roles are **Read** and **Write**; Org Admins implicitly have Write everywhere. Tokens without role claims must be rejected.
- Git-as-Source-of-Truth: UI/CLI can run uncommitted edits (labelled `config_source=uncommitted`). Rocketship helps users open PRs/commits but does not host approval workflows—everything merges in GitHub.
- Tokens: user access/refresh pair (JWT + opaque refresh) plus per-project CI tokens with scopes and TTL. Engine annotates runs with initiator/environment/config source metadata for auditing.
- Controlplane persists users/orgs in Postgres. Initial logins return `pending` roles until an org is created (`POST /api/orgs`) or the user is invited by an admin.
- Guardrails & direction: enforce path scopes, deny unknown RPCs, keep minikube Helm flow as reference implementation.

## Architecture Overview

Rocketship is an open-source testing framework for browser and API testing that uses Temporal for durable execution. The system is built with Go and follows a plugin-based architecture.

There are 3 "server" components that make up the Rocketship system: Temporal, Engine, and Worker. The CLI is meant to communicate with the engine. There are three ways to run Rocketship:

1. **Minikube stack with Skaffold** (RECOMMENDED FOR AGENTS): `scripts/setup-local-dev.sh` + `scripts/start-dev.sh` provisions Temporal + Rocketship inside an isolated cluster with hot-reloading via Skaffold. Make code changes and watch them automatically rebuild and redeploy.
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
rocketship run --debug -af test.yaml    # Full debug output
rocketship run -af test.yaml            # Info level (default)
ROCKETSHIP_LOG=ERROR rocketship run -af test.yaml    # Errors only
```

Debug logging shows:

- Process lifecycle (start, stop, cleanup)
- Temporal connections and workflow execution
- Plugin execution details
- gRPC server initialization

DEBUG LOGGING IS EXTREMELY USEFUL DURING DEVELOPMENT.

### Advanced Debugging Techniques

When debugging complex issues with plugins or workflow state:

```bash
# Run with debug logging and save to file for analysis
rocketship run --debug -af test.yaml --env-file .env 2>&1 > /tmp/debug.log

# Search for specific plugin activity logs
cat /tmp/debug.log | grep -A 10 "SUPABASE Activity"

# Find logs for a specific step by Activity ID
cat /tmp/debug.log | grep -A 5 "ActivityID 47"

# Search for save/state-related logs
cat /tmp/debug.log | grep -E "(Processing save|saved values|State after step)"

# Find all logs for a specific workflow step
cat /tmp/debug.log | grep -A 20 "step 2:"

# Check for variable replacement issues
cat /tmp/debug.log | grep -E "(undefined variables|failed to parse template)"
```

**Key Log Patterns to Look For:**

- `SUPABASE Activity called` - Shows parameters passed to Supabase plugin
- `Processing save configs` - Indicates save operations are being processed
- `State after step N` - Shows workflow state after each step (check if variables are saved)
- `DEBUG processSave` - Shows response data structure during save operations
- `Successfully saved value` - Confirms a value was extracted and saved

**Common Issues:**

1. **Empty state after step**: Variable extraction failed, check `responseData` in logs
2. **undefined variables error**: Variable not saved in previous step, check save configs
3. **null responseData**: API returned error or empty response, check for error logs

### Development Workflow

1. **Make code changes** to engine/worker/CLI
2. **Rebuild binaries**: `make install`
3. **Test with debug logging**: `rocketship run --debug -af examples/simple-http/rocketship.yaml`
4. **Run lint and test suite**: `make lint && make test`

### Local Development Binary Usage

The system automatically uses local development binaries from `internal/embedded/bin/` when available, avoiding GitHub downloads. This makes iterative development faster.

### Common Development Tasks

```bash
# Quick test with debug output
rocketship run --debug -af examples/simple-http/rocketship.yaml

# Background server for iterative testing
rocketship --debug start server --background
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

Plugins include: HTTP, delay, AWS (S3, SQS, DynamoDB), SQL, log, script, agent, playwright, browser_use, supabase, etc.

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

### Browser Testing Plugins

Rocketship provides three browser testing plugins:

- **`playwright`**: For low-level browser control and scripted actions (Python-based, uses Playwright library)
- **`browser_use`**: For AI-driven web automation using natural language tasks (Python-based, uses `browser-use` library)
- **`agent`**: For AI-driven testing using Claude with MCP servers (Python-based, uses Claude Agent SDK)

All three plugins support persistent browser sessions. The `playwright` and `browser_use` plugins manage browser instances directly, while the `agent` plugin can connect to browser sessions via Chrome DevTools Protocol (CDP) when using the Playwright MCP server (`@playwright/mcp`). This allows the agent to control browsers launched by the playwright plugin, enabling powerful multi-step workflows where low-level browser setup can be combined with high-level AI-driven interactions.

**Key Differences:**
- **`playwright`**: Direct Python Playwright API control - best for deterministic, scripted browser actions
- **`browser_use`**: AI agent with browser control - best for complex navigation with natural language
- **`agent`**: Claude-powered agent with MCP tool access - best for test verification, analysis, and workflows requiring multiple tool types (browser via Playwright MCP, filesystem, APIs, etc.)

All Python-based browser plugins have their scripts embedded into the binary using `go:embed`.

## 🚀 Minikube Environment with Skaffold (RECOMMENDED)

The Minikube workflow uses Skaffold for automatic hot-reloading of backend services. This enables rapid iterative development - make code changes and watch them automatically rebuild and redeploy to Kubernetes.

### Quick Start

```bash
# From the repository root

# 1. One-time infrastructure setup
scripts/setup-local-dev.sh

# 2. Configure local DNS
echo "127.0.0.1 auth.minikube.local" | sudo tee -a /etc/hosts

# 3. Start everything with auto hot-reloading
scripts/start-dev.sh
```

The `start-dev.sh` script automatically:
- Starts minikube tunnel
- Starts Vite dev server for the web UI
- Runs Skaffold in dev mode watching for code changes

**Hot Reloading**: Edit any Go file in `cmd/engine/`, `cmd/worker/`, `cmd/controlplane/`, or `internal/` and save. Skaffold will automatically rebuild the Docker image and redeploy to Kubernetes. Logs stream to your terminal.

Press `Ctrl+C` to stop all processes.

### Infrastructure Setup (One-Time)

The `scripts/setup-local-dev.sh` script sets up infrastructure but does NOT deploy Rocketship services (Skaffold handles deployment for hot-reloading):

1. Start (or reuse) a Minikube profile named `rocketship`.
2. Install the Temporal Helm chart with a single-replica, batteries-included stack.
3. Register the Temporal namespace used by Rocketship.
4. Create all required Kubernetes secrets (GitHub OAuth, Postgres, JWT signing keys).
5. Deploy vite-relay for web UI development.

**Note**: You only need to run this once per environment, or when secrets change.

### Skaffold Configuration

The `skaffold.yaml` file in the repository root configures:

- **Build**: Three Docker images (engine, worker, controlplane) built inside minikube
- **Deploy**: Helm-based deployment using `charts/rocketship/values-minikube-local.yaml`
- **Watch**: File watching for automatic rebuilds on Go source changes
- **Access**: All traffic goes through minikube tunnel → ingress (single consistent path)
- **Profiles**:
  - `debug`: Enables debug logging for all services

### Manual Development Workflow

If you prefer to run components separately:

```bash
# 1. Start minikube tunnel (separate terminal)
sudo minikube tunnel -p rocketship

# 2. Start Vite dev server (separate terminal)
cd web && npm run dev

# 3. Run Skaffold (watches for changes and rebuilds)
skaffold dev
```

### Customisation

Environment variables let you change resource names without editing scripts:

| Variable                      | Default      | Description                                   |
| ----------------------------- | ------------ | --------------------------------------------- |
| `MINIKUBE_PROFILE`            | `rocketship` | Minikube profile name                         |
| `ROCKETSHIP_NAMESPACE`        | `rocketship` | Namespace for Rocketship resources            |
| `TEMPORAL_NAMESPACE`          | `rocketship` | Namespace for the Temporal Helm release       |
| `TEMPORAL_WORKFLOW_NAMESPACE` | `default`    | Temporal logical namespace registered via CLI |
| `POSTGRES_PASSWORD`           | `rocketship-dev-password` | Postgres password            |

Example:

```bash
ROCKETSHIP_NAMESPACE=testing scripts/setup-local-dev.sh
```

### Verifying the Stack

```bash
kubectl get pods -n rocketship
kubectl get svc -n rocketship
```

### Accessing Services

**Web UI**: `http://auth.minikube.local` (after running `start-dev.sh`)

**CLI access** (if needed):
```bash
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube
```

### Cleanup

```bash
# Stop Skaffold (Ctrl+C if running)

# Uninstall Helm releases
helm uninstall rocketship temporal -n rocketship

# Delete namespace
kubectl delete namespace rocketship

# Delete cluster
minikube delete -p rocketship
```

Minikube keeps everything self-contained, so you can safely destroy and recreate the environment per branch.

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

**Using the Minikube stack (recommended):** the Temporal and Rocketship pods run inside the cluster created by `scripts/install-minikube.sh`. After the script completes, port-forward the engine and run the suite:

```bash
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship run -af examples/sql-testing/rocketship.yaml
```

**Ad-hoc local databases (optional):** if you only need throwaway databases for quick experiments, you can spin them up with Docker:

```bash
# PostgreSQL
docker run --rm -d   --name rocketship-postgres   -e POSTGRES_PASSWORD=testpass   -e POSTGRES_DB=testdb   -p 5433:5432   postgres:13

# MySQL
docker run --rm -d   --name rocketship-mysql   -e MYSQL_ROOT_PASSWORD=testpass   -e MYSQL_DATABASE=testdb   -p 3306:3306   mysql:8.0
```

Update your test variables/DSNs to match the exposed ports (`postgres://postgres:testpass@localhost:5433/testdb`, `root:testpass@tcp(127.0.0.1:3306)/testdb`). Stop the containers when you are finished:

```bash
docker stop rocketship-postgres rocketship-mysql
```

## Running Tests with Browser Plugin

The Minikube workflow builds fresh engine/worker images that already contain the Python runtime and Playwright dependencies. After running `scripts/install-minikube.sh`, port-forward the engine and execute the suite:

```bash
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship run -af examples/browser-testing/rocketship.yaml
```

**Manual local mode:** when you are not using the cluster, install the browser dependencies once per machine:

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
