#!/usr/bin/env bash
set -euo pipefail

# Start Development Environment
# This is a convenience script that starts all necessary processes for local development
# Run this AFTER you've run scripts/setup-local-dev.sh at least once

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
cd "$REPO_ROOT"

# Defaults (can be overridden via environment)
MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-rocketship}
ROCKETSHIP_NAMESPACE=${ROCKETSHIP_NAMESPACE:-rocketship}

# Detect ingress controller ClusterIP dynamically
echo "Detecting ingress controller ClusterIP..."
ROCKETSHIP_INGRESS_IP=$(kubectl get svc -n ingress-nginx ingress-nginx-controller -o jsonpath='{.spec.clusterIP}' 2>/dev/null)

if [ -z "$ROCKETSHIP_INGRESS_IP" ]; then
  echo "ERROR: Could not detect ingress controller ClusterIP!"
  echo "Make sure you've run 'scripts/setup-local-dev.sh' first."
  exit 1
fi

# Export for Skaffold
export ROCKETSHIP_INGRESS_IP
export MINIKUBE_PROFILE
export ROCKETSHIP_NAMESPACE

# Check if minikube is running
if ! minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
  echo "Error: Minikube profile '$MINIKUBE_PROFILE' is not running."
  echo "Run 'scripts/setup-local-dev.sh' first to set up the environment."
  exit 1
fi

# Set Docker environment to use minikube
eval "$(minikube -p "$MINIKUBE_PROFILE" docker-env)"

# Check if /etc/hosts has the entry
if ! grep -q "auth.minikube.local" /etc/hosts 2>/dev/null; then
  echo ""
  echo "WARNING: /etc/hosts missing entry for auth.minikube.local"
  echo "Run: echo '127.0.0.1 auth.minikube.local' | sudo tee -a /etc/hosts"
  echo ""
  read -p "Press Enter to continue anyway, or Ctrl+C to abort..."
fi

echo "Starting local development environment..."
echo ""
echo "Using configuration:"
echo "  MINIKUBE_PROFILE=$MINIKUBE_PROFILE"
echo "  ROCKETSHIP_INGRESS_IP=$ROCKETSHIP_INGRESS_IP (for engine hostAliases)"
echo ""

# Create a temporary directory for log files
LOG_DIR=$(mktemp -d)
echo "Process logs will be saved to: $LOG_DIR"
echo ""

# Function to cleanup background processes on exit
cleanup() {
  echo ""
  echo "Shutting down development environment..."

  # Kill background processes
  if [ -n "${TUNNEL_PID:-}" ] && kill -0 "$TUNNEL_PID" 2>/dev/null; then
    echo "Stopping minikube tunnel (PID: $TUNNEL_PID)..."
    sudo kill "$TUNNEL_PID" 2>/dev/null || true
  fi

  if [ -n "${VITE_PID:-}" ] && kill -0 "$VITE_PID" 2>/dev/null; then
    echo "Stopping Vite dev server (PID: $VITE_PID)..."
    kill "$VITE_PID" 2>/dev/null || true
  fi

  if [ -n "${SKAFFOLD_PID:-}" ] && kill -0 "$SKAFFOLD_PID" 2>/dev/null; then
    echo "Stopping Skaffold (PID: $SKAFFOLD_PID)..."
    kill "$SKAFFOLD_PID" 2>/dev/null || true
  fi

  echo "Cleanup complete. Logs saved in: $LOG_DIR"
  exit 0
}

trap cleanup SIGINT SIGTERM

# Start minikube tunnel in background
echo "[1/3] Starting minikube tunnel (requires sudo)..."
sudo minikube tunnel -p "$MINIKUBE_PROFILE" > "$LOG_DIR/minikube-tunnel.log" 2>&1 &
TUNNEL_PID=$!
echo "  → minikube tunnel started (PID: $TUNNEL_PID, log: $LOG_DIR/minikube-tunnel.log)"
sleep 2

# Start Vite dev server in background
echo "[2/3] Starting Vite dev server..."
cd "$REPO_ROOT/web"
npm run dev > "$LOG_DIR/vite.log" 2>&1 &
VITE_PID=$!
cd "$REPO_ROOT"
echo "  → Vite started (PID: $VITE_PID, log: $LOG_DIR/vite.log)"
sleep 3

# Start Skaffold dev mode (with no port-forward since we're using minikube tunnel)
echo "[3/3] Starting Skaffold in development mode..."
echo "  → Watching for code changes and rebuilding on save..."
echo ""
echo "=========================================="
echo "Development environment ready!"
echo "=========================================="
echo ""
echo "URLs:"
echo "  Web UI:       http://auth.minikube.local"
echo "  Engine gRPC:  auth.minikube.local:7700 (via ingress)"
echo "  Vite HMR:     http://localhost:5173 (direct access)"
echo ""
echo "Logs:"
echo "  Minikube tunnel: $LOG_DIR/minikube-tunnel.log"
echo "  Vite:            $LOG_DIR/vite.log"
echo "  Skaffold:        (streaming below)"
echo ""
echo "Press Ctrl+C to stop all processes and exit"
echo ""
echo "------------------------------------------"
echo ""

# Run Skaffold in foreground (streaming logs)
# All traffic goes through minikube tunnel → ingress
skaffold dev --cleanup=true

# If we get here, skaffold exited (user pressed Ctrl+C)
cleanup
