#!/bin/bash
# Start services for this worktree
cd "$(dirname "$0")"
echo "Starting services for rocketship-rocketship-bugs..."
docker-compose -p rocketship-rocketship-bugs up -d
echo "Services started! Temporal UI: http://localhost:8118"
