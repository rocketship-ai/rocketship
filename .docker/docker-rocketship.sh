#!/bin/bash
# Docker-based Rocketship CLI wrapper
# This script runs the Rocketship CLI in a container, connecting to the dockerized engine

# Set default values
NETWORK="temporal-network"
ENGINE_HOST="engine:7700"
IMAGE="rocketship-cli:latest"

# Function to show usage
usage() {
    echo "Usage: $0 [rocketship-command] [arguments...]"
    echo ""
    echo "This script runs Rocketship CLI commands in a Docker container."
    echo "It automatically connects to the dockerized Rocketship engine."
    echo ""
    echo "Examples:"
    echo "  $0 validate test.yaml"
    echo "  $0 run -f test.yaml"
    echo "  $0 list runs"
    echo "  $0 get run <run-id>"
    echo ""
    echo "Note: Files should be in the current directory or subdirectories."
    exit 1
}

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker first."
    exit 1
fi

# Check if the network exists
if ! docker network inspect $NETWORK > /dev/null 2>&1; then
    echo "Error: Docker network '$NETWORK' not found. Please run 'docker-compose up -d' first."
    exit 1
fi

# Check if the engine is running
if ! docker ps | grep -q engine; then
    echo "Error: Rocketship engine is not running. Please run 'docker-compose up -d' first."
    exit 1
fi

# Build the image if it doesn't exist
if ! docker image inspect $IMAGE > /dev/null 2>&1; then
    echo "Building Rocketship CLI image..."
    docker build -f Dockerfile.cli -t $IMAGE .. || exit 1
fi

# Determine if we need to add engine host parameter
NEEDS_ENGINE_HOST=""
case "$1" in
    run|get|list|start|stop)
        NEEDS_ENGINE_HOST="--engine $ENGINE_HOST"
        ;;
esac

# Run the command
docker run --rm \
    --network $NETWORK \
    -v "$(pwd)":/workspace \
    -w /workspace \
    $IMAGE "$@" $NEEDS_ENGINE_HOST