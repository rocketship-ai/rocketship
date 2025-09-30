#!/usr/bin/env bash
set -euo pipefail

CHART_DIR="charts/rocketship"
RELEASE_NAME="rocketship-test"
NAMESPACE="rocketship"

render() {
  helm template "$RELEASE_NAME" "$CHART_DIR" --namespace "$NAMESPACE" "$@"
}

# Default values
output=$(render)

# Ensure engine service exposes named ports
if ! grep -q "name: grpc" <<<"$output"; then
  echo "grpc port name not found in rendered templates" >&2
  exit 1
fi
if ! grep -q "name: http" <<<"$output"; then
  echo "http port name not found in rendered templates" >&2
  exit 1
fi

# Minikube values should set service type NodePort
minikube_output=$(render -f "$CHART_DIR/values-minikube.yaml")
if ! grep -q "type: NodePort" <<<"$minikube_output"; then
  echo "Expected NodePort service when using values-minikube.yaml" >&2
  exit 1
fi

# Production values should include gRPC ingress annotations (supports ALB and NGINX)
prod_output=$(render -f "$CHART_DIR/values-production.yaml")
if ! grep -qiE "ingress\.kubernetes\.io/backend-protocol(-version)?:[[:space:]]*\"?GRPC\"?" <<<"$prod_output"; then
  echo "Expected gRPC backend annotation in production values" >&2
  exit 1
fi

# OIDC web preset should render oauth2-proxy deployment and env markers
oidc_output=$(render -f "$CHART_DIR/values-oidc-web.yaml")
if ! grep -q "oauth2-proxy" <<<"$oidc_output"; then
  echo "Expected oauth2-proxy resources in values-oidc-web.yaml render" >&2
  exit 1
fi
if ! grep -q "OAUTH2_PROXY_PROVIDER" <<<"$oidc_output"; then
  echo "Expected OIDC environment variables in oauth2-proxy deployment" >&2
  exit 1
fi

# GitHub broker preset should render broker deployment and env vars
github_output=$(render -f "$CHART_DIR/values-github-cloud.yaml")
if ! grep -q "rocketship-auth-broker" <<<"$github_output"; then
  echo "Expected broker resources in values-github-cloud.yaml render" >&2
  exit 1
fi
if ! grep -q "ROCKETSHIP_BROKER_SIGNING_KEY_FILE" <<<"$github_output"; then
  echo "Expected broker configuration env vars in GitHub preset" >&2
  exit 1
fi
if ! grep -q "auth.rocketship.globalbank.com" <<<"$github_output"; then
  echo "Expected auth broker ingress host in GitHub preset" >&2
  exit 1
fi
