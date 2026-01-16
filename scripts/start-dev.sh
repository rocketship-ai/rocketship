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

# Detect host address for vite-relay based on minikube driver
echo "Detecting host address for vite-relay..."

# Use minikube config to get driver for specific profile (more reliable than parsing profile list)
MINIKUBE_DRIVER=$(minikube config get driver -p "$MINIKUBE_PROFILE" 2>/dev/null || echo "")

# Fallback: try to get from profile list if config doesn't work
if [ -z "$MINIKUBE_DRIVER" ]; then
  MINIKUBE_DRIVER=$(minikube profile list -o json 2>/dev/null | grep -o "\"Name\":\"$MINIKUBE_PROFILE\"[^}]*\"Driver\":\"[^\"]*\"" | grep -o '"Driver":"[^"]*"' | cut -d'"' -f4)
fi

echo "Detected minikube driver: ${MINIKUBE_DRIVER:-unknown}"

# Determine HOST_IP based on driver
if [ "$MINIKUBE_DRIVER" = "docker" ]; then
  # Docker driver: use host.docker.internal (works reliably on macOS/Windows)
  HOST_IP="host.docker.internal"
  echo "Using host.docker.internal for Docker driver"
elif [ -n "$MINIKUBE_DRIVER" ]; then
  # Other known drivers (hyperkit, virtualbox, etc.): detect gateway IP
  MINIKUBE_IP=$(minikube ip -p "$MINIKUBE_PROFILE" 2>/dev/null)
  HOST_IP="${MINIKUBE_IP%.*}.1"
  echo "Using gateway IP: $HOST_IP (minikube IP: $MINIKUBE_IP)"
else
  # Fallback: default to host.docker.internal on macOS (most common setup)
  if [ "$(uname)" = "Darwin" ]; then
    HOST_IP="host.docker.internal"
    echo "WARNING: Could not detect minikube driver. Defaulting to host.docker.internal for macOS."
  else
    # Linux: try to detect gateway
    MINIKUBE_IP=$(minikube ip -p "$MINIKUBE_PROFILE" 2>/dev/null)
    if [ -n "$MINIKUBE_IP" ]; then
      HOST_IP="${MINIKUBE_IP%.*}.1"
      echo "WARNING: Could not detect minikube driver. Using gateway IP: $HOST_IP"
    else
      HOST_IP="host.docker.internal"
      echo "WARNING: Could not detect minikube driver or IP. Defaulting to host.docker.internal."
    fi
  fi
fi

# Update vite-relay deployment if it exists and has wrong host
CURRENT_VITE_HOST=$(kubectl get deployment vite-relay -n "$ROCKETSHIP_NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].args[3]}' 2>/dev/null | sed 's/TCP4:\(.*\):5173/\1/')
if [ -n "$CURRENT_VITE_HOST" ] && [ "$CURRENT_VITE_HOST" != "$HOST_IP" ]; then
  echo "Updating vite-relay from $CURRENT_VITE_HOST to $HOST_IP..."
  kubectl patch deployment vite-relay -n "$ROCKETSHIP_NAMESPACE" --type='json' \
    -p="[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/args\", \"value\": [\"-d\",\"-d\",\"TCP4-LISTEN:5173,fork,reuseaddr\",\"TCP4:${HOST_IP}:5173\"]}]" >/dev/null 2>&1
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

# Check if web dependencies are installed
if [ ! -d "$REPO_ROOT/web/node_modules" ]; then
  echo ""
  echo "ERROR: Web dependencies not installed."
  echo "Run the following command first:"
  echo ""
  echo "  cd web && npm install"
  echo ""
  exit 1
fi

if [ ! -x "$REPO_ROOT/web/node_modules/.bin/vite" ]; then
  echo ""
  echo "ERROR: Vite not found. Web dependencies may be corrupted."
  echo "Run the following commands:"
  echo ""
  echo "  cd web && rm -rf node_modules && npm install"
  echo ""
  exit 1
fi

echo "Starting local development environment..."
echo ""
echo "Using configuration:"
echo "  MINIKUBE_PROFILE=$MINIKUBE_PROFILE"
echo "  ROCKETSHIP_INGRESS_IP=$ROCKETSHIP_INGRESS_IP (for engine hostAliases)"
echo "  HOST_IP=$HOST_IP (for vite-relay)"
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
echo "  → Vite starting (PID: $VITE_PID, log: $LOG_DIR/vite.log)"

# Wait for Vite to be ready (check if it's listening on port 5173)
echo "  → Waiting for Vite to be ready..."
VITE_READY=false
for i in {1..15}; do
  if curl -s -o /dev/null -w "" http://localhost:5173 2>/dev/null; then
    VITE_READY=true
    break
  fi
  # Check if Vite process died
  if ! kill -0 "$VITE_PID" 2>/dev/null; then
    echo ""
    echo "ERROR: Vite failed to start. Check the log:"
    echo "  cat $LOG_DIR/vite.log"
    echo ""
    tail -20 "$LOG_DIR/vite.log" 2>/dev/null
    exit 1
  fi
  sleep 1
done

if [ "$VITE_READY" = true ]; then
  echo "  → Vite ready at http://localhost:5173"
else
  echo ""
  echo "WARNING: Vite may not be ready yet. Check log if web UI doesn't load:"
  echo "  cat $LOG_DIR/vite.log"
fi

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
