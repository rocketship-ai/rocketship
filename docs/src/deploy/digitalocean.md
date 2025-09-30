# Deploy Rocketship on DigitalOcean Kubernetes

This walkthrough recreates the production proof-of-concept we validated on DigitalOcean Kubernetes (DOKS). It covers standing up Temporal, publishing Rocketship images to DigitalOcean Container Registry (DOCR), terminating TLS through an NGINX ingress, and wiring the CLI via profiles.

The steps assume you have a domain (e.g. `globalbank.rocketship.sh`) with a valid certificate bundle from ZeroSSL or a similar CA.

## Prerequisites

- DigitalOcean account with:
  - A Kubernetes cluster (2 × CPU-optimised nodes were used during validation)
  - DigitalOcean Container Registry (`registry.digitalocean.com/<registry>`) enabled
- [`doctl`](https://docs.digitalocean.com/reference/doctl/how-to/install/) authenticated (`doctl auth init`)
- `kubectl` configured for the cluster (`doctl kubernetes cluster kubeconfig save <cluster-name>`)
- Docker CLI with [Buildx](https://docs.docker.com/build/install-buildx/)
- Helm 3
- TLS assets
  - `certificate.crt` and `private.key` (ZeroSSL issues these; concatenate the intermediate bundle with the server cert if required)

All commands below run from the repository root.

## 1. Set Up Namespaces and Ingress Controller

```bash
kubectl create namespace rocketship
kubectl config set-context --current --namespace=rocketship

# Install ingress-nginx (DigitalOcean automatically provisions a Load Balancer)
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --version 4.13.2 \
  --namespace ingress-nginx --create-namespace \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/do-loadbalancer-enable-proxy-protocol"="true"
```

> The annotation enables PROXY protocol support on DigitalOcean’s load balancer, which keeps source IPs available in the ingress logs. Omit or adjust if you do not need it.

## 2. Install Temporal

```bash
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

helm install temporal temporal/temporal \
  --version 0.66.0 \
  --namespace rocketship \
  --set server.replicaCount=1 \
  --set cassandra.config.cluster_size=1 \
  --set elasticsearch.replicas=1 \
  --set prometheus.enabled=false \
  --set grafana.enabled=false \
  --wait --timeout 15m
```

Register the Temporal logical namespace the Rocketship worker will use:

```bash
kubectl exec -n rocketship deploy/temporal-admintools -- \
  temporal operator namespace create --namespace default
```

(Keep `default` unless you intend to manage multiple namespaces; update Helm values accordingly later.)

## 3. Create the TLS Secret

Issue a SAN certificate that covers both `globalbank.rocketship.sh` and `app.globalbank.rocketship.sh` (Let's Encrypt or ZeroSSL work well). After you have the combined cert/key, update the secret:

```bash
# optional: remove the old secret if it exists
kubectl delete secret globalbank-rocketship-tls -n rocketship 2>/dev/null || true

# create the secret with the new cert/key
kubectl create secret tls globalbank-rocketship-tls \
  --namespace rocketship \
  --cert=/etc/letsencrypt/live/globalbank.rocketship.sh/fullchain.pem \
  --key=/etc/letsencrypt/live/globalbank.rocketship.sh/privkey.pem
```

## 4. Authenticate the Registry Inside the Cluster

Create the image pull secret with `doctl` and apply it to the `rocketship` namespace:

```bash
doctl registry kubernetes-manifest --namespace rocketship > do-registry-secret.yaml
kubectl apply -f do-registry-secret.yaml
```

The secret name is typically `registry-<registry-name>` and is referenced automatically by the chart when `imagePullSecrets` is set.

## 5. Build and Push Rocketship Images

DigitalOcean’s nodes run on `linux/amd64`, so build multi-architecture images to avoid “exec format error” crashes:

```bash
export REGISTRY=registry.digitalocean.com/rocketship
export TAG=v0.1-test

# Engine
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f .docker/Dockerfile.engine \
  -t $REGISTRY/rocketship-engine:$TAG . \
  --push

# Worker
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f .docker/Dockerfile.worker \
  -t $REGISTRY/rocketship-worker:$TAG . \
  --push
```

> Re-run these commands whenever you change code; keep the tag stable (for example `v0.1-test`) so the Helm release pulls the updated digest.

## 6. Deploy the Rocketship Helm Chart

Create a values override file (`deploy/do-values.yaml`) or inline the settings:

```bash
helm install rocketship charts/rocketship \
  --namespace rocketship \
  --set temporal.host=temporal-frontend.rocketship:7233 \
  --set temporal.namespace=default \
  --set engine.image.repository=$REGISTRY/rocketship-engine \
  --set engine.image.tag=$TAG \
  --set worker.image.repository=$REGISTRY/rocketship-worker \
  --set worker.image.tag=$TAG \
  --set imagePullSecrets[0].name=registry-rocketship \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.annotations."nginx\.ingress\.kubernetes\.io/backend-protocol"=GRPC \
  --set ingress.annotations."nginx\.ingress\.kubernetes\.io/ssl-redirect"="true" \
  --set ingress.annotations."nginx\.ingress\.kubernetes\.io/proxy-body-size"="0" \
  --set ingress.tls[0].secretName=globalbank-rocketship-tls \
  --set ingress.tls[0].hosts[0]=globalbank.rocketship.sh \
  --set ingress.hosts[0].host=globalbank.rocketship.sh \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --wait
```

Confirm the pods are healthy:

```bash
kubectl get pods -n rocketship
```

`rocketship-engine` and `rocketship-worker` should report `READY 1/1`. Temporal services may restart once while Cassandra and Elasticsearch initialise—that is expected.

## 7. Enable Auth for the Web UI (optional)

After the gRPC ingress is live you can optionally front the engine’s HTTP port with oauth2-proxy. Choose the option that matches your organisation:

### Option A — GitHub broker (reuse CLI device flow)

1. **Create or reuse a GitHub OAuth app:** visit <https://github.com/settings/developers> (or your organisation equivalent) and register an OAuth App for the CLI device flow. Record the generated Client ID and Client Secret – you will supply them via Kubernetes secrets. The Authorization callback can be any valid HTTPS URL because device flow does not redirect end users.

2. **Create the broker secrets:** use the client ID/secret captured in the previous step.
   ```bash
   # GitHub OAuth app credentials
   kubectl create secret generic globalbank-github-oauth \
     --namespace rocketship \
     --from-literal=ROCKETSHIP_GITHUB_CLIENT_ID=YOUR_GITHUB_CLIENT_ID \
     --from-literal=ROCKETSHIP_GITHUB_CLIENT_SECRET=YOUR_GITHUB_CLIENT_SECRET

   # Encrypted refresh-token store key
   kubectl create secret generic globalbank-auth-broker-store \
     --namespace rocketship \
     --from-literal=ROCKETSHIP_BROKER_STORE_KEY=$(openssl rand -hex 32)

   # JWKS signing material (PEM formatted private key + matching cert)
   kubectl create secret generic globalbank-auth-broker-signing \
     --namespace rocketship \
     --from-file=signing-key.pem=./signing-key.pem

   # Web front-door OAuth client (used by oauth2-proxy). Create a SECOND GitHub OAuth app with
   # callback URL https://app.globalbank.rocketship.sh/oauth2/callback and plug its credentials below.
   kubectl create secret generic oauth2-proxy-credentials \
     --namespace rocketship \
     --from-literal=clientID=YOUR_WEB_OAUTH_CLIENT_ID \
     --from-literal=clientSecret=YOUR_WEB_OAUTH_CLIENT_SECRET \
     --from-literal=cookieSecret=$(python -c "import secrets, base64; print(base64.urlsafe_b64encode(secrets.token_bytes(32)).decode())")
   ```

3. **Review `charts/rocketship/values-github-selfhost.yaml` and `charts/rocketship/values-github-web.yaml`:**
   - Ensure the public hostnames (`cli/globalbank/app/globalbank`) match your ingress controller.
   - Replace the placeholder `YOUR_GITHUB_CLIENT_ID` (and the corresponding secret) with the values from your OAuth app.
   - The oauth2-proxy preset points its issuer at `https://auth.globalbank.rocketship.sh`, which is served by the broker deployment.

4. **Apply the presets alongside the base ingress values:**
   ```bash
   helm upgrade --install rocketship charts/rocketship \
     --namespace rocketship \
     -f charts/rocketship/values-production.yaml \
     -f charts/rocketship/values-github-selfhost.yaml \
     -f charts/rocketship/values-github-web.yaml \
     --wait
   ```

5. **Verify the flow:** visit `https://app.globalbank.rocketship.sh/` in a new session. You should be redirected to GitHub, approve the OAuth app you created, and land on the proxied Rocketship health page (`/healthz`).

### Option B — Bring your own IdP (Auth0/Okta/Azure AD)

1. **Create the oauth2-proxy credentials secret:**
   ```bash
   kubectl create secret generic oauth2-proxy-credentials \
     --namespace rocketship \
     --from-literal=clientID=YOUR_IDP_CLIENT_ID \
     --from-literal=clientSecret=YOUR_IDP_CLIENT_SECRET \
     --from-literal=cookieSecret=$(python -c "import secrets, base64; print(base64.urlsafe_b64encode(secrets.token_bytes(32)).decode())")
   ```

2. **Review `charts/rocketship/values-oidc-web.yaml`:**
   - Set `OAUTH2_PROXY_OIDC_ISSUER_URL` to your IdP’s issuer URL (e.g. `https://auth.globalbank.com/oidc`).
   - Update `OAUTH2_PROXY_REDIRECT_URL` to match the web hostname (`https://app.globalbank.rocketship.sh/oauth2/callback`).
   - Populate `auth.oidc.*` with the native-app client (CLI device flow) details from your IdP.

3. **Apply the preset:**
   ```bash
   helm upgrade --install rocketship charts/rocketship \
     --namespace rocketship \
     -f charts/rocketship/values-production.yaml \
     -f charts/rocketship/values-oidc-web.yaml \
     --wait
   ```

4. **Verify the flow:** browse to `https://app.globalbank.rocketship.sh/`, complete your IdP login, and confirm the proxied Rocketship health page renders (`/healthz`).

## 8. Enable Token Authentication for gRPC (recommended)

Issue a long-lived token for the engine so only authenticated CLI or CI jobs can invoke workflows.

1. **Generate a token and store it in a Kubernetes secret** (replace the example value):
   ```bash
   kubectl create secret generic rocketship-engine-token \
     --namespace rocketship \
     --from-literal=token="rst_self_$(openssl rand -hex 32)"
   ```

2. **Patch your Helm values (or create `values-token.yaml`) to inject the token:**

   ```yaml
   engine:
     env:
       - name: ROCKETSHIP_ENGINE_TOKEN
         valueFrom:
           secretKeyRef:
             name: rocketship-engine-token
             key: token
   ```

   Apply it alongside the production values:

   ```bash
   helm upgrade --install rocketship charts/rocketship \
     --namespace rocketship \
     -f charts/rocketship/values-production.yaml \
     -f values-token.yaml \
     --wait
   ```

3. **Configure the CLI** by setting `ROCKETSHIP_TOKEN` before invoking commands or within your CI secret store:

   ```bash
   export ROCKETSHIP_TOKEN="rst_self_yourtoken"
   rocketship list
   ```

   When the environment variable is absent, the CLI now returns a clear error instructing you to supply the token.

> The token lives entirely in Kubernetes secrets and short-lived environment variables; no code changes are required in the chart. Rotate it by updating the secret and re-running the Helm upgrade.

## 10. Point DNS at the Load Balancer

Retrieve the ingress address and configure an A record for your domain:

```bash
kubectl get ingress -n rocketship
```

For example, the ingress might resolve to `104.248.110.90`. Create an A record such as:

| Host | Value |
| --- | --- |
| `globalbank` | `104.248.110.90` |

Propagation is usually near-immediate within DigitalOcean DNS but may take longer with external registrars.

## 9. Enable OIDC Authentication for CLI (optional)

The Helm chart now ships a turn-key configuration that enables both the UI oauth2-proxy and the engine’s gRPC JWT validation. To use it with Auth0 (or a similar IdP), provision two clients and an API:

1. **Create a custom API** (`Applications → APIs → Create API`). Any URL-style identifier works (e.g. `https://rocketship-engine`). Enable **Allow Offline Access** so the CLI can request refresh tokens.
2. **Create or clone a Native application** for the CLI. Under *Advanced Settings → Grant Types* enable **Device Authorization** (Auth0 shows this only for Native apps) and **Refresh Token**. Then open the API you created in step 1, switch to **Machine to Machine Applications**, and authorise the Native client for the scopes you need (`openid profile email offline_access` etc.). Note the client ID.
3. **Keep your existing Regular Web Application** (or oauth2-proxy client) for the UI. Its client ID/secret remain in the `oauth2-proxy-credentials` secret.

With those pieces in place, edit `charts/rocketship/values-oidc-web.yaml` so `auth.oidc.*` matches your tenant (issuer, native client ID, audience/API identifier, and—if your IdP doesn’t expose discovery—explicit device/token/JWKS endpoints). Then deploy with the one-line command:

```bash
helm upgrade --install rocketship charts/rocketship \
  --namespace rocketship \
  -f charts/rocketship/values-production.yaml \
  -f charts/rocketship/values-oidc-web.yaml \
  --set engine.image.repository=$REGISTRY/rocketship-engine \
  --set engine.image.tag=$TAG \
  --set worker.image.repository=$REGISTRY/rocketship-worker \
  --set worker.image.tag=$TAG \
  --wait
```

After rollout, each developer runs `rocketship login` once (device flow) and the CLI will attach validated JWTs to every gRPC call. If your IdP lacks discovery metadata, override `auth.oidc.deviceEndpoint`, `auth.oidc.tokenEndpoint`, and `auth.oidc.jwksURL` in the values file or via `--set` flags.

After deploying, ask users to sign in once with the CLI:

```bash
rocketship login -p globalbank
rocketship status            # confirm expiry and identity
```

The CLI stores access tokens securely and will refresh them as needed when contacting the engine.

### RBAC considerations

Regardless of where Rocketship runs (cloud usage-based, dedicated enterprise, or self-hosted), the recommended RBAC model is the same:

1. **Issue Rocketship JWTs that carry organisation/team roles.** The broker (or customer IdP) mints access tokens with claims such as `org`, `project`, and `role` (`admin`, `editor`, `viewer`, `service-account`).
2. **Engine enforces on every RPC.** When the CLI calls `CreateRun`, `ListRuns`, etc., the engine reads the claims and rejects calls from users without the required role. Tokens are short-lived and verified via JWKS, so enforcement is consistent across cloud and self-hosted clusters.
3. **Role management lives in Rocketship.** Maintain an RBAC table in Rocketship Cloud (or the broker) so you can invite users, sync GitHub teams if desired, or import roles from customer IdPs. The engine only consumes the resulting claims; it doesn’t need to know whether they originated from GitHub, Okta, or internal configuration.
4. **Future enhancements** (optional): provide an `rbac.yaml` or Terraform provider so self-hosted clusters can seed organisations/roles declaratively, and add UI to sync GitHub org/team membership if customers opt in.

This approach lets you offer the same RBAC semantics in every environment. Usage-based customers can rely on the GitHub-backed broker, while enterprise tenants with their own IdP simply mint tokens that include the same claim set.

## 11. Smoke Test the Endpoint

The Rocketship health endpoint answers gRPC, so an HTTPS request returns `415` with `application/grpc`, which confirms end-to-end TLS:

```bash
curl -v https://globalbank.rocketship.sh/healthz
```

Output snippet:

```
< HTTP/2 415
< content-type: application/grpc
< grpc-status: 3
< grpc-message: invalid gRPC request content-type ""
```

Create and use a profile from the CLI:

```bash
rocketship profile create globalbank grpcs://globalbank.rocketship.sh
rocketship profile use globalbank
rocketship list    # Should connect through TLS without --engine
```

If you see a `connection refused` message against `127.0.0.1:7700`, ensure you are running a CLI build that includes the profile resolution fixes introduced in PR #2.

## 10. Updating the Deployment

1. Rebuild and push the images with the same tag (or bump the `TAG`).
2. Run `helm upgrade rocketship charts/rocketship ...` with the updated values.
3. Watch rollout status:
   ```bash
   kubectl rollout status deploy/rocketship-engine -n rocketship
   kubectl rollout status deploy/rocketship-worker -n rocketship
   ```

## 11. Troubleshooting Tips

- `CrashLoopBackOff` with `exec /bin/engine: exec format error` indicates the image was built for the wrong architecture. Rebuild with `--platform linux/amd64`.
- If the worker logs show `Namespace <name> is not found`, rerun the Temporal namespace creation step and verify `temporal.namespace` in the Helm values matches.
- `curl` connecting to `127.0.0.1` usually means DNS hasn’t propagated or the CLI profile points at the wrong port (`7700` vs `443`). Profiles created with `grpcs://` automatically default to port 443.

With these steps you have a durable Rocketship installation bridging a managed Temporal stack, ingress TLS, and CLI profiles—ready for teams to run suites from their laptops or CI pipelines.
