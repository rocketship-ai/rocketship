# Quick Start Guide

Get started with Rocketship in minutes! This guide will help you install Rocketship and run your first test.

!!! tip "For Coding Agents"
    If you're a coding agent (Claude Code, Cursor, Windsurf, etc.), copy and paste the [ROCKETSHIP_AGENT_QUICKSTART.md](https://raw.githubusercontent.com/rocketship-ai/rocketship/main/ROCKETSHIP_AGENT_QUICKSTART.md) file into your context window for a comprehensive reference.

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
