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
  --namespace ingress-nginx --create-namespace \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/do-loadbalancer-enable-proxy-protocol"="true"
```

> The annotation enables PROXY protocol support on DigitalOcean’s load balancer, which keeps source IPs available in the ingress logs. Omit or adjust if you do not need it.

## 2. Install Temporal

```bash
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

helm install temporal temporal/temporal \
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

DigitalOcean expects the key and certificate in PEM format. Convert the ZeroSSL bundle into the standard filenames if necessary:

```bash
# Combine the server cert and intermediate bundle when provided separately
cat certificate.crt ca_bundle.crt > fullchain.pem

kubectl create secret tls globalbank-rocketship-tls \
  --cert=fullchain.pem \
  --key=private.key \
  --namespace rocketship
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

## 7. Point DNS at the Load Balancer

Retrieve the ingress address and configure an A record for your domain:

```bash
kubectl get ingress -n rocketship
```

For example, the ingress might resolve to `104.248.110.90`. Create an A record such as:

| Host | Value |
| --- | --- |
| `globalbank` | `104.248.110.90` |

Propagation is usually near-immediate within DigitalOcean DNS but may take longer with external registrars.

## 8. Smoke Test the Endpoint

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

## 9. Updating the Deployment

1. Rebuild and push the images with the same tag (or bump the `TAG`).
2. Run `helm upgrade rocketship charts/rocketship ...` with the updated values.
3. Watch rollout status:
   ```bash
   kubectl rollout status deploy/rocketship-engine -n rocketship
   kubectl rollout status deploy/rocketship-worker -n rocketship
   ```

## 10. Troubleshooting Tips

- `CrashLoopBackOff` with `exec /bin/engine: exec format error` indicates the image was built for the wrong architecture. Rebuild with `--platform linux/amd64`.
- If the worker logs show `Namespace <name> is not found`, rerun the Temporal namespace creation step and verify `temporal.namespace` in the Helm values matches.
- `curl` connecting to `127.0.0.1` usually means DNS hasn’t propagated or the CLI profile points at the wrong port (`7700` vs `443`). Profiles created with `grpcs://` automatically default to port 443.

With these steps you have a durable Rocketship installation bridging a managed Temporal stack, ingress TLS, and CLI profiles—ready for teams to run suites from their laptops or CI pipelines.
