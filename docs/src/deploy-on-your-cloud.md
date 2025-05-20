# Deploying Rocketship on Your Cloud

Rocketship can be deployed in your cloud environment to run tests at scale, persist test history, and leverage all of Temporal's durable execution features. This guide covers the different deployment options and considerations.

## Architecture Overview

Rocketship consists of three main components:

1. **Engine**: The central service that receives test requests and coordinates test execution with Temporal
2. **Worker**: Executes test steps and reports results back to Temporal
3. **Temporal**: Handles workflow orchestration and state management

## Deployment Options

### Docker Compose

The simplest way to deploy Rocketship is using Docker Compose. This is ideal for:

- Development environments
- Small-scale deployments
- Testing and evaluation

See the [Docker Compose Setup](#docker-compose-setup) section for details.

### Kubernetes

For production deployments, we recommend using Kubernetes. This provides:

- High availability
- Automatic scaling
- Better resource management
- Production-grade monitoring

See the [Kubernetes Deployment](deploy-on-kubernetes.md) guide for details.

## Docker Compose Setup

Clone the Rocketship repository, navigate to the `.docker` directory, and run the following command:

```bash
docker compose up -d
```

Verify the deployment:

```bash
# Check service status
docker-compose ps

# Check engine logs
docker-compose logs engine

# Check worker logs
docker-compose logs worker
```

Run a test:

```bash
rocketship run -f your-test.yaml -e localhost:7700
```

## Next Steps

- [Deploy on Kubernetes](./deploy-on-kubernetes.md) for production-grade deployment
- [Command Reference](./reference/rocketship.md) for CLI usage
- [Examples](./examples.md) for test suite examples
