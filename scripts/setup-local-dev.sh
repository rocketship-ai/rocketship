#!/usr/bin/env bash
set -euo pipefail

# Setup Local Development Environment
# This script prepares the minikube environment for Skaffold-based development
# Run this ONCE to set up infrastructure, then use Skaffold for hot-reloading

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
cd "$REPO_ROOT"

# Load .env file if it exists
if [ -f "$REPO_ROOT/.env" ]; then
  echo "Loading secrets from .env file..."
  set -a
  source "$REPO_ROOT/.env"
  set +a
fi

MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-rocketship}
TEMPORAL_NAMESPACE=${TEMPORAL_NAMESPACE:-rocketship}
ROCKETSHIP_NAMESPACE=${ROCKETSHIP_NAMESPACE:-rocketship}
TEMPORAL_RELEASE=${TEMPORAL_RELEASE:-temporal}
TEMPORAL_WORKFLOW_NAMESPACE=${TEMPORAL_WORKFLOW_NAMESPACE:-default}

# Helper to append a line to .env if not already present
append_to_env() {
  local key="$1"
  local value="$2"
  if ! grep -q "^${key}=" "$REPO_ROOT/.env" 2>/dev/null; then
    # Ensure file ends with newline before appending to avoid concatenating onto last line
    if [ -f "$REPO_ROOT/.env" ] && [ -s "$REPO_ROOT/.env" ]; then
      # Check if file ends with newline; if not, add one
      if [ "$(tail -c1 "$REPO_ROOT/.env" | wc -l)" -eq 0 ]; then
        echo "" >> "$REPO_ROOT/.env"
      fi
    fi
    echo "${key}=${value}" >> "$REPO_ROOT/.env"
    echo "Added ${key} to .env"
  fi
}

check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required but not installed." >&2
    exit 1
  fi
}

echo "Checking required tools..."
check_command minikube
check_command kubectl
check_command helm
check_command skaffold

# =============================================================================
# Validate required secrets from .env
# =============================================================================

ROCKETSHIP_GITHUB_CLIENT_ID=${ROCKETSHIP_GITHUB_CLIENT_ID:-}
ROCKETSHIP_GITHUB_CLIENT_SECRET=${ROCKETSHIP_GITHUB_CLIENT_SECRET:-}
if [ -z "$ROCKETSHIP_GITHUB_CLIENT_ID" ] || [ -z "$ROCKETSHIP_GITHUB_CLIENT_SECRET" ]; then
  echo "ERROR: GitHub OAuth credentials not set."
  echo "  Create a GitHub OAuth App at https://github.com/settings/developers"
  echo "  IMPORTANT: Enable 'Device Flow' in the OAuth App settings"
  echo "  Set callback URL to: http://auth.minikube.local/callback"
  echo "  Then add to .env:"
  echo "    ROCKETSHIP_GITHUB_CLIENT_ID=<your-client-id>"
  echo "    ROCKETSHIP_GITHUB_CLIENT_SECRET=<your-client-secret>"
  exit 1
fi

ROCKETSHIP_EMAIL_FROM=${ROCKETSHIP_EMAIL_FROM:-}
ROCKETSHIP_POSTMARK_SERVER_TOKEN=${ROCKETSHIP_POSTMARK_SERVER_TOKEN:-}
if [ -z "$ROCKETSHIP_EMAIL_FROM" ] || [ -z "$ROCKETSHIP_POSTMARK_SERVER_TOKEN" ]; then
  echo "ERROR: Email configuration missing. Create a .env file with ROCKETSHIP_EMAIL_FROM and ROCKETSHIP_POSTMARK_SERVER_TOKEN."
  echo "  See .env.example for required variables."
  exit 1
fi

# =============================================================================
# Generate auto-managed secrets if not present
# =============================================================================

# Ensure .secrets directory exists
mkdir -p "$REPO_ROOT/.secrets"

# Generate refresh token key if not present
ROCKETSHIP_CONTROLPLANE_REFRESH_KEY=${ROCKETSHIP_CONTROLPLANE_REFRESH_KEY:-}
if [ -z "$ROCKETSHIP_CONTROLPLANE_REFRESH_KEY" ]; then
  echo "Generating ROCKETSHIP_CONTROLPLANE_REFRESH_KEY..."
  ROCKETSHIP_CONTROLPLANE_REFRESH_KEY=$(openssl rand -base64 32)
  append_to_env "ROCKETSHIP_CONTROLPLANE_REFRESH_KEY" "$ROCKETSHIP_CONTROLPLANE_REFRESH_KEY"
fi

# Generate signing key if not present
ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE=${ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE:-}
if [ -z "$ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE" ]; then
  ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE=".secrets/controlplane-signing-key.pem"
  append_to_env "ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE" "$ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE"
fi

# Generate the signing key file if it doesn't exist
SIGNING_KEY_PATH="$REPO_ROOT/$ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE"
if [ ! -f "$SIGNING_KEY_PATH" ]; then
  echo "Generating RSA signing key at $ROCKETSHIP_CONTROLPLANE_SIGNING_KEY_FILE..."
  openssl genrsa -out "$SIGNING_KEY_PATH" 2048
fi

# Start or ensure minikube is running
if ! minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
  echo "Starting minikube profile $MINIKUBE_PROFILE..."
  # Increased resources for Go builds + Temporal stack
  # 6 CPUs and 12GB RAM to handle concurrent builds and heavy workloads
  minikube start -p "$MINIKUBE_PROFILE" --cpus=6 --memory=12288
else
  echo "Minikube profile $MINIKUBE_PROFILE already running."
fi

# Use the correct profile for subsequent kubectl/helm commands
export MINIKUBE_PROFILE
kubectl config use-context "$MINIKUBE_PROFILE" >/dev/null 2>&1 || true

# Point Docker CLI to minikube's Docker daemon for Skaffold
eval "$(minikube -p "$MINIKUBE_PROFILE" docker-env)"

# Enable ingress addon for controlplane hostname routing
echo "Enabling ingress addon..."
minikube addons enable ingress -p "$MINIKUBE_PROFILE"

# Wait for ingress controller and get its ClusterIP for hostAliases
echo "Waiting for ingress controller to be ready..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s >/dev/null 2>&1 || echo "Warning: ingress controller may not be fully ready"

INGRESS_CONTROLLER_IP=$(kubectl get svc -n ingress-nginx ingress-nginx-controller -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
if [ -z "$INGRESS_CONTROLLER_IP" ]; then
  echo "ERROR: Could not get ingress controller ClusterIP. Engine will not be able to reach auth.minikube.local"
  echo "  This is required for authentication to work. Exiting."
  exit 1
else
  echo "Detected ingress controller ClusterIP: $INGRESS_CONTROLLER_IP"
fi

# Add Temporal repo if not present
if ! helm repo list 2>/dev/null | grep -q "go.temporal.io/helm-charts"; then
  echo "Adding Temporal Helm repository..."
  helm repo add temporal https://go.temporal.io/helm-charts
fi
helm repo update

# Install Temporal with minimal footprint suitable for minikube
if ! helm status "$TEMPORAL_RELEASE" -n "$TEMPORAL_NAMESPACE" >/dev/null 2>&1; then
  echo "Installing Temporal (this may take several minutes)..."
  helm install "$TEMPORAL_RELEASE" temporal/temporal \
    --namespace "$TEMPORAL_NAMESPACE" --create-namespace \
    --set server.replicaCount=1 \
    --set cassandra.config.cluster_size=1 \
    --set elasticsearch.replicas=1 \
    --set prometheus.enabled=false \
    --set grafana.enabled=false \
    --wait --timeout 15m
else
  echo "Temporal release $TEMPORAL_RELEASE already installed. Skipping."
fi

# Ensure Temporal workflow namespace exists
ADMIN_TOOLS_DEPLOY="${TEMPORAL_RELEASE}-admintools"
echo "Ensuring Temporal namespace '$TEMPORAL_WORKFLOW_NAMESPACE' exists..."
if kubectl --namespace "$TEMPORAL_NAMESPACE" rollout status "deployment/$ADMIN_TOOLS_DEPLOY" --timeout=120s >/dev/null 2>&1; then
  if ! kubectl exec -n "$TEMPORAL_NAMESPACE" "deploy/$ADMIN_TOOLS_DEPLOY" -- temporal operator namespace describe "$TEMPORAL_WORKFLOW_NAMESPACE" >/dev/null 2>&1; then
    kubectl exec -n "$TEMPORAL_NAMESPACE" "deploy/$ADMIN_TOOLS_DEPLOY" -- temporal operator namespace create --namespace "$TEMPORAL_WORKFLOW_NAMESPACE" --retention 7d || true
  fi
else
  echo "WARNING: admin tools deployment not ready; skipping namespace registration"
fi

# Create namespace for Rocketship if it doesn't exist
kubectl create namespace "$ROCKETSHIP_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# =============================================================================
# Create Kubernetes secrets
# =============================================================================
echo "Creating/updating secrets for controlplane and Postgres..."

# Postgres password (use consistent password for local dev)
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-rocketship-dev-password}
kubectl create secret generic rocketship-postgres-auth \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=password="$POSTGRES_PASSWORD" \
  --from-literal=postgres-password="$POSTGRES_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -

# Controlplane database URL (points to the postgres service that will be created by Skaffold)
kubectl create secret generic rocketship-controlplane-database \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=DATABASE_URL="postgres://rocketship:${POSTGRES_PASSWORD}@rocketship-postgresql:5432/rocketship?sslmode=disable" \
  --dry-run=client -o yaml | kubectl apply -f -

# Controlplane refresh-token HMAC key
kubectl create secret generic rocketship-controlplane-secrets \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=ROCKETSHIP_CONTROLPLANE_REFRESH_KEY="$ROCKETSHIP_CONTROLPLANE_REFRESH_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -

# Controlplane RSA signing key for JWTs
kubectl create secret generic rocketship-controlplane-signing \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-file=signing-key.pem="$SIGNING_KEY_PATH" \
  --dry-run=client -o yaml | kubectl apply -f -

# Worker service account token (for worker -> engine log forwarding)
# Generate a long-lived JWT using the same signing key as the controlplane
echo "Generating worker service account token..."
WORKER_TOKEN=$(go run "$REPO_ROOT/scripts/gen-worker-token/main.go" \
  -key "$SIGNING_KEY_PATH" \
  -issuer "http://auth.minikube.local" \
  -audience "rocketship-cli" \
  -ttl "720h")  # 30 days

kubectl create secret generic rocketship-worker-token \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=token="$WORKER_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
echo "Worker token secret created."

# GitHub OAuth secret (includes both client ID and secret)
kubectl create secret generic rocketship-github-oauth \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=ROCKETSHIP_GITHUB_CLIENT_ID="$ROCKETSHIP_GITHUB_CLIENT_ID" \
  --from-literal=ROCKETSHIP_GITHUB_CLIENT_SECRET="$ROCKETSHIP_GITHUB_CLIENT_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -

# Postmark email secret
kubectl create secret generic rocketship-postmark-secret \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=ROCKETSHIP_EMAIL_FROM="$ROCKETSHIP_EMAIL_FROM" \
  --from-literal=ROCKETSHIP_POSTMARK_SERVER_TOKEN="$ROCKETSHIP_POSTMARK_SERVER_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

# GitHub App secret (for repo access)
ROCKETSHIP_GITHUB_APP_ID=${ROCKETSHIP_GITHUB_APP_ID:-}
ROCKETSHIP_GITHUB_APP_SLUG=${ROCKETSHIP_GITHUB_APP_SLUG:-}
ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE=${ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE:-}
if [ -n "$ROCKETSHIP_GITHUB_APP_ID" ] && [ -n "$ROCKETSHIP_GITHUB_APP_SLUG" ] && [ -n "$ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE" ]; then
  if [ ! -f "$ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE" ]; then
    echo "ERROR: GitHub App private key file not found: $ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE"
    exit 1
  fi
  GITHUB_APP_PRIVATE_KEY_PEM=$(cat "$ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE")
  kubectl create secret generic rocketship-github-app \
    --namespace "$ROCKETSHIP_NAMESPACE" \
    --from-literal=ROCKETSHIP_GITHUB_APP_ID="$ROCKETSHIP_GITHUB_APP_ID" \
    --from-literal=ROCKETSHIP_GITHUB_APP_SLUG="$ROCKETSHIP_GITHUB_APP_SLUG" \
    --from-literal=ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_PEM="$GITHUB_APP_PRIVATE_KEY_PEM" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "GitHub App secret created."
else
  echo "WARNING: GitHub App not configured. Set ROCKETSHIP_GITHUB_APP_ID, ROCKETSHIP_GITHUB_APP_SLUG, and ROCKETSHIP_GITHUB_APP_PRIVATE_KEY_FILE to enable repo access."
fi

# GitHub webhook secret (for webhook ingestion via smee.io relay)
ROCKETSHIP_GITHUB_WEBHOOK_SECRET=${ROCKETSHIP_GITHUB_WEBHOOK_SECRET:-}
ROCKETSHIP_GITHUB_WEBHOOK_SMEE_URL=${ROCKETSHIP_GITHUB_WEBHOOK_SMEE_URL:-}
if [ -n "$ROCKETSHIP_GITHUB_WEBHOOK_SECRET" ] && [ -n "$ROCKETSHIP_GITHUB_WEBHOOK_SMEE_URL" ]; then
  kubectl create secret generic rocketship-github-webhook \
    --namespace "$ROCKETSHIP_NAMESPACE" \
    --from-literal=ROCKETSHIP_GITHUB_WEBHOOK_SECRET="$ROCKETSHIP_GITHUB_WEBHOOK_SECRET" \
    --from-literal=ROCKETSHIP_GITHUB_WEBHOOK_SMEE_URL="$ROCKETSHIP_GITHUB_WEBHOOK_SMEE_URL" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "GitHub webhook secret created."
else
  echo "Skipping rocketship-github-webhook secret; set ROCKETSHIP_GITHUB_WEBHOOK_SECRET and ROCKETSHIP_GITHUB_WEBHOOK_SMEE_URL in .env"
fi

# Deploy vite-relay for web UI development
echo "Deploying vite-relay for local web UI development..."

# Detect the minikube driver to choose the right host address
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
  echo "Using gateway IP: $HOST_IP"
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

# Create vite-relay deployment and service
kubectl apply -n "$ROCKETSHIP_NAMESPACE" -f - <<VITE_RELAY
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vite-relay
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vite-relay
  template:
    metadata:
      labels:
        app: vite-relay
    spec:
      containers:
        - name: socat
          image: alpine/socat
          args: ["-d","-d","TCP4-LISTEN:5173,fork,reuseaddr","TCP4:${HOST_IP}:5173"]
          ports:
            - containerPort: 5173
---
apiVersion: v1
kind: Service
metadata:
  name: vite-relay
spec:
  selector:
    app: vite-relay
  ports:
    - name: http
      port: 5173
      targetPort: 5173
VITE_RELAY

echo ""
echo "=========================================="
echo "Infrastructure Setup Complete!"
echo "=========================================="
echo ""
cat <<SUMMARY
Temporal namespace:          $TEMPORAL_NAMESPACE (release $TEMPORAL_RELEASE)
Rocketship namespace:        $ROCKETSHIP_NAMESPACE
Temporal host:               temporal-frontend.${TEMPORAL_NAMESPACE}:7233
Temporal workflow namespace: $TEMPORAL_WORKFLOW_NAMESPACE
Ingress controller IP:       ${INGRESS_CONTROLLER_IP:-not detected}
Host IP for vite-relay:      $HOST_IP

Infrastructure deployed:
- Temporal (workflow engine)
- PostgreSQL database
- Vite Relay (proxies to host Vite on ${HOST_IP}:5173)
- All secrets configured

Next Steps:
============

1. Configure local DNS:
   echo "127.0.0.1 auth.minikube.local" | sudo tee -a /etc/hosts

2. Use the convenience script to start everything at once:
   scripts/start-dev.sh

   Or manually:
   - Start minikube tunnel: sudo minikube tunnel -p $MINIKUBE_PROFILE
   - Start Vite dev server: cd web && npm run dev
   - Run Skaffold: skaffold dev

3. Visit http://auth.minikube.local and sign in with GitHub

To view logs:
  kubectl logs -n $ROCKETSHIP_NAMESPACE -l app.kubernetes.io/component=engine --tail=50 -f
  kubectl logs -n $ROCKETSHIP_NAMESPACE -l app.kubernetes.io/component=controlplane --tail=50 -f
  kubectl logs -n $ROCKETSHIP_NAMESPACE -l app=vite-relay --tail=50 -f
SUMMARY
