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

1. Create a `docker-compose.yaml` file:

```yaml
services:
  temporal:
    image: temporalio/auto-setup:1.27.2
    environment:
      - DB=postgres12
      - DB_PORT=5432
      - POSTGRES_USER=temporal
      - POSTGRES_PWD=temporal
      - POSTGRES_SEEDS=postgresql
      - DYNAMIC_CONFIG_FILE_PATH=config/dynamicconfig/development-sql.yaml
      - ENABLE_ES=true
      - ES_SEEDS=elasticsearch
      - ES_VERSION=v7
    ports:
      - "7233:7233"

  postgresql:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: temporal
      POSTGRES_USER: temporal
    volumes:
      - postgresql-data:/var/lib/postgresql/data

  elasticsearch:
    image: elasticsearch:7.17.27
    environment:
      - discovery.type=single-node
      - ES_JAVA_OPTS=-Xms256m -Xmx256m
      - xpack.security.enabled=false

  engine:
    image: rocketshipai/rocketship-engine:latest
    depends_on:
      - temporal
    environment:
      - TEMPORAL_HOST=temporal:7233
    ports:
      - "7700:7700"
      - "7701:7701"

  worker:
    image: rocketshipai/rocketship-worker:latest
    depends_on:
      - temporal
      - engine
    environment:
      - TEMPORAL_HOST=temporal:7233

volumes:
  postgresql-data:
```

2. Start the services:

```bash
docker-compose up -d
```

3. Verify the deployment:

```bash
# Check service status
docker-compose ps

# Check engine logs
docker-compose logs engine

# Check worker logs
docker-compose logs worker
```

4. Run a test:

```bash
rocketship run -f your-test.yaml -e localhost:7700
```

## Next Steps

- [Deploy on Kubernetes](./deploy-on-kubernetes.md) for production-grade deployment
- [Command Reference](./reference/rocketship.md) for CLI usage
- [Examples](./examples.md) for test suite examples
