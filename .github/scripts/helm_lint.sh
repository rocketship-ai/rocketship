#!/usr/bin/env bash
set -euo pipefail

CHART_DIR="charts/rocketship"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is required but not installed" >&2
  exit 1
fi

helm lint "$CHART_DIR"
helm lint "$CHART_DIR" -f "$CHART_DIR/values-minikube.yaml"
helm lint "$CHART_DIR" -f "$CHART_DIR/values-production.yaml"
