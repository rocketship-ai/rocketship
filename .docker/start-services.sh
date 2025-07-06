#!/bin/bash
# Start services for this worktree
cd "$(dirname "$0")"
echo "Starting services for rocketship-rocketship-workflow-enhancements..."
docker-compose --env-file .env.local -p rocketship-rocketship-workflow-enhancements up -d
echo "Services started! Temporal UI: http://localhost:8129"
