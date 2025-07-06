#!/bin/bash
# Stop services for this worktree
cd "$(dirname "$0")"
echo "Stopping services for rocketship-rocketship-workflow-enhancements..."
docker-compose -p rocketship-rocketship-workflow-enhancements down -v
echo "Services stopped and cleaned up."
