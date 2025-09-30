# Rocketship Cloud Launch — Master Plan (Usage-Based GitHub SSO)

## Goal

Deliver a production-ready, usage-based Rocketship Cloud offering where customers authenticate via GitHub device flow, run tests against our hosted engine/worker, and manage teams/roles without relying on third-party SaaS IdPs such as Auth0. Enterprise and self-hosted customers must continue to work with the same RBAC scheme.

## Guiding Principles

- **Single auth story:** Device flow for CLI, OAuth for the UI; Rocketship-issued JWTs with consistent claims regardless of underlying IdP.
- **Broker-first:** A lightweight GitHub auth broker mints Rocketship JWTs so the engine continues to use our existing verifier and RBAC logic.
- **Chart-driven:** Helm remains the tactical integration point. Usage-based cloud sets `auth.oidc.mode=github` (broker), while enterprise can point to any OIDC issuer.
- **RBAC everywhere:** Claims-based roles (owner/admin/editor/viewer/service_account) enforced uniformly across cloud, enterprise, and self-hosted deployments.

## Outcome Overview

- Shared DigitalOcean cluster hosts Rocketship Cloud:
  - CLI endpoint: `cli.rocketship.globalbank.com`
  - UI (oauth2-proxy + web): `app.rocketship.globalbank.com`
  - GitHub SSO powers device flow via Rocketship broker.
- CLI experiences (`rocketship login/status/logout`) work out-of-the-box with GitHub accounts.
- Engine validates Rocketship JWTs for gRPC requests, enforcing basic RBAC.
- Documentation covers both usage-based and enterprise setups, including Auth0/Okta/BYO instructions.

---

## PR 1 — GitHub Auth Broker + Helm Integration

**What it accomplishes**
- Implements an HTTP service that proxies the OAuth 2.0 Device Authorization Grant to GitHub and returns Rocketship-signed JWTs + refresh tokens.
- Deploys the broker alongside the engine via Helm and wires engines to the broker endpoints when `auth.oidc.mode=github`.

**Changes**
- Add `internal/authbroker/` service and `cmd/authbroker` entrypoint (Go) with endpoints: `/device/code`, `/token`, `/refresh`, `/.well-known/jwks.json`.
- JWT signer backed by RSA/ECDSA key; refresh token store encrypted at rest (file for dev, KMS/Vault later).
- Helm templates for broker Deployment/Service/ConfigMap, and engine env var wiring when GitHub mode is enabled.
- Usage preset `charts/rocketship/values-github-globalbank.yaml` (or update `values-oidc-web.yaml`) pointing to the broker.

**Tests / validation**
- Unit tests for broker endpoints with mocked GitHub responses.
- `helm_template_check.sh` renders chart with `auth.oidc.mode=github` and asserts broker and engine env vars.
- Manual validation: deploy broker + chart to DO cluster, run `rocketship login` + `rocketship run` end-to-end.

---

## PR 2 — RBAC Claim Enforcement

**What it accomplishes**
- Establishes a minimal role model (`owner/admin/editor/viewer/service_account`), adds claim parsing to the engine, and blocks unauthorized RPCs (e.g., viewers cannot create runs).

**Changes**
- Engine interceptors read `roles` claim from JWT; add a simple `authz` helper.
- Broker minting logic includes role claims (reads from a persistent store, fallback to `owner` on first login).
- CLI surfaces a helpful error when a call is denied by RBAC.

**Tests**
- Engine unit tests simulating tokens with/without roles verifying RPC access.
- Broker tests ensuring JWT claims include roles.

**Manual tests**
- Usage-based cloud: create two users, assign different roles, confirm CLI behavior matches expectations.

---

## PR 3 — Cloud Org & Role Persistence

What it accomplishes
- Introduces a lightweight persistence layer (Postgres table or Redis) for users, organisations, memberships, invitations, and role assignments.

Changes
- Broker (or new `internal/cloud` service) handles user onboarding, role management, and invitation/activation flows.
- API endpoints to list/update org members (usable by future UI/CLI commands).

Tests
- Unit tests for storage layer (CRUD operations, invitation acceptance).
- Broker integration tests verifying `rocketship login` seeds default org and rewrites token claims accordingly.

Manual tests
- Create org, invite additional GitHub usernames, login as invitee, ensure token carries correct org/role.

---

## PR 4 — Docs & Onboarding Experience

**What it accomplishes**
- Updates documentation for the new hosted product, including sign-up flow, CLI login, RBAC basics, and enterprise overrides.

**Changes**
- Cloud quick start: register, `rocketship login`, run tests, view results.
- Enterprise and self-hosted guides updated to describe using the broker vs BYO IdP (Okta/Auth0/Azure). Include `rbac.yaml`/Terraform seeding instructions for self-hosted.
- CLI help text references GitHub login.

**Tests**
- Documentation build (mkdocs) passes.
- CLI help text references GitHub login.

---

## Future Enhancements (Backlog)

- Additional SSO providers via broker (Google, GitLab).
- Web UI for org/role management and billing integration.
- Audit log streaming and webhook events.
- Fine-grained project-level permissions.
