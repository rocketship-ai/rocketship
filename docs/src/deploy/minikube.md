# Run Rocketship on Minikube

Local Kubernetes cluster for development with full auth and database support.

## Prerequisites

- macOS or Linux with 4+ vCPUs, 8+ GB RAM
- [Minikube](https://minikube.sigs.k8s.io/docs/start/) v1.36+
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm 3](https://helm.sh/docs/intro/install/)
- Docker

## Setup

### 1. Configure Secrets

Copy `.env.example` to `.env`:

```bash
cp .env.example .env
```

**Option A: Extract from production cluster** (requires kubectl access to `do-nyc1-rs-test-project`):

```bash
kubectl config use-context do-nyc1-rs-test-project
kubectl get secret rocketship-github-oauth -n rocketship -o jsonpath='{.data.ROCKETSHIP_GITHUB_CLIENT_SECRET}' | base64 -d
kubectl get secret rocketship-postmark-secret -n rocketship -o jsonpath='{.data.ROCKETSHIP_EMAIL_FROM}' | base64 -d
kubectl get secret rocketship-postmark-secret -n rocketship -o jsonpath='{.data.ROCKETSHIP_POSTMARK_SERVER_TOKEN}' | base64 -d
kubectl config use-context rocketship
```

**Option B: Get from team member** (if you don't have cluster access):

Ask a team member with cluster access to share their `.env` file or provide the three required values.

Fill in your `.env` file:

```bash
GITHUB_CLIENT_SECRET=<your-value>
ROCKETSHIP_EMAIL_FROM=<your-value>
ROCKETSHIP_POSTMARK_SERVER_TOKEN=<your-value>
```

### 2. Run Install Script

```bash
scripts/install-minikube.sh
```

This creates a minikube cluster with:
- Temporal (workflow engine)
- Rocketship Engine (gRPC server)
- Rocketship Worker (test runner)
- Rocketship Auth Broker (OAuth)
- PostgreSQL (database)

### 3. Set Up Local Access

**For CLI (engine only):**

```bash
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
```

**For Auth (CLI + web app):**

1. Add to `/etc/hosts`:
   ```bash
   echo "127.0.0.1 auth.minikube.local" | sudo tee -a /etc/hosts
   ```

2. Run minikube tunnel (separate terminal, keep running):
   ```bash
   sudo minikube tunnel -p rocketship
   ```

3. Verify auth broker:
   ```bash
   curl http://auth.minikube.local/.well-known/jwks.json
   ```

## Usage

### CLI Without Auth

```bash
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube
rocketship run -af examples/simple-http/rocketship.yaml
```

### CLI With Auth

```bash
rocketship login
# Follow GitHub device flow prompts
```

### Web App

1. Update `web/.env.local`:
   ```
   VITE_API_URL=http://auth.minikube.local
   ```

2. Start dev server:
   ```bash
   cd web && npm run dev
   ```

3. Visit `http://localhost:5173` and sign in

## Customization

Override defaults via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MINIKUBE_PROFILE` | `rocketship` | Minikube profile name |
| `ROCKETSHIP_NAMESPACE` | `rocketship` | Kubernetes namespace |
| `TEMPORAL_NAMESPACE` | `rocketship` | Temporal namespace |
| `POSTGRES_PASSWORD` | `rocketship-dev-password` | Postgres password |

Example:

```bash
ROCKETSHIP_NAMESPACE=testing scripts/install-minikube.sh
```

## Troubleshooting

**Pods not ready**: Wait 2-3 minutes for Temporal and Postgres to initialize.

**Auth broker fails**: Check `.env` has valid `GITHUB_CLIENT_SECRET` and `ROCKETSHIP_POSTMARK_SERVER_TOKEN`.

**Can't connect to engine**: Ensure port-forward is running on port 7700.

**Auth broker unreachable from inside cluster**: The install script automatically detects the ingress controller's ClusterIP and configures engine hostAliases. If this failed, manually get the IP and redeploy:

```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller -o jsonpath='{.spec.clusterIP}'
helm upgrade rocketship charts/rocketship --namespace rocketship \
  --values charts/rocketship/values-minikube-local.yaml \
  --set temporal.host=temporal-frontend.rocketship:7233 \
  --set temporal.namespace=rocketship \
  --set "engine.hostAliases[0].ip=<YOUR-IP>" --wait
```

**Auth broker unreachable from local machine**: Ensure `/etc/hosts` entry exists and `sudo minikube tunnel` is running.

## Cleanup

```bash
helm uninstall rocketship temporal -n rocketship
kubectl delete namespace rocketship
minikube delete -p rocketship
```
