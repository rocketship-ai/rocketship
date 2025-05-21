# Quick Start Guide

Get started with Rocketship in minutes! This guide will help you install Rocketship and run your first test.

## Installation

First, install Temporal (required for the local engine):

```bash
brew install temporal
```

Then install the Rocketship CLI:

```bash
# for arm64 macos
curl -Lo /usr/local/bin/rocketship https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64
chmod +x /usr/local/bin/rocketship
```

For detailed installation instructions for other platforms and optional aliases, see the [Installation Guide](installation.md).

## Your First Test

1. Create a test file:

```bash
cat > simple-test.yaml << 'EOF'
name: "Simple Test Suite"
description: "A simple test suite!"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Check API status"
        plugin: "http"
        config:
          method: "GET"
          url: "https://httpbin.org/status/200"
        assertions:
          - type: "status_code"
            expected: 200
EOF
```

2. Run the test:

```bash
rocketship run -af simple-test.yaml
```

The `-a` flag tells Rocketship to automatically start and stop the local server, and `-f` specifies the test file to run.

## Next Steps

- Learn about [test specifications](test-specs.md)
- Explore the [CLI reference](reference/rocketship.md)
- Check out [example tests](examples.md)
