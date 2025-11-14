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

# Enable ingress addon for auth broker hostname routing
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

# Create secrets for auth broker and Postgres
echo "Creating/updating secrets for auth broker and Postgres..."

# Postgres password (use consistent password for local dev)
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-rocketship-dev-password}
kubectl create secret generic rocketship-postgres-auth \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=password="$POSTGRES_PASSWORD" \
  --from-literal=postgres-password="$POSTGRES_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -

# Auth broker database URL (points to the postgres service that will be created by Skaffold)
kubectl create secret generic rocketship-auth-broker-database \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=DATABASE_URL="postgres://rocketship:${POSTGRES_PASSWORD}@rocketship-postgresql:5432/rocketship?sslmode=disable" \
  --dry-run=client -o yaml | kubectl apply -f -

# Refresh-token HMAC key (32 bytes, Base64 encoded)
if ! kubectl get secret rocketship-auth-broker-secrets -n "$ROCKETSHIP_NAMESPACE" >/dev/null 2>&1; then
  REFRESH_KEY=$(openssl rand -base64 32)
  kubectl create secret generic rocketship-auth-broker-secrets \
    --namespace "$ROCKETSHIP_NAMESPACE" \
    --from-literal=ROCKETSHIP_BROKER_REFRESH_KEY="$REFRESH_KEY"
else
  echo "Secret rocketship-auth-broker-secrets already exists, skipping."
fi

# RSA signing key for JWTs (generate if not exists)
if ! kubectl get secret rocketship-auth-broker-signing -n "$ROCKETSHIP_NAMESPACE" >/dev/null 2>&1; then
  echo "Generating RSA signing key for auth broker..."
  TEMP_KEY=$(mktemp)
  openssl genrsa -out "$TEMP_KEY" 2048
  kubectl create secret generic rocketship-auth-broker-signing \
    --namespace "$ROCKETSHIP_NAMESPACE" \
    --from-file=signing-key.pem="$TEMP_KEY"
  rm -f "$TEMP_KEY"
else
  echo "Secret rocketship-auth-broker-signing already exists, skipping."
fi

# GitHub OAuth secret
GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET:-}
if [ -z "$GITHUB_CLIENT_SECRET" ]; then
  echo "ERROR: GITHUB_CLIENT_SECRET not set. Create a .env file with secrets from production."
  echo "  See .env.example for required variables."
  exit 1
fi
kubectl create secret generic rocketship-github-oauth \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=ROCKETSHIP_GITHUB_CLIENT_SECRET="$GITHUB_CLIENT_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -

# Postmark email secret
ROCKETSHIP_EMAIL_FROM=${ROCKETSHIP_EMAIL_FROM:-}
ROCKETSHIP_POSTMARK_SERVER_TOKEN=${ROCKETSHIP_POSTMARK_SERVER_TOKEN:-}
if [ -z "$ROCKETSHIP_EMAIL_FROM" ] || [ -z "$ROCKETSHIP_POSTMARK_SERVER_TOKEN" ]; then
  echo "ERROR: Email configuration missing. Create a .env file with ROCKETSHIP_EMAIL_FROM and ROCKETSHIP_POSTMARK_SERVER_TOKEN."
  echo "  See .env.example for required variables."
  exit 1
fi
kubectl create secret generic rocketship-postmark-secret \
  --namespace "$ROCKETSHIP_NAMESPACE" \
  --from-literal=ROCKETSHIP_EMAIL_FROM="$ROCKETSHIP_EMAIL_FROM" \
  --from-literal=ROCKETSHIP_POSTMARK_SERVER_TOKEN="$ROCKETSHIP_POSTMARK_SERVER_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

# Deploy vite-relay for web UI development
echo "Deploying vite-relay for local web UI development..."

# Detect the host IP that pods can reach
HOST_IP=""
for ip in "192.168.64.1" "192.168.49.1" "host.minikube.internal"; do
  echo "Testing connectivity to $ip from cluster..."
  if kubectl run -n "$ROCKETSHIP_NAMESPACE" test-host-ip-${ip//\./-} --rm -it --restart=Never --image=busybox:1.36 -- sh -c "wget -qO- --timeout=2 http://$ip:5173/ 2>&1 | head -n 1" >/dev/null 2>&1; then
    HOST_IP=$ip
    echo "Detected reachable host IP: $HOST_IP"
    break
  fi
done

if [ -z "$HOST_IP" ]; then
  echo "WARNING: Could not detect reachable host IP. Using 192.168.64.1 as default."
  echo "  If web UI doesn't work, check:"
  echo "  1. Vite is running: 'cd web && npm run dev'"
  echo "  2. Vite is listening on 0.0.0.0 (check vite.config.ts has 'host: true')"
  echo "  3. macOS firewall allows Node on port 5173"
  HOST_IP="192.168.64.1"
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
  kubectl logs -n $ROCKETSHIP_NAMESPACE -l app.kubernetes.io/component=auth-broker --tail=50 -f
  kubectl logs -n $ROCKETSHIP_NAMESPACE -l app=vite-relay --tail=50 -f
SUMMARY
