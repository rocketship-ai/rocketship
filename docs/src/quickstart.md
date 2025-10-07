# Quick Start Guide

Get started with Rocketship in minutes! This guide will help you install Rocketship and run your first test.

## Installation

First, install Temporal (required for the local engine):

```bash
brew install temporal
```

Then install the Rocketship CLI:

**macOS (recommended via Homebrew):**
```bash
brew tap rocketship-ai/tap
brew install rocketship
```

**Linux and macOS (portable installer):**
```bash
curl -fsSL https://raw.githubusercontent.com/rocketship-ai/rocketship/main/scripts/install.sh | bash
```

For detailed installation instructions including Windows, Docker, and optional aliases, see the [Installation Guide](installation.md).

## Your First Test

Create a test file:

```bash
cat > simple-test.yaml << 'EOF'
name: "Simple Test Suite"
description: "A simple test suite!"
tests:
  - name: "API Health Check"
    steps:
      - name: "Check API status"
        plugin: "http"
        config:
          method: "GET"
          url: "https://tryme.rocketship.sh/status/200"
        assertions:
          - type: "status_code"
            expected: 200
EOF
```

Run the test:

```bash
rocketship run -af simple-test.yaml
```

The `-a` flag tells Rocketship to automatically start and stop the local server, and `-f` specifies the test file to run.

## Test Run Management

Rocketship automatically tracks your test runs with context information, making it easy to organize and find results.

### Authenticating Against Remote Engines

If you're connecting to a remote Rocketship deployment that requires OIDC, sign in once with the device-flow helper:

```bash
rocketship login
# follow the printed URL to approve the device
```

The CLI stores short-lived tokens securely and will refresh them automatically when they expire. Run `rocketship status` to confirm the current profile is authenticated or `rocketship logout` to clear credentials.

### Adding Context to Your Runs

You can add context to your test runs for better organization:

```bash
# Run with project context
rocketship run -af simple-test.yaml \
  --project-id "my-app" \
  --source "cli-local" \
  --branch "main" \
  --trigger "manual"

# Add custom metadata
rocketship run -af simple-test.yaml \
  --project-id "my-app" \
  --metadata "env=staging" \
  --metadata "team=backend"
```

### Viewing Test History

List your recent test runs:

```bash
# List all recent runs
rocketship list --engine localhost:7700

# Filter by project
rocketship list --engine localhost:7700 --project-id "my-app"

# Filter by status
rocketship list --engine localhost:7700 --status FAILED

# Get detailed information about a specific run
rocketship get <run-id> --engine localhost:7700
```

## Next Steps

- Explore the [CLI reference](reference/rocketship.md)
- Check out [example tests](examples.md)
- Learn about [run management](reference/rocketship_list.md)
