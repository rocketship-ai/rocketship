#!/usr/bin/env bash
set -euo pipefail

MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-rocketship}
TEMPORAL_NAMESPACE=${TEMPORAL_NAMESPACE:-temporal}
ROCKETSHIP_NAMESPACE=${ROCKETSHIP_NAMESPACE:-rocketship}
TEMPORAL_RELEASE=${TEMPORAL_RELEASE:-temporal}
ROCKETSHIP_RELEASE=${ROCKETSHIP_RELEASE:-rocketship}
ROCKETSHIP_CHART_PATH=${ROCKETSHIP_CHART_PATH:-charts/rocketship}

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

# Determine Temporal frontend host
TEMPORAL_HOST="${TEMPORAL_RELEASE}-frontend.${TEMPORAL_NAMESPACE}:7233"

# Install/upgrade Rocketship chart
if helm status "$ROCKETSHIP_RELEASE" -n "$ROCKETSHIP_NAMESPACE" >/dev/null 2>&1; then
  echo "Upgrading Rocketship chart..."
  helm upgrade "$ROCKETSHIP_RELEASE" "$ROCKETSHIP_CHART_PATH" \
    --namespace "$ROCKETSHIP_NAMESPACE" --create-namespace \
    --set temporal.host="$TEMPORAL_HOST" \
    --wait
else
  echo "Installing Rocketship chart..."
  helm install "$ROCKETSHIP_RELEASE" "$ROCKETSHIP_CHART_PATH" \
    --namespace "$ROCKETSHIP_NAMESPACE" --create-namespace \
    --set temporal.host="$TEMPORAL_HOST" \
    --wait
fi

echo "Deployment complete!"
echo
cat <<SUMMARY
Temporal namespace:   $TEMPORAL_NAMESPACE (release $TEMPORAL_RELEASE)
Rocketship namespace: $ROCKETSHIP_NAMESPACE (release $ROCKETSHIP_RELEASE)
Temporal host used:   $TEMPORAL_HOST

To port-forward the Rocketship engine gRPC endpoint:
  kubectl port-forward -n $ROCKETSHIP_NAMESPACE svc/${ROCKETSHIP_RELEASE}-rocketship-engine 7700:7700

To port-forward the Temporal Frontend:
  kubectl port-forward -n $TEMPORAL_NAMESPACE svc/${TEMPORAL_RELEASE}-frontend 7233:7233
SUMMARY
