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
