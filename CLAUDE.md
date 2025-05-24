# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

Rocketship is an open-source testing engine for E2E API testing that uses Temporal for durable execution. The system is built with Go and follows a plugin-based architecture.

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
make build          # Build CLI with embedded binaries
make install        # Install CLI to $GOPATH/bin
make dev-setup      # Set up development environment with git hooks
```

### Testing and Quality

```bash
make test           # Run all tests
make lint           # Run golangci-lint and workflowcheck
go test ./...       # Run tests for specific packages
```

### Embedded Binaries

The CLI embeds engine and worker binaries. Always run `make build-binaries` after modifying engine/worker code.

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

### Development Workflow

1. **Make code changes** to engine/worker/CLI
2. **Rebuild binaries**: `make build` (includes `make build-binaries`)
3. **Test with debug logging**: `ROCKETSHIP_LOG=DEBUG rocketship run -af examples/simple-http/rocketship.yaml`
4. **Run full test suite**: `make test`
5. **Check linting**: `make lint`

### Local Development Binary Usage

The system automatically uses local development binaries from `internal/embedded/bin/` when available, avoiding GitHub downloads. This makes iterative development faster.

### Common Development Tasks

```bash
# Quick test with debug output
ROCKETSHIP_LOG=DEBUG rocketship run -af examples/simple-http/rocketship.yaml

# Background server for iterative testing
ROCKETSHIP_LOG=DEBUG rocketship start server -lb
rocketship run test.yaml
rocketship stop server

# Validate YAML changes
rocketship validate test.yaml

# Run CI checks locally
make test && make lint
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

Current plugins: HTTP, delay, AWS (S3, SQS, DynamoDB)

## Running Tests

```bash
rocketship run -af test.yaml    # Auto-start local engine, run tests, auto-stop engine
rocketship start server -lb     # Start engine in locally, in the background
rocketship run test.yaml        # Run against existing engine
rocketship stop server          # Stop local background engine
```
