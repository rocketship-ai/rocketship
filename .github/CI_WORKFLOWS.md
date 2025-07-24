# CI Workflows Structure

This document describes the reorganized CI workflow structure for the Rocketship project.

## Overview

The CI system has been split into focused, individual workflow files that all run in parallel on every PR. This provides comprehensive testing coverage while maintaining organization and easier debugging.

## Workflow Files

### 1. Build & Test (`build-test.yml`)

**Purpose**: Core Go compilation and Docker build testing

**Tests**:
- Multi-platform Go builds (darwin/linux/windows, amd64/arm64)
- Docker image builds (CLI, Engine, Worker)
- Go linting with golangci-lint
- Unit tests (`go test ./...`)

### 2. CLI Integration Tests (`cli-integration.yml`)

**Purpose**: End-to-end CLI functionality testing

**Tests**:
- CLI command functionality
- Database integration (PostgreSQL, MySQL)
- Browser plugin testing
- Example test suites
- Run persistence and management
- Error handling scenarios

**Requirements**:
- External API keys (Anthropic, OpenAI, Supabase)
- Test databases via Docker services
- Python dependencies for browser automation

### 3. Documentation Validation (`docs-validation.yml`)

**Purpose**: Ensure documentation stays in sync with code

**Tests**:
- Plugin reference documentation sync
- Schema validation
- Auto-generated docs verification

### 4. MCP Integration Tests (`mcp-integration.yml`)

**Purpose**: Model Context Protocol server testing

**Tests**:
- MCP server build and packaging
- NPM package functionality
- Integration with Rocketship CLI
- Package installation testing

### 5. Helm Chart Tests (`helm-chart-tests.yml`)

**Purpose**: Kubernetes deployment validation

**Tests**:
- Helm chart linting
- Template validation (minikube and production)
- Kind cluster deployment
- Chart upgrade/downgrade testing
- Documentation completeness

**Infrastructure**:
- Kind Kubernetes cluster
- NGINX Ingress Controller
- Test Docker images
- Helm chart dependencies

### 6. PR Check Orchestrator (`pr-check-orchestrator.yml`)

**Purpose**: Provides overview and coordination
**Triggers**: All PRs to main branch

**Function**:
- Explains the CI structure
- Provides status overview
- Serves as entry point for understanding CI

### 7. Legacy PR Check (`pr-check.yml`)

**Purpose**: Information and redirection
**Function**: Explains the new workflow organization

## Benefits of This Structure

### 🚀 Performance
- **Parallel execution**: All workflows run simultaneously on every PR
- **Comprehensive coverage**: No hidden dependencies missed by selective testing
- **Resource efficiency**: Better resource utilization through parallelization

### 🎯 Focus
- **Clear separation**: Each workflow has a single, clear purpose
- **Easier debugging**: Isolated failures are easier to diagnose
- **Comprehensive validation**: All components tested on every change

### 🔧 Maintenance
- **Modular design**: Easy to modify individual workflow aspects
- **Clear ownership**: Each workflow has well-defined scope
- **Better logs**: Smaller, focused log output per workflow

### 📊 Scalability
- **Add new workflows**: Easy to add specialized test suites
- **Simple triggers**: All workflows use the same trigger pattern
- **Resource allocation**: Each workflow can have appropriate resource limits

## Trigger Strategy

All workflows use the same simple trigger pattern to run on every PR:

```yaml
# Standard trigger configuration for all workflows
on:
  pull_request:
    branches: ["main"]
```

This ensures comprehensive testing coverage without missing any unexpected dependencies between components.

## Workflow Dependencies

While workflows are independent, they share some common patterns:

1. **Setup Steps**: Go installation, dependency management
2. **Security**: API key management for external services
3. **Infrastructure**: Database services, test environments
4. **Artifact Sharing**: Docker images, built binaries

## Monitoring Workflow Status

To check the status of all workflows:

1. Go to the **Actions** tab in GitHub
2. Each workflow will appear as a separate check
3. Green checkmarks indicate passing workflows
4. Red X marks indicate failing workflows
5. Yellow circles indicate workflows in progress

## Adding New Workflows

To add a new workflow:

1. Create a new `.yml` file in `.github/workflows/`
2. Use the standard trigger pattern (all PRs to main)
3. Add the workflow description to this document
4. Update the orchestrator workflow information if needed

## Standard Workflow Template

All workflows follow this basic structure:

```yaml
name: Workflow Name

on:
  pull_request:
    branches: ["main"]

jobs:
  job-name:
    name: Job Display Name
    runs-on: ubuntu-latest
    steps:
      # workflow steps here
```

This structure ensures comprehensive testing and maintains consistency across all CI workflows.