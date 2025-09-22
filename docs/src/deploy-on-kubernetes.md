# Deploying Rocketship on Kubernetes

This guide shows how to bring up a complete Rocketship stack—Temporal + Rocketship Engine/Worker—on Kubernetes. The fastest path is to use the provided Minikube script, but you can also install the Helm charts manually.

## Prerequisites

- Kubernetes cluster (Minikube, kind, or managed Kubernetes)
- `kubectl` configured against the cluster
- Helm v3
- Docker Hub access (to pull `rocketshipai/rocketship-*` images)

## Quick Start (Minikube Script)

```bash
# From the repository root
scripts/install-minikube.sh
```

The script will:

1. Start a Minikube profile (defaults to `rocketship`).
2. Build local `rocketship-engine` and `rocketship-worker` images inside the Minikube Docker runtime.
3. Install Temporal using the official Temporal Helm chart with a lightweight configuration suitable for local testing.
4. Install (or upgrade) the Rocketship Helm chart, wiring the engine/worker to the Temporal frontend service.

Environment variables allow customisation:

| Variable              | Default            | Description                                   |
| --------------------- | ------------------ | --------------------------------------------- |
| `MINIKUBE_PROFILE`    | `rocketship`       | Minikube profile name                         |
| `TEMPORAL_NAMESPACE`  | `rocketship`       | Namespace for the Temporal release            |
| `TEMPORAL_RELEASE`    | `temporal`         | Helm release name for Temporal                |
| `ROCKETSHIP_NAMESPACE`| `rocketship`       | Namespace for Rocketship                      |
| `ROCKETSHIP_RELEASE`  | `rocketship`       | Helm release name for Rocketship              |
| `TEMPORAL_WORKFLOW_NAMESPACE` | `default` | Temporal logical namespace used by Rocketship (set to `default` unless you register your own) |
| `ROCKETSHIP_CHART_PATH` | `charts/rocketship` | Path to the Rocketship chart                 |

Example: install everything into a single namespace called `testing`:

```bash
TEMPORAL_NAMESPACE=testing \
ROCKETSHIP_NAMESPACE=testing \
scripts/install-minikube.sh
```

At the end, the script prints port-forward commands for both Temporal and the Rocketship engine.

## Manual Installation

### 1. Install Temporal

```bash
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

helm install temporal temporal/temporal \
  --namespace rocketship --create-namespace \
  --set server.replicaCount=1 \
  --set cassandra.config.cluster_size=1 \
  --set elasticsearch.replicas=1 \
  --set prometheus.enabled=false \
  --set grafana.enabled=false \
  --wait --timeout 15m
```

This deploys Temporal with the baked-in dependencies (Cassandra, Elasticsearch) in minimal mode. Adjust the values for production (larger replicas, external databases, metrics, etc.).

### 2. Install Rocketship

```bash
helm install rocketship charts/rocketship \
  --namespace rocketship --create-namespace \
  --set temporal.host="temporal-frontend.rocketship:7233" \
  --set temporal.namespace="default" \
  --wait

> **Tip:** When testing locally (e.g., with Minikube) build engine/worker images inside the cluster and override `engine.image.*` / `worker.image.*` via `--set` so the chart uses those local tags.
```

The `temporal.host` value must point at the Temporal frontend service. If you installed Temporal with a different release or namespace, update the hostname accordingly (`<release>-frontend.<namespace>:7233`).

### 3. Validate

```bash
kubectl get pods --namespace rocketship
kubectl get pods --namespace rocketship
```

You should see the Temporal services along with `rocketship-engine` and `rocketship-worker` pods.

To exercise the stack locally:

```bash
# Port-forward Rocketship engine gRPC endpoint
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700

# Port-forward Temporal frontend (optional)
kubectl port-forward -n temporal svc/temporal-frontend 7233:7233

# In another terminal, run tests (uses default gRPC port 7700)
rocketship run -af examples/simple-http/rocketship.yaml
```

## Helm Chart Overview

The Rocketship Helm chart (`charts/rocketship`) contains:

- `rocketship-engine` Deployment + Service (named ports `grpc` and `http`).
- `rocketship-worker` Deployment.
- Optional Ingress configuration for exposing the engine over gRPC (use the production values file for ALB annotations).
- Minimal default settings for CPU/memory; adjust via `values.yaml` as needed.

Key values:

| Value                    | Description                                                |
| ------------------------ | ---------------------------------------------------------- |
| `temporal.host`          | Temporal frontend host:port used by engine & worker        |
| `engine.image.*`         | Container image/pull policy for the engine                 |
| `worker.image.*`         | Container image/pull policy for the worker                 |
| `engine.service.type`    | Service type (`ClusterIP` by default, `NodePort` for minikube)|
| `ingress.*`              | Enable and configure ingress                               |

Additional values files:

- `values-minikube.yaml`: switches the engine service to `NodePort`.
- `values-production.yaml`: enables an Ingress with AWS ALB gRPC annotations.

## Teardown

```bash
helm uninstall rocketship -n rocketship
helm uninstall temporal -n temporal
kubectl delete namespace rocketship temporal
minikube delete -p rocketship  # optional
```

## Next Steps

- Integrate with an external Temporal installation by disabling the bundled dependencies and pointing `temporal.host` to your service.
- Enable metrics/monitoring by wiring Prometheus/Grafana in the Temporal install and exposing the Rocketship health endpoint.
- Configure ingress TLS and authentication as needed for your environment.
