# Rocketship Kubernetes & Discovery — Master Plan (v1)

This plan delivers a production‑ready, Kubernetes‑hostable Rocketship server that the CLI can connect to securely, without over‑complexity. It ships incrementally via small PRs, each with targeted validation. We avoid CLI Kubernetes wrappers in v1 (no `rocketship kube`) to keep surface area small; Helm and kubectl remain the primary deployment tools. Enterprise needs (e.g., GlobalBank) are supported through a Helm chart and TLS/auth upgrades. RBAC is intentionally deferred to a later epic.

## Guiding Principles

- Minimal v1 surface: profiles, TLS, discovery, Helm chart, health checks.
- Keep CLI simple: no `rocketship kube` in v1; rely on Helm/kubectl.
- Backward compatible: local `--auto` and existing flags continue to work.
- Progressive auth: token first (great for CI/CD and self‑hosted), then OIDC device flow for CLI (RFC 8628); RBAC later.
- Respect configuration precedence: explicit flags > env vars (e.g., `ROCKETSHIP_TOKEN`) > active profile > defaults.
- Testable increments: add CI scripts/workflows as each feature lands.

## Outcome Overview

- Users deploy Engine + Worker on Kubernetes via Helm.
- Ingress controllers (NGINX, cloud load balancers) supported; HTTP health endpoint exposed; gRPC over TLS supported.
- CLI can connect to `grpcs://` endpoints via profiles or `--engine`.
- Server discovery tells the CLI which auth mode is active; CLI adapts.
- Optional ingress with OIDC (via oauth2-proxy) for web endpoints; CLI remains token/JWT‑based.


## Test Resources (For Manual/E2E Verification)

These resources are available for manual validation where applicable:

- Domain/TLS: `globalbank.rocketship.sh` with a ZeroSSL certificate
  - Use to verify Ingress TLS and gRPC over TLS.
  - Create a Kubernetes TLS secret (e.g., `globalbank-rocketship-tls`) with the ZeroSSL cert and key.
  - Point DNS for `globalbank.rocketship.sh` (or subdomain) to the ingress/ALB.

- IdP: Auth0 tenant/account
  - Use to validate OIDC in two places when we reach them:
    1) OIDC at ingress for web endpoints (PR 4, optional)
    2) OIDC Device Flow for CLI login (PR 7, optional/phase‑next)


---

## PR 1 — TLS‑Aware Engine Dialing + Profiles Polishing

What it accomplishes
- Enables `rocketship` to connect to remote engines over TLS using `--engine https://host[:port]` or via profiles with TLS metadata.
- Preserves localhost behavior; `--auto` remains unchanged.

Changes
- `internal/cli/client.go`: resolve URL schemes (`http/https/grpc/grpcs`) and set dial creds; SNI set from profile TLS domain.
- Logging polish and clearer error messages during dial/discovery.

Tests (unit)
- `go test ./...` additions:
  - Address resolution and TLS options (table‑driven).
  - Profile resolution precedence (explicit flag > profile > default).

CI integration
- Workflow: `.github/workflows/go-tests.yml` (runs unit tests on PR).

---

## PR 2 — Engine HTTP Health Endpoint

What it accomplishes
- Adds `/healthz` on engine HTTP port (7701) for ALB/NLB health checks and ops diagnostics.

Changes
- Small HTTP server in `cmd/engine` with `/` and `/healthz` returning 200 and minimal JSON.

Tests (unit)
- Handler returns 200 and valid payload.

CI integration
- Extend `go-tests.yml` (no network), cover handler logic.

---

## PR 3 — Helm Chart (Engine + Worker)

What it accomplishes
- Helm chart for deploying Rocketship in Kubernetes (Engine, Worker, Services, optional Ingress).
- Supports minikube (NodePort) and production (Ingress) via separate values files.

Changes
- `helm/rocketship/` chart with:
  - `Deployment` and `Service` for engine and worker
  - Optional `Ingress` (annotations for gRPC, health path)
  - `values-minikube.yaml`, `values-production.yaml`

Manual test prerequisites (optional)
- If validating TLS with a real hostname, create a TLS secret `globalbank-rocketship-tls` from the ZeroSSL cert and set `ingress.tls.secretName=globalbank-rocketship-tls` in values.
- Create a DNS record for `globalbank.rocketship.sh` (or a subdomain) pointing to the ingress/ALB/NLB.

Tests (templating)
- Scripts:
  - `.github/scripts/helm_lint.sh` — helm lint + schema checks
  - `.github/scripts/helm_template_check.sh` — render with minikube/production values and assert on key fields (port name `grpc`, health path `/healthz`, selectors)

CI integration
- Workflow: `.github/workflows/helm-chart.yml` running the two scripts on PR.

---

## PR 4 — Ingress Presets (gRPC baseline + optional OIDC web)

What it accomplishes
- Document the existing NGINX gRPC values as the production baseline (DigitalOcean-ready).
- Provide an additional values file for enabling OIDC at the ingress for HTTP/web endpoints via oauth2-proxy (leaves gRPC untouched).

Changes
- `charts/rocketship/values-production.yaml`: clarify it targets NGINX gRPC ingress (already in place).
- `charts/rocketship/values-oidc-web.yaml`: add oauth2-proxy deployment and NGINX annotations for OIDC (issuer, client ID/secret, scopes, cookie name).

Tests (templating)
- Update `helm_template_check.sh` to render `values-oidc-web.yaml` and assert OIDC annotations and oauth2-proxy env are present.

CI integration
- Reuse the Helm template check workflow; add the new values file to the matrix.

Manual test prerequisites (optional)
- Use the `globalbank.rocketship.sh` certificate via `globalbank-rocketship-tls`.
- Configure oauth2-proxy with Auth0 (or other IdP) credentials and verify browser login gates `/healthz` (gRPC remains token-based in later PRs).

---

## PR 5 — Discovery v2 (Capabilities & Version) [Optional in v1]

What it accomplishes
- Extends the discovery RPC to return server version, capability flags, preferred endpoints; CLI displays info in `profile show`.

Changes
- `proto/engine.proto`: add `GetServerInfo` returning version, capability flags, and advertised `auth_type` values: `none`, `token`, `oidc`, `cloud`.
- Server returns static values initially. `GetAuthConfig` remains for backward compatibility.

Tests (unit)
- Proto gen compiles; client displays info when the RPC is available.

CI integration
- Covered by go tests workflow.

---

## PR 6 — Token Auth v1 (Enterprise‑Friendly)

What it accomplishes
- Enables bearer token auth for gRPC: simple, CLI‑friendly, and ingress‑agnostic.

Changes
- Engine: gRPC unary interceptor validating tokens against a secret or pluggable verifier. Support token prefix conventions for clarity (e.g., `rst_self_...` for self‑hosted; `rst_cloud_...` reserved for cloud).
- CLI: attach token from `ROCKETSHIP_TOKEN` to gRPC metadata. Keep v1 minimal by not persisting tokens; profile‑stored tokens can be a v2 enhancement.

Tests (unit)
- Interceptor accept/deny paths.
- Client header injection from env var.

CI integration
- Extend `go-tests.yml` for auth tests.

---

## PR 7 — OIDC Device Flow Login (CLI) [Phase‑Next]

What it accomplishes
- CLI obtains and stores OIDC tokens using OAuth 2.0 Device Authorization Grant (RFC 8628), ideal for CLI/SSH/headless environments. Uses JWT as bearer for gRPC. Suitable for enterprise IdPs and future cloud.

Changes
- `rocketship login|logout|status` commands; device code UX (copy/paste code + URL). Token storage via OS keyring (with file fallback). Automatic refresh for long‑lived sessions when refresh token provided.
- Engine validates JWTs via JWKS (caching with periodic refresh); discovery advertises `auth_type: oidc` or `cloud`.

Tests (unit)
- Token storage and state machine; mocked device flow; JWT validation with test keys; JWKS cache behavior.

CI integration
- Add a non‑network test suite; E2E guarded in a separate workflow if secrets are provided.

Manual test prerequisites (optional)
- Configure an Auth0 application that permits Device Authorization Flow.
- Provide device authorization, token, and issuer endpoints to the engine/CLI configuration for testing.

---

## PR 8 — Docs & Examples

What it accomplishes
- Documentation for Kubernetes deployment with Helm, minikube quick start, TLS/auth configuration, and profile usage.

Changes
- Update `docs/src/deploy-on-kubernetes.md` to Helm‑first flow.
- Add “Ingress with OIDC at ALB (Web)” and “gRPC over ALB/NLB” sections.

Tests
- Documentation build in CI.

CI integration
- Workflow: `.github/workflows/docs.yml` building docs on PR.

---

## Why No `rocketship kube` in v1?

- Reduces maintenance burden and user confusion (Helm/kubectl are standard and powerful).
- Keeps CLI focused on test execution and server discovery/auth.
- We can revisit a `kube` group later if we identify high‑value wrappers (e.g., `connect` that reads ingress/service to auto‑create a profile).

---

## Integration Testing Details (Scripts & Workflows)

Planned scripts under `.github/scripts/`:

- `helm_lint.sh`
  - Installs helm (if needed), runs `helm lint helm/rocketship`.

- `helm_template_check.sh`
  - Renders chart with: default, `values-minikube.yaml`, `values-production.yaml`, `values-grpc.yaml`, `values-oidc-web.yaml`.
  - Uses `yq`/`jq`/`grep` assertions:
    - Engine service has port name `grpc` and targetPort 7700
    - Health path `/healthz` appears in ingress annotations when enabled
    - Worker deployment has correct labels/selectors

- `go_unit.sh`
  - Runs `go test ./...` with race; caches modules.

Workflows under `.github/workflows/`:

- `go-tests.yml`
  - Triggers on PR; runs `go_unit.sh`.

- `helm-chart.yml`
  - Triggers on PR; sets up helm; runs `helm_lint.sh` and `helm_template_check.sh` on a matrix of values files.

- `docs.yml`
  - Builds docs to ensure no broken references.

Optional E2E (follow‑up)
- A gated workflow using kind/minikube to install the chart and probe `/healthz`; disabled by default, enabled in a staging branch or with a label.
- A gated auth workflow running a local mock OIDC provider (or static JWKS) to issue a JWT and verify engine authorization end‑to‑end.

---

## Ingress With OIDC at ALB (For Web) vs gRPC For CLI

Ingress pattern: ALB can enforce OIDC at the edge using annotations (issuer, authorize/token/userinfo endpoints, client secret), maintaining a browser session via `AWSELBAuthSessionCookie`. This works well for web UIs and HTTP routes. However, OIDC at ALB is not compatible with raw gRPC clients (the redirect/302-based flow breaks gRPC). Therefore:

- Use OIDC at ALB for web/HTTP endpoints (future Rocketship web UI, simple status pages).
- Keep the Engine gRPC ingress separate and protect gRPC via application-level auth (token or JWT validated by the engine). This is CLI-friendly and avoids ALB redirect complexity.

The chart will ship two presets:
- `values-grpc.yaml` — gRPC ingress with `alb.ingress.kubernetes.io/backend-protocol-version: GRPC`, service port named `grpc`.
- `values-oidc-web.yaml` — OIDC annotations for web endpoints; session cookie and scopes configurable. Not used for gRPC.

RBAC Note
- RBAC is out of scope for this epic. Plan to add role/permission enforcement once orgs/users are fully modeled and JWTs are in place (v2).

---

## Post-Epic Naming Alignment (Managed Cloud & Self-Hosted)

When the v1 workstream (PRs 1–8) completes and we begin prepping the managed cloud rollout:

- **Web UI hostnames**
  - Self-hosted reference environments: migrate the web ingress to become the primary apex host (`globalbank.rocketship.sh`).
  - Managed cloud environment: serve the UI from `app.rocketship.sh`.
- **CLI / gRPC hostnames**
  - Self-hosted reference environments: move the gRPC ingress to `cli.globalbank.rocketship.sh` (profiles and docs must be updated accordingly).
  - Managed cloud environment: expose the gRPC endpoint at `cli.rocketship.sh`.

This hostname swap is intentionally deferred until after PR 8 so the current validation work stays stable. Capture it as a dedicated follow-up task that updates Helm values, DNS guidance, documentation, and profile examples in one pass.
