#!/bin/bash
# Start services for this worktree
cd "$(dirname "$0")"
echo "Starting services for rocketship-rocketship..."

# Load both env files
set -a
source .env
source .env.local
set +a

docker-compose -p rocketship-rocketship up -d
echo "Services started! Temporal UI: http://localhost:8082"
