# Rocketship Helm Chart

This Helm chart deploys an enterprise-grade Rocketship testing framework on Kubernetes with Temporal workflow orchestration.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- PV provisioner support in the underlying infrastructure
- NGINX Ingress Controller (for ingress)
- cert-manager (optional, for automatic TLS certificates)

## Installation

### Quick Start with Minikube

```bash
# Add required Helm repositories
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

# Install NGINX Ingress Controller in minikube
minikube addons enable ingress

# Edit values file with your configuration
# Replace all <placeholders> with your specific values
nano ./helm/rocketship/values-minikube.yaml

# Deploy Rocketship with minikube configuration
helm install rocketship ./helm/rocketship -f ./helm/rocketship/values-minikube.yaml

# Add domain to /etc/hosts for local testing
echo "$(minikube ip) <your-domain> temporal.<your-domain>" | sudo tee -a /etc/hosts
```

### Production Deployment

```bash
# Add required Helm repositories
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

# Create namespace
kubectl create namespace rocketship

# Create secrets for production
kubectl create secret generic rocketship-oidc-secret \
  --from-literal=issuer="https://your-company.auth0.com/" \
  --from-literal=client-id="your-client-id" \
  --from-literal=client-secret="your-client-secret" \
  --namespace rocketship

kubectl create secret generic rocketship-db-secret \
  --from-literal=password="your-secure-database-password" \
  --namespace rocketship

# Deploy with production values
helm install rocketship ./helm/rocketship \
  -f ./helm/rocketship/values-production.yaml \
  --namespace rocketship
```

## Configuration

### Key Configuration Parameters

| Parameter                    | Description                     | Default                    |
| ---------------------------- | ------------------------------- | -------------------------- |
| `rocketship.engine.replicas` | Number of engine replicas       | `2`                        |
| `rocketship.worker.replicas` | Number of worker replicas       | `3`                        |
| `auth.oidc.issuer`           | OIDC issuer URL                 | `""`                       |
| `auth.oidc.clientId`         | OIDC client ID                  | `""`                       |
| `auth.adminEmails`           | Comma-separated admin emails    | `""`                       |
| `tls.enabled`                | Enable HTTPS/TLS                | `true`                     |
| `tls.domain`                 | TLS domain name                 | `globalbank.rocketship.sh` |
| `ingress.enabled`            | Enable ingress                  | `true`                     |
| `postgresql.enabled`         | Enable PostgreSQL dependency    | `true`                     |
| `elasticsearch.enabled`      | Enable Elasticsearch dependency | `true`                     |
| `temporal.enabled`           | Enable Temporal dependency      | `true`                     |

### Environment-Specific Values

The chart includes pre-configured values files for different environments:

- `values-minikube.yaml`: Optimized for local minikube testing
- `values-production.yaml`: Enterprise-grade production settings

## Authentication Setup

Rocketship uses OIDC for authentication. Configure your identity provider:

### Auth0 Setup

1. Create an Auth0 application
2. Set redirect URI to `https://your-domain/auth/callback`
3. Configure the following in your values file:

```yaml
auth:
  oidc:
    issuer: "https://your-tenant.auth0.com/"
    clientId: "your-client-id"
    clientSecret: "your-client-secret"
  adminEmails: "admin@company.com"
```

### Other OIDC Providers

The chart supports any OIDC-compliant provider (Okta, Azure AD, Google, etc.):

```yaml
auth:
  oidc:
    issuer: "https://your-provider.com/"
    clientId: "your-client-id"
    clientSecret: "your-client-secret"
```

## TLS/HTTPS Configuration

### Self-Signed Certificates (Development)

```yaml
tls:
  enabled: true
  domain: "globalbank.rocketship.sh"
  certificate:
    selfSigned: true
```

### Custom Certificates

```yaml
tls:
  enabled: true
  domain: "your-domain.com"
  certificate:
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
      -----END PRIVATE KEY-----
```

### cert-manager Integration

```yaml
ingress:
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
```

## Resource Requirements

### Minimum Requirements (Minikube)

- CPU: 2 cores
- Memory: 4GB RAM
- Storage: 10GB

### Production Requirements

- CPU: 8+ cores
- Memory: 16+ GB RAM
- Storage: 100+ GB (with fast SSDs recommended)

## Monitoring

### Prometheus Integration

Enable ServiceMonitor for Prometheus scraping:

```yaml
serviceMonitor:
  enabled: true
  namespace: "monitoring"
  labels:
    release: prometheus
```

### Available Metrics

- Engine gRPC metrics on port 7701
- Temporal workflow metrics
- PostgreSQL database metrics
- Kubernetes resource metrics

## Scaling

### Horizontal Pod Autoscaling

```yaml
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
```

### Manual Scaling

```bash
# Scale engine replicas
kubectl scale deployment rocketship-engine --replicas=5

# Scale worker replicas
kubectl scale deployment rocketship-worker --replicas=10
```

## Backup and Recovery

### Database Backups

```bash
# Backup PostgreSQL
kubectl exec -it rocketship-postgresql-0 -- pg_dump -U temporal temporal > backup.sql

# Backup Auth Database
kubectl exec -it rocketship-auth-postgresql-0 -- pg_dump -U authuser auth > auth-backup.sql
```

### Restore from Backup

```bash
# Restore PostgreSQL
kubectl exec -i rocketship-postgresql-0 -- psql -U temporal temporal < backup.sql
```

## Troubleshooting

### Common Issues

#### Engine Not Starting

```bash
# Check engine logs
kubectl logs deployment/rocketship-engine

# Common issues:
# 1. Database connection failed
# 2. OIDC configuration invalid
# 3. TLS certificate issues
```

#### Workers Not Connecting to Temporal

```bash
# Check worker logs
kubectl logs deployment/rocketship-worker

# Check Temporal connectivity
kubectl exec -it deployment/rocketship-worker -- nc -zv rocketship-temporal-frontend 7233
```

#### Ingress Not Working

```bash
# Check ingress status
kubectl get ingress rocketship

# Check NGINX controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Verify DNS resolution
nslookup globalbank.rocketship.sh
```

### Health Checks

```bash
# Check all pods
kubectl get pods

# Check services
kubectl get services

# Check ingress
kubectl get ingress

# Test engine connectivity
kubectl port-forward service/rocketship-engine 7700:7700
```

## Security Considerations

### Network Policies

Enable network policies for production:

```yaml
networkPolicy:
  enabled: true
```

### Security Contexts

All containers run as non-root users with security contexts:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
```

### Secrets Management

Use Kubernetes secrets or external secret management:

```bash
# Using sealed-secrets
kubeseal --format=yaml < secret.yaml > sealed-secret.yaml

# Using external-secrets
kubectl apply -f external-secret.yaml
```

## Upgrading

### Helm Upgrade

```bash
# Upgrade to new version
helm upgrade rocketship ./helm/rocketship \
  -f ./helm/rocketship/values-production.yaml \
  --namespace rocketship
```

### Database Migrations

Database schemas are automatically migrated during upgrades via init containers.

## Development

### Local Development

```bash
# Install dependencies
helm dependency update ./helm/rocketship

# Lint chart
helm lint ./helm/rocketship

# Template and validate
helm template rocketship ./helm/rocketship \
  -f ./helm/rocketship/values-minikube.yaml | kubectl apply --dry-run=client -f -
```

### Testing

```bash
# Install with test values
helm install rocketship-test ./helm/rocketship \
  -f ./helm/rocketship/values-minikube.yaml \
  --set rocketship.engine.image.tag=test

# Run tests
kubectl apply -f tests/
```

## Support

- Documentation: https://docs.rocketship.sh
