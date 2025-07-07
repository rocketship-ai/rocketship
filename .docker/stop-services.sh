#!/bin/bash
# Stop services for this worktree
cd "$(dirname "$0")"
echo "Stopping services for rocketship-rocketship..."
docker-compose -p rocketship-rocketship down -v
echo "Services stopped and cleaned up."
