# Run Rocketship on Minikube

Use this guide when you want a fully isolated Rocketship stack on your laptop for development, demos, or CI automation. The repository ships with `scripts/install-minikube.sh`, which provisions Temporal plus the Rocketship engine and worker in a single namespace.

## Prerequisites

- macOS or Linux host with at least 4 vCPUs and 8 GB RAM free
- [Minikube](https://minikube.sigs.k8s.io/docs/start/) `v1.36+`
- [Kubectl](https://kubernetes.io/docs/tasks/tools/) pointed to your Minikube context
- [Helm 3](https://helm.sh/docs/intro/install/)
- Docker (Minikube uses the local Docker daemon to build images)

## 1. Launch the Stack

From the repository root:

```bash
scripts/install-minikube.sh
```

The script performs the following steps:

1. Starts/ensures a Minikube profile (defaults to `rocketship`).
2. Switches the Docker context to Minikube and builds the engine and worker images as `rocketship-engine-local:dev` and `rocketship-worker-local:dev`.
3. Installs Temporal via the official Helm chart in the `rocketship` namespace using a minimal configuration (single replica services, Cassandra + Elasticsearch, no Prometheus/Grafana).
4. Registers the Temporal logical namespace `rocketship` using the Temporal CLI.
5. Installs the Rocketship Helm chart, pointing at the Temporal frontend service, and exposes ClusterIP services with named `grpc`/`http` ports.
6. Prints summary information and port-forward helper commands.

Typical output ends with:

```
Engine endpoint (gRPC): service rocketship-engine.rocketship:7700
Health endpoint: service rocketship-engine.rocketship:7701
Temporal host configured as: temporal-frontend.rocketship:7233
```

## 2. Customise the Install (Optional)

Environment variables let you adjust namespaces, release names, and Temporal workflow namespace without editing the script:

| Variable | Default | Purpose |
| --- | --- | --- |
| `MINIKUBE_PROFILE` | `rocketship` | Minikube profile name |
| `ROCKETSHIP_NAMESPACE` | `rocketship` | Kubernetes namespace for Rocketship services |
| `TEMPORAL_NAMESPACE` | `rocketship` | Namespace for Temporal Helm release |
| `TEMPORAL_WORKFLOW_NAMESPACE` | `rocketship` | Temporal logical namespace registered via CLI |
| `TEMPORAL_RELEASE` | `temporal` | Helm release name for Temporal |
| `ROCKETSHIP_RELEASE` | `rocketship` | Helm release name for Rocketship |
| `ROCKETSHIP_CHART_PATH` | `charts/rocketship` | Path to the chart (override when packaging) |

Example: run everything inside a namespace called `testing`:

```bash
ROCKETSHIP_NAMESPACE=testing \
TEMPORAL_NAMESPACE=testing \
TEMPORAL_WORKFLOW_NAMESPACE=testing \
scripts/install-minikube.sh
```

## 3. Verify the Deployment

```bash
kubectl get pods -n rocketship
kubectl get svc -n rocketship
```

You should see Temporal pods (frontend, history, matching, worker, cassandra, elasticsearch) and two Rocketship deployments (`rocketship-engine`, `rocketship-worker`).

To exercise the stack locally:

```bash
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube
rocketship run -af examples/simple-http/rocketship.yaml
```

## 4. Manual Helm Flow (Optional)

If you prefer to perform the steps yourself:

1. Install Temporal:
   ```bash
   helm repo add temporal https://go.temporal.io/helm-charts
   helm repo update
   helm install temporal temporal/temporal \
     --version 0.66.0 \
     --namespace rocketship --create-namespace \
     --set server.replicaCount=1 \
     --set cassandra.config.cluster_size=1 \
     --set elasticsearch.replicas=1 \
     --set prometheus.enabled=false \
     --set grafana.enabled=false \
     --wait --timeout 15m
   ```
2. Register the Temporal namespace (default `rocketship`):
   ```bash
   kubectl exec -n rocketship deploy/temporal-admintools -- \
     temporal operator namespace create --namespace rocketship
   ```
3. Build local images and load them into Minikube:
   ```bash
   eval "$(minikube docker-env)"
   docker build -f .docker/Dockerfile.engine -t rocketship-engine-local:dev .
   docker build -f .docker/Dockerfile.worker -t rocketship-worker-local:dev .
   ```
4. Install Rocketship using overrides:
   ```bash
   helm install rocketship charts/rocketship \
     --namespace rocketship \
     --set temporal.host=temporal-frontend.rocketship:7233 \
     --set temporal.namespace=rocketship \
     --set engine.image.repository=rocketship-engine-local \
     --set engine.image.tag=dev \
     --set worker.image.repository=rocketship-worker-local \
     --set worker.image.tag=dev \
     --values charts/rocketship/values-minikube.yaml \
     --wait
   ```

## 5. Cleanup

```bash
helm uninstall rocketship -n rocketship
helm uninstall temporal -n rocketship
kubectl delete namespace rocketship
minikube delete -p rocketship
```

You now have a repeatable local environment for building new features, running integration tests, or debugging plugin behaviour before deploying to a managed cluster.
