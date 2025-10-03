# Deploy Rocketship on Your Cloud

Rocketship can run anywhere Kubernetes is available. The CLI embeds the engine and worker binaries for local auto mode, but real deployments separate the components and connect them to a Temporal cluster.

This section outlines the supported deployment paths and what each delivers so you can pick the right starting point.

## Core Components

Every deployment provisions:

1. **Temporal** – Durable workflow orchestration. The Helm chart from Temporal provides a ready-made stack with Cassandra, Elasticsearch, and UI components for development and staging clusters.
2. **Rocketship Engine** – gRPC API that accepts suite executions, manages profiles, and streams results.
3. **Rocketship Worker** – Executes plugin steps inside Temporal workflows.

Both Rocketship services require the Temporal frontend host and namespace; everything else (ingress, TLS, auth) is layered on top through Kubernetes objects.

## Deployment Paths

| Scenario | Guide | Highlights |
| --- | --- | --- |
| Local iteration | [Run on Minikube](deploy/minikube.md) | Single script (`scripts/install-minikube.sh`) that starts Minikube, installs Temporal, builds local engine/worker images, and deploys the Rocketship chart. Great for fast feedback and integration testing inside CI. |
| Production-ready proof of concept | [Deploy on DigitalOcean Kubernetes](deploy/digitalocean.md) | Walks through preparing a managed cluster, wiring an NGINX ingress with TLS, publishing custom images to DigitalOcean Container Registry, and installing the Rocketship + Temporal Helm releases. |
| Web UI with OIDC front-door | [Deploy on DigitalOcean Kubernetes](deploy/digitalocean.md#7-enable-auth-for-the-web-ui-optional) | Layer oauth2-proxy + NGINX annotations. Use `values-github-selfhost.yaml` + `values-github-web.yaml` for GitHub device-flow reuse, or `values-oidc-web.yaml` to integrate with an external IdP. |

> Looking for another cloud? The DigitalOcean flow covers all building blocks: registry authentication, TLS secrets, ingress, and chart overrides. Adapt the same pattern for EKS, GKE, AKS, or on-prem clusters by swapping provider-specific commands.

## After Deployment

- Use `rocketship profile create` and `rocketship profile use` to store the engine endpoint (`grpcs://…`) and default to TLS where appropriate.
- Inject a secret token into the engine (`ROCKETSHIP_ENGINE_TOKEN`) and export the same value as `ROCKETSHIP_TOKEN` in the CLI/CI environment so gRPC calls are authenticated.
- Alternatively, set `ROCKETSHIP_AUTH_MODE=oidc` with the issuer/client metadata so the engine validates JWTs. Team members then run `rocketship login` (device flow) or `rocketship status` to manage their own credentials.
- When enabling the GitHub auth broker, provision a Postgres instance and pass the DSN via `auth.broker.database.secretName`/`secretKey`, and generate a 32-byte base64 `ROCKETSHIP_BROKER_REFRESH_KEY` secret. The Helm presets reference the `globalbank-auth-broker-database` and `globalbank-auth-broker-secrets` names shown in the DigitalOcean guide.
- Run suites with `rocketship run --engine`. When profiles are active, the CLI resolves the engine address automatically.
- Expose Prometheus/Grafana, RBAC, and authentication once the core stack is stable (tracked for future epics).

Once the core stack is running, you can optionally apply the OIDC preset to front any HTTP/UI endpoints with your IdP before traffic reaches Rocketship.

> First-time logins on a fresh cluster return access tokens with a `pending` role. Use the bearer token to call `POST /api/orgs` on the auth broker to create the first organisation/project, or invite the user via the forthcoming org management endpoints.
