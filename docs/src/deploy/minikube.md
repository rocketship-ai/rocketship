# Run Rocketship on Minikube

Local Kubernetes cluster for development with full auth, database support, and hot-reloading via Skaffold.

## Quick Start

```bash
# 1. Create .env file with secrets (see "Configure Secrets" below)
echo "GITHUB_CLIENT_SECRET=..." > .env
echo "ROCKETSHIP_EMAIL_FROM=..." >> .env
echo "ROCKETSHIP_POSTMARK_SERVER_TOKEN=..." >> .env

# 2. Run setup script (one-time infrastructure setup)
scripts/setup-local-dev.sh

# 3. Configure local DNS
echo "127.0.0.1 auth.minikube.local" | sudo tee -a /etc/hosts

# 4. Start everything with one command
scripts/start-dev.sh

# 5. Visit http://auth.minikube.local and sign in with GitHub
```

The `start-dev.sh` script automatically:
- Starts minikube tunnel
- Starts Vite dev server for the web UI
- Runs Skaffold in dev mode for hot-reloading backend services

Make code changes and watch them automatically rebuild and redeploy!

## Prerequisites

- macOS or Linux with **6+ vCPUs, 12+ GB RAM** (for Temporal + Go builds)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/) v1.36+
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm 3](https://helm.sh/docs/intro/install/)
- [Skaffold](https://skaffold.dev/docs/install/) v2.0+
- Docker (required for Minikube driver)
- [Node.js](https://nodejs.org/) v20+ and npm (for web UI development)

## Setup

### 1. Configure Secrets

Create a `.env` file in the repository root with the following secrets:

```bash
GITHUB_CLIENT_SECRET=<github-oauth-client-secret>
ROCKETSHIP_EMAIL_FROM=<email-sender-address>
ROCKETSHIP_POSTMARK_SERVER_TOKEN=<postmark-api-token>
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

**GitHub OAuth App Setup:**

The `GITHUB_CLIENT_SECRET` corresponds to a GitHub OAuth App with these settings:

- **Client ID**: `Ov23li2GoXfR7bC7fR0y` (configured in `charts/rocketship/values-minikube-local.yaml`)
- **Authorization callback URL**: `http://auth.minikube.local/callback`

If you need to create your own OAuth app for testing:

1. Go to GitHub Settings → Developer settings → OAuth Apps → New OAuth App
2. Set **Homepage URL**: `http://auth.minikube.local`
3. Set **Authorization callback URL**: `http://auth.minikube.local/callback`
4. Copy the Client Secret to your `.env` file
5. Update `charts/rocketship/values-minikube-local.yaml` with your Client ID

### 2. Run Setup Script

```bash
scripts/setup-local-dev.sh
```

This sets up the minikube infrastructure (one-time setup):

- Minikube cluster with ingress enabled
- Temporal (workflow engine)
- PostgreSQL (database)
- All required secrets
- Vite relay for web UI development

**Note:** This script does NOT deploy Rocketship services. Skaffold handles that for hot-reloading.

### 3. Configure Local DNS

Add to `/etc/hosts`:

```bash
echo "127.0.0.1 auth.minikube.local" | sudo tee -a /etc/hosts
```

**Why needed:** This ensures:
- Your browser sends `Host: auth.minikube.local` header (matches ingress rule)
- Stable origin for cookies (`auth.minikube.local` instead of `localhost`)
- OAuth callback URLs and JWT issuer URLs are consistent

## Usage

### Quick Start - All-in-One Script

```bash
scripts/start-dev.sh
```

This automatically starts:
1. Minikube tunnel (binds ingress to `127.0.0.1`)
2. Vite dev server for web UI
3. Skaffold in development mode (watches code, rebuilds on changes)

Visit `http://auth.minikube.local` and sign in with GitHub!

**Hot Reloading:** Edit any Go file in `cmd/engine/`, `cmd/worker/`, `cmd/authbroker/`, or `internal/`, save, and Skaffold will automatically:
- Rebuild the Docker image
- Redeploy to Kubernetes
- Stream logs to your terminal

Press `Ctrl+C` to stop all processes.

### Manual Development Workflow

If you prefer to run components separately:

1. Start minikube tunnel (separate terminal):
   ```bash
   sudo minikube tunnel -p rocketship
   ```

2. Start Vite dev server (separate terminal):
   ```bash
   cd web && npm run dev
   ```

3. Run Skaffold (watches for changes and rebuilds):
   ```bash
   # Standard mode (all traffic through minikube tunnel)
   skaffold dev

   # Debug mode (with verbose logging)
   skaffold dev -p debug
   ```

### CLI Without Auth

If you just need CLI access without web UI:

```bash
# Port-forward the engine
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700 &

# Configure CLI
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube

# Run tests
rocketship run -af examples/simple-http/rocketship.yaml
```

### CLI With Auth

```bash
rocketship login
# Follow GitHub device flow prompts
```

### Web App

The web UI is served through the same ingress as the auth broker (single-origin architecture):

- UI: `http://auth.minikube.local/`
- Auth API: `http://auth.minikube.local/api`, `/authorize`, `/token`, `/callback`
- Engine API: `http://auth.minikube.local/engine`

**How it works:**
- The `vite-relay` deployment proxies requests to your local Vite server
- All services share the same origin for cookie-based auth
- Hot Module Replacement (HMR) still works through the ingress

## Customization

Override defaults via environment variables:

| Variable               | Default                   | Description           |
| ---------------------- | ------------------------- | --------------------- |
| `MINIKUBE_PROFILE`     | `rocketship`              | Minikube profile name |
| `ROCKETSHIP_NAMESPACE` | `rocketship`              | Kubernetes namespace  |
| `TEMPORAL_NAMESPACE`   | `rocketship`              | Temporal namespace    |
| `POSTGRES_PASSWORD`    | `rocketship-dev-password` | Postgres password     |

Example:

```bash
ROCKETSHIP_NAMESPACE=testing scripts/setup-local-dev.sh
```

### Skaffold Configuration

The `skaffold.yaml` file defines what gets built and deployed:

- **Artifacts**: Three Docker images (engine, worker, authbroker)
- **Deploy**: Helm-based deployment with values from `charts/rocketship/values-minikube-local.yaml`
- **Profiles**:
  - `debug`: Enables debug logging for all services
  - `no-port-forward`: Disables port forwarding (use with minikube tunnel)

Edit `skaffold.yaml` to customize build or deployment behavior.

## Troubleshooting

**Skaffold build fails**: Ensure Docker is using minikube's daemon:
```bash
eval "$(minikube -p rocketship docker-env)"
```

**Pods not ready**: Wait 2-3 minutes for Temporal and Postgres to initialize.

**Auth broker fails**: Check `.env` has valid `GITHUB_CLIENT_SECRET` and `ROCKETSHIP_POSTMARK_SERVER_TOKEN`.

**Skaffold not watching files**: Ensure you're in the repository root when running `skaffold dev`.

**Changes not deploying**: Check Skaffold output for build errors. Press `Ctrl+C` and restart if needed.

**Web UI not loading**:

1. Ensure web dependencies are installed: `cd web && npm install`
2. Verify Vite is running: `curl http://localhost:5173`
3. Check vite-relay can reach host:
   ```bash
   kubectl run -n rocketship test-vite --rm -it --image=busybox:1.36 --restart=Never -- \
     wget -qO- http://192.168.64.1:5173/ | head -n 5
   ```
   If this fails, check macOS firewall settings for Node/Vite on port 5173.

**Install script warnings about host connectivity**: The install script tests connectivity to your host's Vite server to auto-detect the correct IP. If Vite isn't running yet, you'll see warnings like "Could not detect reachable host IP" - this is expected and the script will use a sensible default (192.168.64.1). Start Vite after installation completes.

**Cookies not working**: Ensure you're accessing the UI via `http://auth.minikube.local` (not `http://localhost:5173`). The single-origin architecture requires UI, auth broker, and engine to be served from the same host for cookies to work.

**Engine API not accessible**: Verify the ingress includes the `/engine` path:

```bash
kubectl get ingress rocketship-gateway -n rocketship -o jsonpath='{.spec.rules[0].http.paths[*].path}'
```

Should show: `/api /authorize /token /callback /logout /engine /`

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
