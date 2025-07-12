#!/bin/bash
# Multi-Stack Docker Rocketship CLI wrapper
# This script runs the Rocketship CLI in a container, connecting to a specified stack

# Function to show usage
usage() {
    echo "Usage: $0 --stack <stack1|stack2> [rocketship-command] [arguments...]"
    echo ""
    echo "This script runs Rocketship CLI commands in a Docker container."
    echo "It connects to the specified isolated Rocketship stack environment."
    echo ""
    echo "Options:"
    echo "  --stack <name>    Specify which stack to connect to (stack1 or stack2)"
    echo ""
    echo "Examples:"
    echo "  $0 --stack stack1 validate test.yaml"
    echo "  $0 --stack stack2 run -f test.yaml"
    echo "  $0 --stack stack1 list runs"
    echo "  $0 --stack stack2 get run <run-id>"
    echo ""
    echo "Available Stacks:"
    echo "  stack1: Primary development stack (ports 7xxx, 8xxx, 9xxx)"
    echo "  stack2: Secondary development stack (ports 8xxx, 9xxx)"
    echo ""
    echo "Note: Files should be in the current directory or subdirectories."
    exit 1
}

# Parse stack argument
STACK=""
if [[ "$1" == "--stack" ]]; then
    STACK="$2"
    shift 2
else
    echo "Error: --stack parameter is required"
    usage
fi

# Validate stack parameter
if [[ "$STACK" != "stack1" && "$STACK" != "stack2" ]]; then
    echo "Error: Invalid stack '$STACK'. Must be 'stack1' or 'stack2'"
    usage
fi

# Set stack-specific values
if [[ "$STACK" == "stack1" ]]; then
    ENV_FILE=".env.stack1"
    NETWORK="rocketship-stack1-network"
    ENGINE_HOST="rocketship-stack1-engine-1:7700"
    IMAGE="rocketship-cli:latest"
elif [[ "$STACK" == "stack2" ]]; then
    ENV_FILE=".env.stack2"
    NETWORK="rocketship-stack2-network"
    ENGINE_HOST="rocketship-stack2-engine-1:7700"
    IMAGE="rocketship-cli:latest"
fi

# Get script directory to find .env files
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker first."
    exit 1
fi

# Check if the environment file exists
if [[ ! -f "$SCRIPT_DIR/$ENV_FILE" ]]; then
    echo "Error: Environment file '$ENV_FILE' not found in $SCRIPT_DIR"
    exit 1
fi

# Check if the network exists
if ! docker network inspect $NETWORK > /dev/null 2>&1; then
    echo "Error: Docker network '$NETWORK' not found."
    echo "Please start the $STACK environment first:"
    echo "  docker-compose --env-file $ENV_FILE up -d"
    exit 1
fi

# Check if the engine is running
if ! docker ps | grep -q "${STACK}-engine-1"; then
    echo "Error: Rocketship engine for $STACK is not running."
    echo "Please start the $STACK environment first:"
    echo "  docker-compose --env-file $ENV_FILE up -d"
    exit 1
fi

# Build the image if it doesn't exist
if ! docker image inspect $IMAGE > /dev/null 2>&1; then
    echo "Building Rocketship CLI image..."
    docker build -f "$SCRIPT_DIR/Dockerfile.cli" -t $IMAGE "$SCRIPT_DIR/.." || exit 1
fi

# Determine if we need to add engine host parameter
NEEDS_ENGINE_HOST=""
case "$1" in
    run|get|list|start|stop)
        NEEDS_ENGINE_HOST="--engine $ENGINE_HOST"
        ;;
esac

echo "🚀 Connecting to Rocketship $STACK environment..."
echo "📡 Engine: $ENGINE_HOST"
echo "🌐 Network: $NETWORK"

# Run the command
docker run --rm \
    --network $NETWORK \
    -v "$(pwd)":/workspace \
    -w /workspace \
    $IMAGE "$@" $NEEDS_ENGINE_HOST