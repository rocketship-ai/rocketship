# Quick Start Guide

Get started with Rocketship in minutes! This guide will help you install Rocketship and run your first test.

**What you'll learn:**
- How to install Rocketship on your computer
- How to write a simple test
- How to run your test and see the results

!!! tip "For Coding Agents"
If you're a coding agent (Claude Code, Cursor, Windsurf, etc.), copy and paste the [ROCKETSHIP_QUICKSTART.md](https://raw.githubusercontent.com/rocketship-ai/rocketship/main/ROCKETSHIP_QUICKSTART.md) file into your context window for a comprehensive reference.

## Installation

### Step 1: Install Temporal (if running locally)

Temporal helps Rocketship manage long-running tests. You only need this if you'll run tests on your own computer:

```bash
brew install temporal
```

**Note:** If you're using a cloud-hosted Rocketship, you can skip this step.

### Step 2: Install Rocketship

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

Let's create a simple test that checks if a web service is responding. This test will:
1. Send a request to an API endpoint
2. Check if it returns the expected status code (200 means success)
3. Show you the results

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

**What this does:**
- `-a` (auto): Automatically starts and stops the testing engine for you
- `-f` (file): Tells Rocketship which test file to run

You should see output showing the test running and whether it passed or failed!

## Running Tests Against a Remote Engine

If your team has Rocketship running on a shared server (like in the cloud), you can connect to it using **profiles**. A profile is like a saved connection that remembers where your engine is located.

```bash
# Create a profile pointing to your engine
rocketship profile create production https://rocketship.company.com

# Authenticate (OIDC device flow)
rocketship login --profile production

# Use the profile
rocketship profile use production

# Run tests
rocketship run -f simple-test.yaml
```

For local Minikube deployments, port-forward the engine and create a profile:

```bash
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube
rocketship run -f simple-test.yaml
```

See [Deploy On Your Cloud](deploy-on-your-cloud.md) for production deployment options.
