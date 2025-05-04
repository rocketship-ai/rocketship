# Rocketship

### 🚀 **Rocketship** – AI-Native End-to-End API Testing

**Rocketship** is an open-source, AI-driven platform reimagining end-to-end API testing. Designed to reduce manual test creation overhead, Rocketship uses intelligent automation to streamline test creation and maintenance, enabling developers to build robust software faster.

### 🎯 Vision & Manifesto

We envision a world where software testing is automated intelligently and continuously evolves alongside your application code. Our mission is to empower developers to focus on innovation rather than maintenance, ensuring high-quality software releases at rapid speed.

## Components

- **Engine**: Parse YAML → WF spec; Start/track WF
- **Worker**: Poll task-queues, run interpreter, invoke Activities
- **CLI**: Developer UX; talks to Engine; may launch Compose
- **LocalStack**: Mock S3/SQS/DynamoDB

## Getting Started

```bash
# Build the project
make build

# Start the local runtime
rocketship start

# Run a test
rocketship run --file examples/order-workflow/rocketship.yaml
```

## Documentation

For detailed documentation, see the docs directory.

## Development Setup

### Pre-commit Hooks

To ensure code quality, we use pre-commit hooks to run linting and tests before each commit. To set this up:

1. Run the setup script:

   ```bash
   ./for-maintainers/setup-hooks.sh
   ```

   Or, if you prefer to set it up manually:

2. Create the pre-commit hook:

   ```bash
   echo '#!/bin/sh
   echo "Running lint and tests..."
   if ! make lint test; then
       echo "❌ Lint or tests failed. Commit aborted."
       exit 1
   fi
   echo "✅ Lint and tests passed!"
   exit 0' > .git/hooks/pre-commit
   ```

3. Make it executable:
   ```bash
   chmod +x .git/hooks/pre-commit
   ```

Now, every time you try to commit, the hook will run `make lint test` and prevent the commit if either fails.
