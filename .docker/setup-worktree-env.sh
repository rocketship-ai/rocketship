#!/bin/bash
# Fixed setup script for isolated Docker environment in git worktrees
# Each Claude Code instance should run this to get their own isolated containers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the current directory name (will be unique for each worktree)
WORKTREE_NAME=$(basename $(pwd))
PROJECT_NAME="rocketship-${WORKTREE_NAME}"

echo -e "${GREEN}Setting up isolated Docker environment for worktree: ${WORKTREE_NAME}${NC}"
echo -e "Project name: ${PROJECT_NAME}"

# Check if we're in a git worktree
if [ ! -d .git ] && [ ! -f .git ]; then
    echo -e "${RED}Error: This doesn't appear to be a git repository or worktree${NC}"
    echo "Please run this from the root of your git worktree"
    exit 1
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running. Please start Docker first.${NC}"
    exit 1
fi

# Clean up any existing containers/networks for this project
echo -e "${YELLOW}Cleaning up any existing containers for ${PROJECT_NAME}...${NC}"
docker-compose -f .docker/docker-compose.yaml -p ${PROJECT_NAME} down -v 2>/dev/null || true
docker network rm ${PROJECT_NAME}-network 2>/dev/null || true

# Calculate unique ports for this instance
HASH=$(echo -n "${WORKTREE_NAME}" | cksum | cut -d' ' -f1)
PORT_OFFSET=$((HASH % 100))

# Temporal UI port (base: 8080)
TEMPORAL_UI_PORT=$((8080 + PORT_OFFSET))

# Engine ports (base: 7700, 7701)
ENGINE_PORT=$((7700 + PORT_OFFSET))
ENGINE_METRICS_PORT=$((7701 + PORT_OFFSET))

# Test database ports
POSTGRES_TEST_PORT=$((5433 + PORT_OFFSET))
MYSQL_TEST_PORT=$((3307 + PORT_OFFSET))

# Create .env file with unique project name and Temporal versions
cat > .docker/.env.local << EOF
# Auto-generated environment file for worktree: ${WORKTREE_NAME}
COMPOSE_PROJECT_NAME=${PROJECT_NAME}

# Temporal environment variables (from original .env)
CASSANDRA_VERSION=3.11.9
ELASTICSEARCH_VERSION=7.17.27
MYSQL_VERSION=8
TEMPORAL_VERSION=1.27.2
TEMPORAL_ADMINTOOLS_VERSION=1.27.2-tctl-1.18.2-cli-1.3.0
TEMPORAL_UI_VERSION=2.34.0
POSTGRESQL_VERSION=16
POSTGRES_PASSWORD=temporal
POSTGRES_USER=temporal
POSTGRES_DEFAULT_PORT=5432
OPENSEARCH_VERSION=2.5.0

# Unique ports for this instance
TEMPORAL_UI_PORT=${TEMPORAL_UI_PORT}
ENGINE_PORT=${ENGINE_PORT}
ENGINE_METRICS_PORT=${ENGINE_METRICS_PORT}
POSTGRES_TEST_PORT=${POSTGRES_TEST_PORT}
MYSQL_TEST_PORT=${MYSQL_TEST_PORT}
EOF

# Create docker-compose override file
cat > .docker/docker-compose.override.yml << EOF
# Auto-generated override for worktree: ${WORKTREE_NAME}
# This file provides unique ports and container names

services:
  temporal-ui:
    container_name: ${PROJECT_NAME}-temporal-ui
    ports:
      - "${TEMPORAL_UI_PORT}:8080"

  engine:
    container_name: ${PROJECT_NAME}-engine
    ports:
      - "${ENGINE_PORT}:7700"
      - "${ENGINE_METRICS_PORT}:7701"

  worker:
    container_name: ${PROJECT_NAME}-worker

  postgres-test:
    container_name: ${PROJECT_NAME}-postgres-test
    ports:
      - "${POSTGRES_TEST_PORT}:5432"

  mysql-test:
    container_name: ${PROJECT_NAME}-mysql-test
    ports:
      - "${MYSQL_TEST_PORT}:3306"

  temporal:
    container_name: ${PROJECT_NAME}-temporal

  temporal-postgresql:
    container_name: ${PROJECT_NAME}-temporal-postgresql

  temporal-elasticsearch:
    container_name: ${PROJECT_NAME}-temporal-elasticsearch

  temporal-admin-tools:
    container_name: ${PROJECT_NAME}-temporal-admin-tools

networks:
  temporal-network:
    name: temporal-network
EOF

# Update the docker-rocketship.sh script to use the correct values
cat > .docker/docker-rocketship-local.sh << EOF
#!/bin/bash
# Docker-based Rocketship CLI wrapper for this worktree
# Auto-generated - do not edit

# Set values based on this worktree
WORKTREE_NAME="${WORKTREE_NAME}"
PROJECT_NAME="${PROJECT_NAME}"
NETWORK="temporal-network"
ENGINE_HOST="${PROJECT_NAME}-engine:7700"
IMAGE="${PROJECT_NAME}-cli:latest"

# Function to show usage
usage() {
    echo "Usage: \$0 [rocketship-command] [arguments...]"
    echo ""
    echo "This script runs Rocketship CLI commands in a Docker container."
    echo "It automatically connects to this worktree's dockerized engine."
    echo ""
    echo "Worktree: \${WORKTREE_NAME}"
    echo "Project: \${PROJECT_NAME}"
    echo ""
    exit 1
}

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker first."
    exit 1
fi

# Check if the network exists
if ! docker network inspect \$NETWORK > /dev/null 2>&1; then
    echo "Error: Docker network '\$NETWORK' not found."
    echo "Please run: cd .docker && docker-compose -p \${PROJECT_NAME} up -d"
    exit 1
fi

# Check if the engine is running
if ! docker ps | grep -q "\${PROJECT_NAME}-engine"; then
    echo "Error: Rocketship engine is not running."
    echo "Please run: cd .docker && docker-compose -p \${PROJECT_NAME} up -d"
    exit 1
fi

# Build the image if it doesn't exist
if ! docker image inspect \$IMAGE > /dev/null 2>&1; then
    echo "Building Rocketship CLI image..."
    docker build -f "\$(dirname "\$0")/Dockerfile.cli" -t \$IMAGE "\$(dirname "\$0")/.." || exit 1
fi

# Run the command
docker run --rm \\
    --network \$NETWORK \\
    -v "\$(pwd)":/workspace \\
    -w /workspace \\
    \$IMAGE "\$@" -e \$ENGINE_HOST
EOF

chmod +x .docker/docker-rocketship-local.sh

# Create a simple start script
cat > .docker/start-services.sh << EOF
#!/bin/bash
# Start services for this worktree
cd "\$(dirname "\$0")"
echo "Starting services for ${PROJECT_NAME}..."
docker-compose --env-file .env.local -p ${PROJECT_NAME} up -d
echo "Services started! Temporal UI: http://localhost:${TEMPORAL_UI_PORT}"
EOF

chmod +x .docker/start-services.sh

# Create a stop script
cat > .docker/stop-services.sh << EOF
#!/bin/bash
# Stop services for this worktree
cd "\$(dirname "\$0")"
echo "Stopping services for ${PROJECT_NAME}..."
docker-compose -p ${PROJECT_NAME} down -v
echo "Services stopped and cleaned up."
EOF

chmod +x .docker/stop-services.sh

# Show the generated configuration
echo -e "\n${GREEN}Generated configuration:${NC}"
echo "- Project name: ${PROJECT_NAME}"
echo "- Network name: ${PROJECT_NAME}-network"
echo "- Temporal UI: http://localhost:${TEMPORAL_UI_PORT}"
echo "- Engine port: ${ENGINE_PORT}"
echo "- PostgreSQL test: localhost:${POSTGRES_TEST_PORT}"
echo "- MySQL test: localhost:${MYSQL_TEST_PORT}"

echo -e "\n${GREEN}Next steps:${NC}"
echo "1. Start the services:"
echo "   .docker/start-services.sh"
echo ""
echo "2. Use the CLI wrapper:"
echo "   .docker/docker-rocketship-local.sh run -f test.yaml"
echo ""
echo "3. View Temporal UI:"
echo "   open http://localhost:${TEMPORAL_UI_PORT}"
echo ""
echo "4. Stop services when done:"
echo "   .docker/stop-services.sh"
echo ""
echo -e "${YELLOW}Note: Each worktree will have its own isolated set of containers and ports.${NC}"