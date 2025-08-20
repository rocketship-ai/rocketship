# Test Run Management

Rocketship provides comprehensive test run tracking and management capabilities, allowing you to organize, filter, and analyze your test execution history.

## Overview

Every test run in Rocketship is automatically tracked with contextual metadata including:

- **Project ID**: Organize runs by project or application
- **Source**: Track where the run originated (CLI, CI/CD, scheduled)
- **Branch**: Git branch information for version tracking
- **Trigger**: How the run was initiated (manual, webhook, schedule)
- **Metadata**: Custom key-value pairs for additional context

## Running Tests with Context

### Basic Context Flags

When running tests, you can provide context information that will be stored with the run:

```bash
# Run with full context
rocketship run -f test.yaml \
  --project-id "my-app" \
  --source "cli-local" \
  --branch "feature/new-api" \
  --trigger "manual"
```

### Auto-Detection

If not specified, Rocketship automatically detects:

- **Project ID**: Uses "default" if not provided
- **Source**: Detects CI environment or defaults to "cli-local"
- **Branch**: Uses `git branch --show-current`
- **Commit SHA**: Uses `git rev-parse HEAD`
- **Trigger**: Infers based on source (webhook for CI, manual for local)

### Custom Metadata

Add custom metadata for additional context:

```bash
rocketship run -f test.yaml \
  --project-id "my-app" \
  --metadata "env=staging" \
  --metadata "team=backend" \
  --metadata "version=1.2.3"
```

### Auto Run Mode

When using the `--auto` flag, Rocketship automatically displays recent test runs after your tests complete:

```bash
# Run tests with auto mode
rocketship run --auto -f test.yaml

# After tests complete, you'll see:
# 1. Final test summary
# 2. Recent test runs table (all runs)
```

This provides immediate visibility into your test history without needing to run a separate `list` command.

## Listing Test Runs

The `rocketship list` command provides powerful filtering and sorting capabilities:

### Basic Listing

```bash
# List all recent runs (default: 20 most recent)
rocketship list --engine localhost:7700

# Limit results
rocketship list --engine localhost:7700 --limit 50
```

### Filtering

Filter runs by various criteria:

```bash
# By project
rocketship list --engine localhost:7700 --project-id "my-app"

# By status
rocketship list --engine localhost:7700 --status FAILED
rocketship list --engine localhost:7700 --status PASSED

# By source
rocketship list --engine localhost:7700 --source "ci-branch"

# By git branch
rocketship list --engine localhost:7700 --branch "main"

# By schedule name (for scheduled runs)
rocketship list --engine localhost:7700 --schedule-name "nightly-tests"

# Combine filters
rocketship list --engine localhost:7700 --project-id "my-app" --status FAILED --branch "main"
```

### Sorting

Control the order of results:

```bash
# Sort by start time (default: newest first)
rocketship list --engine localhost:7700 --order-by started_at

# Sort by duration (longest first)
rocketship list --engine localhost:7700 --order-by duration

# Sort in ascending order
rocketship list --engine localhost:7700 --order-by duration --ascending
```

## Getting Run Details

Use `rocketship get` to view detailed information about a specific run:

```bash
# Get run details (accepts truncated run IDs)
rocketship get abc123def456 --engine localhost:7700

# Full run details include:
# - Run metadata (ID, suite name, status, timing)
# - Context information (project, source, branch, etc.)
# - Individual test results
# - Custom metadata
```

### Example Output

```
Test Run Details
================

Run ID:      abc123def456789
Suite Name:  My Test Suite
Status:      ✓ PASSED
Started:     2025-06-25T10:30:00Z
Ended:       2025-06-25T10:32:15Z
Duration:    2m15s

Context:
  Project ID:    my-app
  Source:        ci-branch
  Branch:        feature/new-api
  Commit:        a1b2c3d4
  Trigger:       webhook
  Metadata:
    env: staging
    team: backend

Tests (3):
  #  NAME           STATUS    DURATION  ERROR
  -  ----           ------    --------  -----
  1  Health Check   ✓ PASSED  1.2s      
  2  User Login     ✓ PASSED  0.8s      
  3  Data Fetch     ✓ PASSED  0.3s
```

## CI/CD Integration

### Branch-based CI

For branch-based CI workflows:

```bash
# In your CI pipeline
rocketship run -f tests/api.yaml \
  --project-id "$PROJECT_NAME" \
  --source "ci-branch" \
  --branch "$CI_BRANCH" \
  --commit "$CI_COMMIT_SHA" \
  --trigger "webhook" \
  --metadata "build_id=$BUILD_ID"
```

### Scheduled Runs

For scheduled test runs:

```bash
# In your scheduler (cron, GitHub Actions, etc.)
rocketship run -f tests/nightly.yaml \
  --project-id "my-app" \
  --source "scheduled" \
  --schedule-name "nightly-tests" \
  --trigger "schedule" \
  --metadata "schedule_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

## Advanced Usage

### Server Management

When running multiple tests or in CI environments, you can manage the server separately:

```bash
# Start server in background
rocketship start server --local --background

# Run tests against existing server
rocketship run -f test1.yaml --engine localhost:7700 --project-id "my-app"
rocketship run -f test2.yaml --engine localhost:7700 --project-id "my-app"

# List results
rocketship list --engine localhost:7700 --project-id "my-app"

# Stop server
rocketship stop server
```

### Output Formats

Future versions will support additional output formats:

```bash
# JSON output (planned)
rocketship list --format json

# YAML output (planned)
rocketship get abc123 --engine localhost:7700 --format yaml
```

## Best Practices

1. **Consistent Project IDs**: Use consistent project identifiers across your organization
2. **Meaningful Metadata**: Add relevant context like environment, version, or team information
3. **Filter Effectively**: Use filters to quickly find relevant test runs
4. **Monitor Failures**: Regularly check for failed runs with `rocketship list --status FAILED`
5. **CI Integration**: Include context flags in your CI/CD pipelines for better traceability

## Troubleshooting

### Common Issues

- **Run not found**: Run IDs can be truncated (12 characters minimum)
- **No runs listed**: Check your filter criteria or server connection
- **Missing context**: Context is auto-detected when possible but can be explicitly set

### Debug Information

Use debug logging to troubleshoot:

```bash
ROCKETSHIP_LOG=DEBUG rocketship list --engine localhost:7700
ROCKETSHIP_LOG=DEBUG rocketship get abc123 --engine localhost:7700
```