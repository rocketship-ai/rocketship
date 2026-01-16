#!/usr/bin/env bash
set -euo pipefail

# Delete Local Development Environment
# Removes minikube cluster while preserving .env and .secrets/

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
cd "$REPO_ROOT"

MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-rocketship}

echo "Deleting local development environment..."

# Kill any running dev processes
echo "Stopping running processes..."
pkill -f "minikube tunnel.*$MINIKUBE_PROFILE" 2>/dev/null || true
pkill -f "vite.*5173" 2>/dev/null || true
pkill -f "skaffold dev" 2>/dev/null || true

# Delete minikube cluster
if minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
  echo "Deleting minikube cluster '$MINIKUBE_PROFILE'..."
  minikube delete -p "$MINIKUBE_PROFILE"
else
  echo "Minikube cluster '$MINIKUBE_PROFILE' not found, skipping..."
fi

# Clean up Docker images
echo "Cleaning up Docker images..."
docker images --format '{{.Repository}}:{{.Tag}}' | grep -E '^rocketship-.*-local' | xargs -r docker rmi 2>/dev/null || true
docker image prune -f >/dev/null 2>&1 || true

echo ""
echo "Done. Run 'make setup-local' to recreate."
