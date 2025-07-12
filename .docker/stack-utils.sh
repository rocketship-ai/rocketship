#!/bin/bash
# Stack Utilities for Rocketship Multi-Stack Environment
# Provides auto-discovery and dynamic port allocation for git worktrees

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Function to log messages with colors
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to get current git branch name
get_git_branch() {
    if git rev-parse --git-dir > /dev/null 2>&1; then
        git branch --show-current 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo "unknown"
    else
        echo "unknown"
    fi
}

# Function to get stack name from current context
get_stack_name() {
    local branch_name=$(get_git_branch)
    
    # Clean branch name for use as stack identifier
    # Replace invalid characters with hyphens, convert to lowercase
    local stack_name=$(echo "$branch_name" | sed 's/[^a-zA-Z0-9_-]/-/g' | tr '[:upper:]' '[:lower:]')
    
    # Ensure it doesn't start with a number (Docker requirement)
    if [[ $stack_name =~ ^[0-9] ]]; then
        stack_name="stack-$stack_name"
    fi
    
    # Ensure it's not empty
    if [[ -z "$stack_name" || "$stack_name" == "unknown" ]]; then
        # Use directory name as fallback
        local dir_name=$(basename "$(pwd)")
        stack_name=$(echo "$dir_name" | sed 's/[^a-zA-Z0-9_-]/-/g' | tr '[:upper:]' '[:lower:]')
        if [[ $stack_name =~ ^[0-9] ]]; then
            stack_name="stack-$stack_name"
        fi
    fi
    
    echo "$stack_name"
}

# Function to calculate port offset from stack name
get_port_offset() {
    local stack_name="$1"
    
    # Calculate hash of stack name (using cksum for portability)
    local hash=$(echo -n "$stack_name" | cksum | cut -d' ' -f1)
    
    # Calculate offset: (hash % 50) * 100
    # This gives us 50 possible port ranges: 0, 100, 200, ..., 4900
    local offset=$(( (hash % 50) * 100 ))
    
    echo "$offset"
}

# Function to calculate ports for a stack
calculate_ports() {
    local stack_name="$1"
    local offset=$(get_port_offset "$stack_name")
    
    # Base ports + offset
    local temporal_port=$((7233 + offset))
    local temporal_ui_port=$((8080 + offset))
    local engine_port=$((7700 + offset))
    local engine_metrics_port=$((7701 + offset))
    local elasticsearch_port=$((9200 + offset))
    local postgres_port=$((5432 + offset))
    local postgres_test_port=$((5433 + offset))
    local mysql_test_port=$((3307 + offset))
    
    # Return as associative array format (for bash 4+) or simple format
    cat <<EOF
TEMPORAL_PORT=$temporal_port
TEMPORAL_UI_PORT=$temporal_ui_port
ENGINE_PORT=$engine_port
ENGINE_METRICS_PORT=$engine_metrics_port
ELASTICSEARCH_PORT=$elasticsearch_port
POSTGRES_PORT=$postgres_port
POSTGRES_TEST_PORT=$postgres_test_port
MYSQL_TEST_PORT=$mysql_test_port
EOF
}

# Function to check if stack is running
is_stack_running() {
    local stack_name="$1"
    docker ps --filter "name=rocketship-${stack_name}-engine-1" --format "{{.Names}}" | grep -q "rocketship-${stack_name}-engine-1"
}

# Function to get stack status
get_stack_status() {
    local stack_name="$1"
    local project_name="rocketship-${stack_name}"
    
    local container_count=$(docker ps --filter "name=${project_name}" --format "{{.Names}}" | wc -l)
    local healthy_count=$(docker ps --filter "name=${project_name}" --filter "health=healthy" --format "{{.Names}}" | wc -l)
    
    if [[ $container_count -eq 0 ]]; then
        echo "stopped"
    elif [[ $container_count -eq 9 ]] && [[ $healthy_count -gt 0 ]]; then
        echo "running"
    else
        echo "starting"
    fi
}

# Function to display stack info
show_stack_info() {
    local stack_name="$1"
    local ports_info=$(calculate_ports "$stack_name")
    
    echo "📊 Stack Information:"
    echo "   Name: $stack_name"
    echo "   Project: rocketship-${stack_name}"
    echo "   Branch: $(get_git_branch)"
    echo ""
    echo "🌐 Access Points:"
    
    # Parse ports from calculate_ports output
    local temporal_ui_port=$(echo "$ports_info" | grep TEMPORAL_UI_PORT | cut -d'=' -f2)
    local engine_port=$(echo "$ports_info" | grep ENGINE_PORT | cut -d'=' -f2)
    
    echo "   Temporal UI: http://localhost:${temporal_ui_port}"
    echo "   Engine API: localhost:${engine_port}"
    echo ""
    echo "🔧 Port Allocation:"
    echo "$ports_info" | sed 's/^/   /'
}

# Function to validate Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker first."
        return 1
    fi
    return 0
}

# Function to check if environment file exists
env_file_exists() {
    local stack_name="$1"
    [[ -f "$SCRIPT_DIR/.env.${stack_name}" ]]
}

# Function to check if stack needs initialization
needs_initialization() {
    local stack_name="$1"
    ! env_file_exists "$stack_name"
}

# Export functions for use in other scripts
export -f get_stack_name
export -f get_port_offset
export -f calculate_ports
export -f is_stack_running
export -f get_stack_status
export -f show_stack_info
export -f check_docker
export -f env_file_exists
export -f needs_initialization
export -f log_info
export -f log_success
export -f log_warn
export -f log_error