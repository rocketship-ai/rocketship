# Deploy Rocketship on Your Cloud

Rocketship supports three deployment modes. Choose based on your needs:

1. **Local Processes** – Quick experiments without Kubernetes
2. **Minikube Stack** – Isolated local Kubernetes cluster for development
3. **Cloud Kubernetes** – Production deployments on any Kubernetes cluster

## Deployment Modes

### 1. Local Processes (Quick Start)

Best for: Quick testing, development, learning

The CLI embeds engine and worker binaries for standalone operation:

```bash
# Auto-start engine, run tests, auto-stop
rocketship run -af test.yaml

# Or manually manage the engine
rocketship start server -b      # Background engine
rocketship run test.yaml        # Run tests
rocketship stop server          # Stop engine
```

**Requirements:** Temporal installed locally (`brew install temporal`)

### 2. Minikube Stack (Local Kubernetes)

Best for: Development, CI testing, isolated environments

Single script provisions everything in an isolated Minikube cluster:

```bash
scripts/setup-local-dev.sh  # One-time infrastructure setup
scripts/start-dev.sh         # Start services with hot-reloading

# Or for non-development deployments, use Helm directly:
# helm install rocketship charts/rocketship -n rocketship
# kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship profile create minikube grpc://localhost:7700
rocketship profile use minikube
rocketship run -f test.yaml
```

**Guide:** [Run on Minikube](deploy/minikube.md)

### 3. Cloud Kubernetes (Production)

Best for: Production deployments, team collaboration, test history

Deploy to any Kubernetes cluster (EKS, GKE, AKS, DigitalOcean, on-prem):

- Full Temporal stack with persistence
- OIDC authentication for CLI and web UI
- Persistent test run history
- Scalable worker pools

**Guides:**
- [Deploy on DigitalOcean](deploy/digitalocean.md) – Step-by-step production guide
- [DigitalOcean with Web UI](deploy/digitalocean.md#7-enable-auth-for-the-web-ui-optional) – Add OIDC authentication

Adapt the DigitalOcean pattern for other clouds by swapping provider-specific commands.

## Core Components

Cloud deployments provision:

1. **Temporal** – Durable workflow orchestration (Helm chart)
2. **Rocketship Engine** – gRPC API accepting test executions
3. **Rocketship Worker** – Executes plugin steps in Temporal workflows
4. **Auth Broker** (optional) – OIDC authentication for CLI/web UI
5. **PostgreSQL** (optional) – Test run history and auth persistence

## Using Cloud Deployments

After deploying, create a profile and authenticate:

```bash
# Create profile
rocketship profile create production https://rocketship.company.com

# Authenticate via OIDC device flow
rocketship login --profile production

# Use the profile
rocketship profile use production

# Run tests
rocketship run -f test.yaml
```
