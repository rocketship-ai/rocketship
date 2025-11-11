# Run Rocketship on Minikube

Local Kubernetes cluster for development with full auth and database support.

## Quick Start

```bash
# 1. Create .env file with secrets (see "Configure Secrets" below)
echo "GITHUB_CLIENT_SECRET=..." > .env
echo "ROCKETSHIP_EMAIL_FROM=..." >> .env
echo "ROCKETSHIP_POSTMARK_SERVER_TOKEN=..." >> .env

# 2. Run install script
scripts/install-minikube.sh

# 3. Configure local access
echo "127.0.0.1 auth.minikube.local" | sudo tee -a /etc/hosts
sudo minikube tunnel -p rocketship  # Keep running in separate terminal

# 4. Set up and start web UI
cd web && npm install && npm run dev  # Keep running in separate terminal

# 5. Visit http://auth.minikube.local and sign in with GitHub
```

## Prerequisites

- macOS or Linux with 4+ vCPUs, 8+ GB RAM
- [Minikube](https://minikube.sigs.k8s.io/docs/start/) v1.36+
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm 3](https://helm.sh/docs/intro/install/)
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

   **Why needed:** This ensures:

   - Your browser sends `Host: auth.minikube.local` header (matches ingress rule)
   - Stable origin for cookies (`auth.minikube.local` instead of `localhost`)
   - OAuth callback URLs and JWT issuer URLs are consistent
   - Future HTTPS/TLS setup is easier (mkcert for `*.minikube.local`)

2. Run minikube tunnel (separate terminal, keep running):

   ```bash
   sudo minikube tunnel -p rocketship
   ```

   This binds the ingress LoadBalancer to `127.0.0.1`.

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

The web UI is served through the same ingress as the auth broker (single-origin architecture) to enable cookie-based authentication.

1. Install web dependencies (first time only):

   ```bash
   cd web && npm install
   ```

2. Start Vite dev server (must listen on all interfaces):

   ```bash
   npm run dev
   ```

   Vite is configured in `vite.config.ts` to:

   - Listen on `0.0.0.0:5173` (reachable from minikube pods)
   - Accept requests from `auth.minikube.local`
   - Enable HMR through the ingress

3. Visit `http://auth.minikube.local` and sign in with GitHub

**How it works:**

- The `vite-relay` deployment in the cluster proxies requests from the ingress to your local Vite server
- UI (`/`), Auth API (`/api`, `/authorize`, etc.), and Engine API (`/engine`) are all served from `auth.minikube.local`
- Same-origin = cookies work automatically, HMR still functions
- Frontend can call engine with: `fetch('/engine/health', { credentials: 'include' })`

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
ROCKETSHIP_NAMESPACE=testing scripts/install-minikube.sh
```

## Troubleshooting

**Pods not ready**: Wait 2-3 minutes for Temporal and Postgres to initialize.

**Auth broker fails**: Check `.env` has valid `GITHUB_CLIENT_SECRET` and `ROCKETSHIP_POSTMARK_SERVER_TOKEN`.

**Can't connect to engine**: Ensure port-forward is running on port 7700.

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
