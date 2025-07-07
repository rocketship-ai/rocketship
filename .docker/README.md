# Rocketship Docker Setup

This directory contains the Docker configuration for running Rocketship in a fully containerized environment. This setup is ideal for:
- Isolated development environments
- Running multiple Rocketship instances
- CI/CD pipelines
- Testing without local installation

## Architecture

The Docker setup includes:
- **Temporal Stack**: PostgreSQL, Elasticsearch, Temporal Server, UI, and Admin Tools
- **Rocketship Engine**: gRPC server that orchestrates test execution
- **Rocketship Worker**: Executes test workflows using plugins
- **Rocketship CLI**: Command-line interface in a container
- **Test Databases**: PostgreSQL and MySQL for SQL plugin testing

## Quick Start

### 1. Start All Services

```bash
cd .docker
docker-compose up -d
```

This will start:
- Temporal server on port 7233
- Temporal UI on http://localhost:8080
- Rocketship Engine on port 7700
- Test PostgreSQL on port 5433
- Test MySQL on port 3307

### 2. Build the CLI Container

```bash
docker build -f Dockerfile.cli -t rocketship-cli:latest ..
```

### 3. Run Tests Using the CLI Container

```bash
# Using the convenience script
./docker-rocketship.sh run -f ../examples/simple-http/rocketship.yaml

# Or directly with docker
docker run --rm \
  --network temporal-network \
  -v $(pwd)/..:/workspace \
  -w /workspace \
  rocketship-cli:latest run -e temporal-engine-1:7700 -f examples/simple-http/rocketship.yaml
```

## Using the Docker Rocketship CLI

The `docker-rocketship.sh` script provides a convenient wrapper:

```bash
# Validate test files
./docker-rocketship.sh validate test.yaml

# Run tests
./docker-rocketship.sh run -f test.yaml

# List test runs
./docker-rocketship.sh list runs

# Get run details
./docker-rocketship.sh get run <run-id>
```

## Running SQL Tests

The setup includes test databases for SQL plugin testing:

```bash
# PostgreSQL is available at localhost:5433
# MySQL is available at localhost:3307

# Run SQL tests
./docker-rocketship.sh run -f ../examples/sql-testing/rocketship.yaml
```

## Environment Variables

You can pass environment variables to the CLI container:

```bash
docker run --rm \
  --network temporal-network \
  -v $(pwd):/workspace \
  -e API_KEY=your-key \
  -e API_URL=https://api.example.com \
  rocketship-cli:latest run -e temporal-engine-1:7700 -f test.yaml
```

## Monitoring

- **Temporal UI**: http://localhost:8080 - Monitor workflows and workers
- **Engine Logs**: `docker logs temporal-engine-1`
- **Worker Logs**: `docker logs temporal-worker-1`

## Stopping Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (clean slate)
docker-compose down -v
```

## Troubleshooting

### Container can't connect to engine
Ensure you're using the correct network and hostname:
- Network: `temporal-network`
- Engine host: `temporal-engine-1:7700`

### Permission issues with mounted volumes
The CLI container runs as user `rocketship` (UID 1001). Ensure your test files are readable.

### Build failures
Check that you have the latest Go version that matches the project (currently Go 1.24).

## Advanced Usage

### Running Multiple Instances

To run multiple isolated Rocketship instances:

```bash
# Create a new compose file with different project name
docker-compose -p instance1 up -d
docker-compose -p instance2 up -d
```

### Custom Configuration

Modify `docker-compose.yaml` to:
- Change port mappings
- Add environment variables
- Mount configuration files
- Scale workers

### Browser Testing Support

To add browser testing support to the CLI container, build with the browser-enabled Dockerfile:

```bash
# TODO: Create Dockerfile.cli.browser with Python and Playwright
docker build -f Dockerfile.cli.browser -t rocketship-cli:browser ..
```

## Development Workflow

1. Make changes to Rocketship code
2. Rebuild the affected containers:
   ```bash
   docker-compose build engine worker
   docker build -f Dockerfile.cli -t rocketship-cli:latest ..
   ```
3. Restart services:
   ```bash
   docker-compose restart engine worker
   ```
4. Test your changes using the CLI container

## Security Considerations

- All services run as non-root users
- Containers use minimal Alpine Linux images
- Network isolation between services
- No unnecessary ports exposed

## Next Steps

- Add browser testing dependencies to CLI container
- Create development vs production compose profiles
- Add health check endpoints
- Implement log aggregation