#!/bin/bash
# Stop services for this worktree
cd "$(dirname "$0")"
echo "Stopping services for rocketship-rocketship-bugs..."
docker-compose -p rocketship-rocketship-bugs down -v
echo "Services stopped and cleaned up."
