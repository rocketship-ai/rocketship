#!/bin/bash
# Initialize Rocketship Stack for Current Git Worktree
# This script sets up an isolated Docker environment for the current git worktree

set -e

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source utilities
source "$SCRIPT_DIR/stack-utils.sh"

# Function to show usage
usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Initialize an isolated Rocketship environment for the current git worktree."
    echo ""
    echo "Options:"
    echo "  -f, --force     Force re-initialization (overwrite existing environment)"
    echo "  -s, --show      Show what would be created without creating it"
    echo "  -h, --help      Show this help message"
    echo ""
    echo "This script will:"
    echo "  1. Auto-detect the current git branch/worktree"
    echo "  2. Calculate unique ports to avoid conflicts"
    echo "  3. Generate environment configuration"
    echo "  4. Set up the isolated Docker stack"
    echo ""
    exit 1
}

# Parse command line arguments
FORCE_INIT=false
SHOW_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--force)
            FORCE_INIT=true
            shift
            ;;
        -s|--show)
            SHOW_ONLY=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Main initialization function
main() {
    log_info "Initializing Rocketship stack for current worktree..."
    echo ""
    
    # Check Docker is running
    if ! check_docker; then
        exit 1
    fi
    
    # Get stack information
    local stack_name=$(get_stack_name)
    local git_branch=$(get_git_branch)
    local port_offset=$(get_port_offset "$stack_name")
    
    # Show stack information
    show_stack_info "$stack_name"
    echo ""
    
    # Check if already initialized
    if env_file_exists "$stack_name" && [[ "$FORCE_INIT" != "true" ]]; then
        log_warn "Stack '$stack_name' is already initialized."
        log_info "Environment file: $SCRIPT_DIR/.env.${stack_name}"
        log_info "Use --force to re-initialize or run './rocketship start' to start the stack."
        return 0
    fi
    
    if [[ "$SHOW_ONLY" == "true" ]]; then
        log_info "Show mode - no changes will be made."
        echo ""
        log_info "Would create environment file: .env.${stack_name}"
        log_info "Would use ports with offset: $port_offset"
        return 0
    fi
    
    # Generate environment file
    log_info "Generating environment configuration..."
    generate_env_file "$stack_name" "$git_branch" "$port_offset"
    
    log_success "Stack initialization completed!"
    echo ""
    log_info "Next steps:"
    echo "  1. Start the stack:     ./rocketship start"
    echo "  2. Run tests:          ./rocketship run -f test.yaml"
    echo "  3. Check status:       ./rocketship status"
    echo "  4. Stop the stack:     ./rocketship stop"
    echo ""
    log_info "Temporal UI will be available at: http://localhost:$(calculate_ports "$stack_name" | grep TEMPORAL_UI_PORT | cut -d'=' -f2)"
}

# Function to generate environment file from template
generate_env_file() {
    local stack_name="$1"
    local git_branch="$2"
    local port_offset="$3"
    
    local template_file="$SCRIPT_DIR/templates/.env.template"
    local output_file="$SCRIPT_DIR/.env.${stack_name}"
    
    if [[ ! -f "$template_file" ]]; then
        log_error "Template file not found: $template_file"
        exit 1
    fi
    
    # Calculate all ports
    local ports_info=$(calculate_ports "$stack_name")
    
    # Parse individual ports
    local temporal_port=$(echo "$ports_info" | grep "^TEMPORAL_PORT=" | cut -d'=' -f2)
    local temporal_ui_port=$(echo "$ports_info" | grep "^TEMPORAL_UI_PORT=" | cut -d'=' -f2)
    local engine_port=$(echo "$ports_info" | grep "^ENGINE_PORT=" | cut -d'=' -f2)
    local engine_metrics_port=$(echo "$ports_info" | grep "^ENGINE_METRICS_PORT=" | cut -d'=' -f2)
    local elasticsearch_port=$(echo "$ports_info" | grep "^ELASTICSEARCH_PORT=" | cut -d'=' -f2)
    local postgres_port=$(echo "$ports_info" | grep "^POSTGRES_PORT=" | cut -d'=' -f2)
    local postgres_test_port=$(echo "$ports_info" | grep "^POSTGRES_TEST_PORT=" | cut -d'=' -f2)
    local mysql_test_port=$(echo "$ports_info" | grep "^MYSQL_TEST_PORT=" | cut -d'=' -f2)
    
    # Generate timestamp
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    # Create environment file from template
    sed \
        -e "s/__STACK_NAME__/${stack_name}/g" \
        -e "s/__GIT_BRANCH__/${git_branch}/g" \
        -e "s/__PORT_OFFSET__/${port_offset}/g" \
        -e "s/__TIMESTAMP__/${timestamp}/g" \
        -e "s/__TEMPORAL_PORT__/${temporal_port}/g" \
        -e "s/__TEMPORAL_UI_PORT__/${temporal_ui_port}/g" \
        -e "s/__ENGINE_PORT__/${engine_port}/g" \
        -e "s/__ENGINE_METRICS_PORT__/${engine_metrics_port}/g" \
        -e "s/__ELASTICSEARCH_PORT__/${elasticsearch_port}/g" \
        -e "s/__POSTGRES_PORT__/${postgres_port}/g" \
        -e "s/__POSTGRES_TEST_PORT__/${postgres_test_port}/g" \
        -e "s/__MYSQL_TEST_PORT__/${mysql_test_port}/g" \
        "$template_file" > "$output_file"
    
    log_success "Environment file created: .env.${stack_name}"
}

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi