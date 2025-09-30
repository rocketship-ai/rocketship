# Deploy Rocketship on DigitalOcean Kubernetes

This walkthrough recreates the production proof-of-concept we validated on DigitalOcean Kubernetes (DOKS). It covers standing up Temporal, publishing Rocketship images to DigitalOcean Container Registry (DOCR), terminating TLS through an NGINX ingress, and wiring the CLI via profiles.

The steps assume you control public DNS for `cli.rocketship.globalbank.com`, `app.rocketship.globalbank.com`, and `auth.rocketship.globalbank.com` (or equivalent) and can issue a SAN certificate that covers all three hosts.

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

Issue a SAN certificate that covers `cli.rocketship.globalbank.com`, `app.rocketship.globalbank.com`, and `auth.rocketship.globalbank.com` (Let’s Encrypt or ZeroSSL work well). After you have the combined cert/key, update the secret:

```bash
# optional: remove the old secret if it exists
kubectl delete secret globalbank-tls -n rocketship 2>/dev/null || true

# create the secret with the new cert/key
kubectl create secret tls globalbank-tls \
  --namespace rocketship \
  --cert=/etc/letsencrypt/live/rocketship.sh/fullchain.pem \
  --key=/etc/letsencrypt/live/rocketship.sh/privkey.pem
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

# Auth broker
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f .docker/Dockerfile.authbroker \
  -t $REGISTRY/rocketship-auth-broker:$TAG . \
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
  --set ingress.tls[0].secretName=globalbank-tls \
  --set ingress.tls[0].hosts[0]=cli.rocketship.globalbank.com \
  --set ingress.hosts[0].host=cli.rocketship.globalbank.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --wait
```

Confirm the pods are healthy:

```bash
kubectl get pods -n rocketship
```

`rocketship-engine`, `rocketship-worker`, `rocketship-auth-broker`, and `rocketship-web-oauth2-proxy` should report `READY 1/1`. Temporal services may restart once while Cassandra and Elasticsearch initialise—that is expected.

## 7. Choose an authentication path

Self-hosted teams typically pick one of two flows:

### Option 1 – GitHub OAuth with the bundled auth broker

This keeps everything inside the chart. The broker handles CLI device flow, and oauth2-proxy fronts the web UI with the same GitHub app.

1. **Provision broker secrets.**

   ```bash
   openssl genrsa -out signing-key.pem 2048
   kubectl create secret generic globalbank-auth-broker-signing \
     --namespace rocketship \
     --from-file=signing-key.pem

   python -c "import os,base64;print(base64.b64encode(os.urandom(32)).decode())" > broker-store.key
   kubectl create secret generic globalbank-auth-broker-store \
     --namespace rocketship \
     --from-file=ROCKETSHIP_BROKER_STORE_KEY=broker-store.key

   kubectl create secret generic globalbank-github-oauth \
     --namespace rocketship \
     --from-literal=ROCKETSHIP_GITHUB_CLIENT_SECRET=<github-client-secret>
   ```

2. **Bootstrap oauth2-proxy credentials.** Use the same GitHub OAuth application (client ID/secret) so both CLI and UI share it.

   ```bash
   COOKIE_SECRET=$(python3 - <<'PY'
   import secrets
   print(secrets.token_hex(16))
   PY
   )
   kubectl create secret generic oauth2-proxy-credentials \
     --namespace rocketship \
     --from-literal=clientID=<github-client-id> \
     --from-literal=clientSecret=<github-client-secret> \
     --from-literal=cookieSecret="$COOKIE_SECRET"
   ```

3. **Deploy with the GitHub presets.**

   ```bash
   helm upgrade --install rocketship charts/rocketship \
     --namespace rocketship \
     -f charts/rocketship/values-production.yaml \
     -f charts/rocketship/values-github-cloud.yaml \
     -f charts/rocketship/values-github-web.yaml \
     --set engine.image.repository=$REGISTRY/rocketship-engine \
     --set engine.image.tag=$TAG \
     --set worker.image.repository=$REGISTRY/rocketship-worker \
     --set worker.image.tag=$TAG \
     --set auth.broker.image.repository=$REGISTRY/rocketship-auth-broker \
     --set auth.broker.image.tag=$TAG \
     --wait
   ```

4. **Validate the flows.**
   ```bash
   rocketship profile create cloud grpcs://cli.rocketship.globalbank.com
   rocketship profile use cloud
   rocketship login
   rocketship status
   ```
   Browse to `https://app.rocketship.globalbank.com/` in a fresh session—you should be redirected through GitHub and land back on the proxied Rocketship UI after approval. The CLI command above walks you through device flow (`https://github.com/login/device`) and persists the refresh token locally.

> The broker stores only hashed refresh tokens encrypted at rest. Rotate the signing key or store key by updating the Kubernetes secrets and rerunning `helm upgrade`.

### Option 2 – Bring your own IdP (Auth0, Okta, Azure AD, …)

If you already manage an internal IdP, point the chart at it. Provision the necessary applications in your provider (typically a native app for the CLI device flow and a web app for oauth2-proxy), then update `charts/rocketship/values-oidc-web.yaml` with your issuer, client IDs, and scopes.

```bash
helm upgrade --install rocketship charts/rocketship \
  --namespace rocketship \
  -f charts/rocketship/values-production.yaml \
  -f charts/rocketship/values-oidc-web.yaml \
  --set engine.image.repository=$REGISTRY/rocketship-engine \
  --set engine.image.tag=$TAG \
  --set worker.image.repository=$REGISTRY/rocketship-worker \
  --set worker.image.tag=$TAG \
  --set auth.broker.image.repository=$REGISTRY/rocketship-auth-broker \
  --set auth.broker.image.tag=$TAG \
  --wait
```

After rollout, point your CLI profile at the engine (`rocketship profile create <name> grpcs://cli.rocketship.globalbank.com`) and run `rocketship login`. The CLI follows the device flow your IdP exposes and automatically refreshes the issued token on subsequent commands.

### RBAC considerations

Regardless of where Rocketship runs (cloud usage-based, dedicated enterprise, or self-hosted), the recommended RBAC model is the same:

1. **Issue Rocketship JWTs that carry organisation/team roles.** The broker (or customer IdP) mints access tokens with claims such as `org`, `project`, and `role` (`admin`, `editor`, `viewer`, `service-account`).
2. **Engine enforces on every RPC.** When the CLI calls `CreateRun`, `ListRuns`, etc., the engine reads the claims and rejects calls from users without the required role. Tokens are short-lived and verified via JWKS, so enforcement is consistent across cloud and self-hosted clusters.
3. **Role management lives in Rocketship.** Maintain an RBAC table in Rocketship Cloud (or the broker) so you can invite users, sync GitHub teams if desired, or import roles from customer IdPs. The engine only consumes the resulting claims; it doesn’t need to know whether they originated from GitHub, Okta, or internal configuration.
4. **Future enhancements** (optional): provide an `rbac.yaml` or Terraform provider so self-hosted clusters can seed organisations/roles declaratively, and add UI to sync GitHub org/team membership if customers opt in.

This approach lets you offer the same RBAC semantics in every environment. Usage-based customers rely on the GitHub-backed broker, while enterprise tenants with their own IdP simply mint tokens that include the same claim set.

## 10. Point DNS at the Load Balancer

Create A (or CNAME) records for `cli.rocketship.globalbank.com`, `app.rocketship.globalbank.com`, and `auth.rocketship.globalbank.com` pointing at the ingress load balancer IP (see step 6). DNS propagation usually completes within a minute on DigitalOcean DNS, but public resolvers may take longer.

## 11. Smoke Test the Endpoint

The Rocketship health endpoint answers gRPC, so an HTTPS request returns `415` with `application/grpc`, which confirms end-to-end TLS:

```bash
curl -v https://cli.rocketship.globalbank.com/healthz
curl -v https://auth.rocketship.globalbank.com/healthz
```

Create and use the default cloud profile from the CLI (already pointing at `cli.rocketship.globalbank.com:443`):

```bash
rocketship profile list
rocketship login
rocketship run -f examples/simple-http/rocketship.yaml
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
