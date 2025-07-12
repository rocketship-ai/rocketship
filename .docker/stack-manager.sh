#!/bin/bash
# Stack Manager for Rocketship Multi-Stack Environment
# This script manages starting/stopping isolated Rocketship stacks

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Function to show usage
usage() {
    echo "Usage: $0 <command> <stack>"
    echo ""
    echo "Commands:"
    echo "  start <stack>    Start the specified stack"
    echo "  stop <stack>     Stop the specified stack"  
    echo "  status <stack>   Show status of the specified stack"
    echo "  logs <stack>     Show logs for the specified stack"
    echo "  list             List all available stacks"
    echo ""
    echo "Available Stacks:"
    echo "  stack1: Primary development stack (ports 7xxx, 8xxx, 9xxx)"
    echo "  stack2: Secondary development stack (ports 8xxx, 9xxx)"
    echo ""
    echo "Examples:"
    echo "  $0 start stack1"
    echo "  $0 stop stack2"
    echo "  $0 status stack1"
    echo "  $0 logs stack2"
    exit 1
}

# Validate arguments
if [[ $# -lt 1 ]]; then
    usage
fi

COMMAND="$1"
STACK="$2"

# Handle list command
if [[ "$COMMAND" == "list" ]]; then
    echo "Available Rocketship Stacks:"
    echo ""
    echo "Stack 1 (.env.stack1):"
    echo "  - Temporal: :7233"
    echo "  - Engine: :7700"  
    echo "  - Temporal UI: :8080"
    echo "  - Elasticsearch: :9200"
    echo ""
    echo "Stack 2 (.env.stack2):"
    echo "  - Temporal: :8233"
    echo "  - Engine: :8700"
    echo "  - Temporal UI: :9080"  
    echo "  - Elasticsearch: :9300"
    exit 0
fi

# Validate stack argument for other commands
if [[ "$STACK" != "stack1" && "$STACK" != "stack2" ]]; then
    echo "Error: Invalid stack '$STACK'. Must be 'stack1' or 'stack2'"
    usage
fi

# Set environment file
ENV_FILE=".env.$STACK"

# Check if environment file exists
if [[ ! -f "$SCRIPT_DIR/$ENV_FILE" ]]; then
    echo "Error: Environment file '$ENV_FILE' not found in $SCRIPT_DIR"
    exit 1
fi

# Execute commands
case "$COMMAND" in
    start)
        echo "🚀 Starting Rocketship $STACK environment..."
        cd "$SCRIPT_DIR"
        docker-compose --env-file "$ENV_FILE" up -d
        echo "✅ $STACK started successfully!"
        echo ""
        echo "Access points:"
        if [[ "$STACK" == "stack1" ]]; then
            echo "  - Temporal UI: http://localhost:8080"
            echo "  - Engine API: localhost:7700"
        else
            echo "  - Temporal UI: http://localhost:9080"
            echo "  - Engine API: localhost:8700"
        fi
        ;;
    stop)
        echo "🛑 Stopping Rocketship $STACK environment..."
        cd "$SCRIPT_DIR"
        docker-compose --env-file "$ENV_FILE" down
        echo "✅ $STACK stopped successfully!"
        ;;
    status)
        echo "📊 Status of Rocketship $STACK environment:"
        PROJECT_NAME=$(grep COMPOSE_PROJECT_NAME "$SCRIPT_DIR/$ENV_FILE" | cut -d'=' -f2)
        docker ps --filter "name=$PROJECT_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        ;;
    logs)
        echo "📝 Logs for Rocketship $STACK environment:"
        cd "$SCRIPT_DIR"
        docker-compose --env-file "$ENV_FILE" logs -f
        ;;
    *)
        echo "Error: Invalid command '$COMMAND'"
        usage
        ;;
esac