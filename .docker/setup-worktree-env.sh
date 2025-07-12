#!/bin/bash
# Setup script for isolated Docker environment in git worktrees
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

# Calculate unique ports for this instance with better separation
HASH=$(echo -n "${WORKTREE_NAME}" | cksum | cut -d' ' -f1)
# Use larger offsets and ensure no overlaps between different port ranges
PORT_OFFSET=$((HASH % 50))  # Smaller range to prevent conflicts
WORKTREE_ID=$((PORT_OFFSET * 10))  # Multiply by 10 for separation

# Temporal UI port (base: 8080, range: 8080-8500)
TEMPORAL_UI_PORT=$((8080 + WORKTREE_ID))

# Engine ports (base: 7700, range: 7700-8000) 
ENGINE_PORT=$((7700 + WORKTREE_ID))
ENGINE_METRICS_PORT=$((7701 + WORKTREE_ID))

# Test database ports with large separation
POSTGRES_TEST_PORT=$((5500 + WORKTREE_ID))  # Range: 5500-6000
MYSQL_TEST_PORT=$((3400 + WORKTREE_ID))     # Range: 3400-3900

# Create .env file with unique project name and all required variables
cat > .docker/.env.local << EOF
# Auto-generated environment file for worktree: ${WORKTREE_NAME}
COMPOSE_PROJECT_NAME=${PROJECT_NAME}

# Temporal environment variables (required for docker-compose)
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

# No override file needed - environment variables will be used directly

# Create the CLI wrapper script with strict isolation
cat > .docker/docker-rocketship-local.sh << EOF
#!/bin/bash
# Docker-based Rocketship CLI wrapper for this worktree
# Auto-generated - do not edit
# IMPORTANT: This ensures complete isolation between Claude agents

# Set values based on this worktree
WORKTREE_NAME="${WORKTREE_NAME}"
PROJECT_NAME="${PROJECT_NAME}"
NETWORK="${PROJECT_NAME}-network"
ENGINE_HOST="${PROJECT_NAME}-engine:7700"
IMAGE="${PROJECT_NAME}-cli:latest"

# Unique session ID for this agent (prevents cross-contamination)
SESSION_ID="${WORKTREE_NAME}-\$(date +%s)-\$(head -c 8 /dev/urandom | base64 | tr -d '=+/' | cut -c1-8)"

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
    echo "Please run the start-services.sh script first"
    exit 1
fi

# Check if the engine is running
if ! docker ps | grep -q "\${PROJECT_NAME}-engine"; then
    echo "Error: Rocketship engine is not running."
    echo "Please run the start-services.sh script first"
    exit 1
fi

# Build the image if it doesn't exist
if ! docker image inspect \$IMAGE > /dev/null 2>&1; then
    echo "Building Rocketship CLI image..."
    docker build -f "\$(dirname "\$0")/Dockerfile.cli" -t \$IMAGE "\$(dirname "\$0")/.." || exit 1
fi

# Prevent manual engine specification to ensure isolation
FILTERED_ARGS=()
for arg in "\$@"; do
    if [[ "\$arg" == "-e" ]] || [[ "\$arg" == "--engine" ]]; then
        echo "Warning: Engine flag ignored. Using isolated engine: \$ENGINE_HOST"
        continue
    fi
    # Skip the next argument if it's an engine address
    if [[ "\$prev_arg" == "-e" ]] || [[ "\$prev_arg" == "--engine" ]]; then
        prev_arg="\$arg"
        continue
    fi
    FILTERED_ARGS+=("\$arg")
    prev_arg="\$arg"
done

# Run the command with strict isolation
echo "🔒 Using isolated engine: \$ENGINE_HOST"
echo "🆔 Session ID: \$SESSION_ID"

# If no arguments provided, show help
if [ \${#FILTERED_ARGS[@]} -eq 0 ]; then
    docker run --rm \\
        --network \$NETWORK \\
        -v "\$(pwd)":/workspace \\
        -w /workspace \\
        \$IMAGE --help
else
    # For commands that need engine connection, add -e flag
    case "\${FILTERED_ARGS[0]}" in
        run|list|get)
            docker run --rm \\
                --network \$NETWORK \\
                -v "\$(pwd)":/workspace \\
                -w /workspace \\
                -e "ROCKETSHIP_SESSION_ID=\$SESSION_ID" \\
                \$IMAGE "\${FILTERED_ARGS[@]}" -e \$ENGINE_HOST
            ;;
        *)
            # For other commands (validate, help, version), don't add engine flag
            docker run --rm \\
                --network \$NETWORK \\
                -v "\$(pwd)":/workspace \\
                -w /workspace \\
                \$IMAGE "\${FILTERED_ARGS[@]}"
            ;;
    esac
fi
EOF

chmod +x .docker/docker-rocketship-local.sh

# Create the start script that sources both env files
cat > .docker/start-services.sh << EOF
#!/bin/bash
# Start services for this worktree
echo "Starting services for ${PROJECT_NAME}..."

# Get the directory where this script is located
SCRIPT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"

# Load both env files to ensure all variables are available
set -a
if [ -f "\$SCRIPT_DIR/.env" ]; then
    source "\$SCRIPT_DIR/.env"
fi
if [ -f "\$SCRIPT_DIR/.env.local" ]; then
    source "\$SCRIPT_DIR/.env.local"
fi
set +a

# Start services with explicit paths to avoid cd issues
docker-compose -f "\$SCRIPT_DIR/docker-compose.yaml" -p ${PROJECT_NAME} up -d
echo "Services started! Temporal UI: http://localhost:${TEMPORAL_UI_PORT}"
echo "Engine available at: localhost:${ENGINE_PORT}"
EOF

chmod +x .docker/start-services.sh

# Create a stop script
cat > .docker/stop-services.sh << EOF
#!/bin/bash
# Stop services for this worktree
echo "Stopping services for ${PROJECT_NAME}..."

# Get the directory where this script is located
SCRIPT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"

# Stop services with explicit paths
docker-compose -f "\$SCRIPT_DIR/docker-compose.yaml" -p ${PROJECT_NAME} down -v
echo "Services stopped and cleaned up."
EOF

chmod +x .docker/stop-services.sh

# Show the generated configuration
echo -e "\n${GREEN}Generated configuration:${NC}"
echo "- Project name: ${PROJECT_NAME}"
echo "- Network name: temporal-network"
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