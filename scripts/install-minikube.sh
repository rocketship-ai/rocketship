#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
cd "$REPO_ROOT"

MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-rocketship}
TEMPORAL_NAMESPACE=${TEMPORAL_NAMESPACE:-rocketship}
ROCKETSHIP_NAMESPACE=${ROCKETSHIP_NAMESPACE:-rocketship}
TEMPORAL_RELEASE=${TEMPORAL_RELEASE:-temporal}
ROCKETSHIP_RELEASE=${ROCKETSHIP_RELEASE:-rocketship}
ROCKETSHIP_CHART_PATH=${ROCKETSHIP_CHART_PATH:-charts/rocketship}
TEMPORAL_WORKFLOW_NAMESPACE=${TEMPORAL_WORKFLOW_NAMESPACE:-default}
ENGINE_IMAGE_LOCAL=${ENGINE_IMAGE_LOCAL:-rocketship-engine-local}
WORKER_IMAGE_LOCAL=${WORKER_IMAGE_LOCAL:-rocketship-worker-local}
LOCAL_IMAGE_TAG=${LOCAL_IMAGE_TAG:-dev}

check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required but not installed." >&2
    exit 1
  fi
}

check_command minikube
check_command kubectl
check_command helm

# Start or ensure minikube is running
if ! minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
  echo "Starting minikube profile $MINIKUBE_PROFILE..."
  minikube start -p "$MINIKUBE_PROFILE" --cpus=4 --memory=8192
else
  echo "Minikube profile $MINIKUBE_PROFILE already running."
fi

# Use the correct profile for subsequent kubectl/helm commands
export MINIKUBE_PROFILE
kubectl config use-context "$MINIKUBE_PROFILE" >/dev/null 2>&1 || true

# Add Temporal repo if not present
if ! helm repo list 2>/dev/null | grep -q "go.temporal.io/helm-charts"; then
  echo "Adding Temporal Helm repository..."
  helm repo add temporal https://go.temporal.io/helm-charts
fi
helm repo update

# Build local Rocketship images inside the Minikube Docker daemon
echo "Building Rocketship engine and worker images inside Minikube..."
eval "$(minikube -p "$MINIKUBE_PROFILE" docker-env)"
docker build -t "${ENGINE_IMAGE_LOCAL}:${LOCAL_IMAGE_TAG}" -f .docker/Dockerfile.engine .
docker build -t "${WORKER_IMAGE_LOCAL}:${LOCAL_IMAGE_TAG}" -f .docker/Dockerfile.worker .
eval "$(minikube -p "$MINIKUBE_PROFILE" docker-env --unset)"

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
    kubectl exec -n "$TEMPORAL_NAMESPACE" "deploy/$ADMIN_TOOLS_DEPLOY" -- temporal operator namespace create --namespace "$TEMPORAL_WORKFLOW_NAMESPACE" --retention 72h || true
  fi
else
  echo "WARNING: admin tools deployment not ready; skipping namespace registration"
fi

# Determine Temporal frontend host
TEMPORAL_HOST="${TEMPORAL_RELEASE}-frontend.${TEMPORAL_NAMESPACE}:7233"

# Install/upgrade Rocketship chart
if helm status "$ROCKETSHIP_RELEASE" -n "$ROCKETSHIP_NAMESPACE" >/dev/null 2>&1; then
  echo "Upgrading Rocketship chart..."
  helm upgrade "$ROCKETSHIP_RELEASE" "$ROCKETSHIP_CHART_PATH" \
    --namespace "$ROCKETSHIP_NAMESPACE" --create-namespace \
    --set temporal.host="$TEMPORAL_HOST" \
    --set temporal.namespace="$TEMPORAL_WORKFLOW_NAMESPACE" \
    --set engine.image.repository="$ENGINE_IMAGE_LOCAL" \
    --set engine.image.tag="$LOCAL_IMAGE_TAG" \
    --set engine.image.pullPolicy=IfNotPresent \
    --set worker.image.repository="$WORKER_IMAGE_LOCAL" \
    --set worker.image.tag="$LOCAL_IMAGE_TAG" \
    --set worker.image.pullPolicy=IfNotPresent \
    --wait
else
  echo "Installing Rocketship chart..."
  helm install "$ROCKETSHIP_RELEASE" "$ROCKETSHIP_CHART_PATH" \
    --namespace "$ROCKETSHIP_NAMESPACE" --create-namespace \
    --set temporal.host="$TEMPORAL_HOST" \
    --set temporal.namespace="$TEMPORAL_WORKFLOW_NAMESPACE" \
    --set engine.image.repository="$ENGINE_IMAGE_LOCAL" \
    --set engine.image.tag="$LOCAL_IMAGE_TAG" \
    --set engine.image.pullPolicy=IfNotPresent \
    --set worker.image.repository="$WORKER_IMAGE_LOCAL" \
    --set worker.image.tag="$LOCAL_IMAGE_TAG" \
    --set worker.image.pullPolicy=IfNotPresent \
    --wait
fi

echo "Deployment complete!"
echo
cat <<SUMMARY
Temporal namespace:   $TEMPORAL_NAMESPACE (release $TEMPORAL_RELEASE)
Rocketship namespace: $ROCKETSHIP_NAMESPACE (release $ROCKETSHIP_RELEASE)
Temporal host used:   $TEMPORAL_HOST
Temporal workflow namespace: $TEMPORAL_WORKFLOW_NAMESPACE
Engine image:         ${ENGINE_IMAGE_LOCAL}:${LOCAL_IMAGE_TAG}
Worker image:         ${WORKER_IMAGE_LOCAL}:${LOCAL_IMAGE_TAG}

To port-forward the Rocketship engine gRPC endpoint:
  kubectl port-forward -n $ROCKETSHIP_NAMESPACE svc/${ROCKETSHIP_RELEASE}-engine 7700:7700

To port-forward the Temporal Frontend:
  kubectl port-forward -n $TEMPORAL_NAMESPACE svc/${TEMPORAL_RELEASE}-frontend 7233:7233
SUMMARY
