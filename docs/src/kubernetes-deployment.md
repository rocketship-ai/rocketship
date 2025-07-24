# Kubernetes Deployment Guide

Deploy Rocketship on Kubernetes for production use with enterprise-grade features including authentication, HTTPS, high availability, and monitoring.

## Overview

This guide covers deploying Rocketship on Kubernetes using Helm charts. You'll get a complete testing platform with:

- ✅ **Enterprise authentication** (Auth0, Okta, Azure AD, Google Workspace)
- ✅ **HTTPS with SSL certificates** (cert-manager or bring-your-own)
- ✅ **High availability** with multiple replicas and autoscaling
- ✅ **Team-based access control** with granular permissions
- ✅ **Production monitoring** with Prometheus integration
- ✅ **Automated deployments** with Helm charts

## Prerequisites

### Infrastructure Requirements
- Kubernetes cluster (v1.19+)
- kubectl configured and connected to your cluster
- Helm 3.8+ installed
- NGINX Ingress Controller
- Persistent Volume support
- cert-manager (optional, for automatic TLS certificates)

### Access Requirements
- Admin access to your identity provider (Auth0, Okta, etc.)
- DNS management for your domain
- SSL certificates (or ability to generate via cert-manager)

### Resource Requirements

**Minimum (Development/Testing):**
- 2 CPU cores, 4GB RAM, 10GB storage

**Production:**
- 8+ CPU cores, 16+ GB RAM, 100+ GB storage (fast SSDs recommended)

## Quick Start (Minikube Testing)

Deploy a complete working Rocketship system on minikube:

```bash
# Start minikube with sufficient resources
minikube start --memory=8192 --cpus=4

# Enable ingress addon
minikube addons enable ingress

# Clone repository and prepare
git clone https://github.com/rocketship-ai/rocketship.git
cd rocketship/helm/rocketship

# Add Helm repositories
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

# Install dependencies
helm dependency update

# Build Rocketship images for minikube
eval $(minikube docker-env)  # Use minikube's Docker daemon
make install                 # Build binaries first
docker build -f .docker/Dockerfile.engine -t rocketshipio/engine:latest .
docker build -f .docker/Dockerfile.worker -t rocketshipio/worker:latest .

# Create test TLS secret (self-signed)
kubectl create secret tls rocketship-tls \
  --cert=<(openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout /dev/stdout -out /dev/stdout -subj "/CN=globalbank.rocketship.sh") \
  --key=<(openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout /dev/stdout -out /dev/stdout -subj "/CN=globalbank.rocketship.sh")

# Deploy complete Rocketship system
helm install rocketship-test . -f values-minikube.yaml

# Add to hosts file for HTTPS testing
echo "$(minikube ip) globalbank.rocketship.sh" | sudo tee -a /etc/hosts

# Start tunnel for ingress (in separate terminal)
minikube tunnel

# Wait for infrastructure to be ready (may take 5-10 minutes)
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=postgresql --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=elasticsearch --timeout=300s

# Check deployment status
kubectl get pods | grep rocketship-test
```

**Result**: Full working Rocketship deployment with engine, worker, databases, Temporal, and monitoring.

**Note**: The engine and worker may take several minutes to start while Temporal initializes. This is normal for the first deployment.

### Alternative: Local Image Build

If you prefer to build images locally instead of using the registry:

```bash
# After the helm install above, build local images
eval $(minikube docker-env)  # Use minikube's Docker daemon
make install                # Build binaries first
docker build -f .docker/Dockerfile.engine -t rocketshipio/engine:latest .
docker build -f .docker/Dockerfile.worker -t rocketshipio/worker:latest .

# Restart deployments to use local images
kubectl rollout restart deployment rocketship-test-engine
kubectl rollout restart deployment rocketship-test-worker
```

For production deployment, continue with the detailed steps below.

## Production Deployment

### Step 1: Prepare Your Cluster

#### 1.1 Create Namespace

```bash
kubectl create namespace rocketship
kubectl config set-context --current --namespace=rocketship
```

#### 1.2 Install NGINX Ingress Controller

If not already installed:

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer
```

#### 1.3 Install cert-manager (Optional)

For automatic TLS certificate management:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.crds.yaml

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.13.0
```

### Step 2: Configure Authentication

#### 2.1 Set Up Your Identity Provider

**For Auth0:**

1. **Create Application**
   - Go to Auth0 Dashboard → Applications → Create Application
   - Name: `Rocketship Enterprise`
   - Type: `Web Application`

2. **Configure Settings**
   ```
   Allowed Callback URLs: https://rocketship.your-domain.com/auth/callback
   Allowed Logout URLs: https://rocketship.your-domain.com/logout
   Grant Types: ✓ Authorization Code, ✓ Refresh Token
   ```

3. **Note Credentials**
   - Domain: `your-tenant.auth0.com`
   - Client ID and Client Secret

**For Okta:**

1. **Create App Integration**
   - Okta Admin Console → Applications → Create App Integration
   - Sign-in method: `OIDC - OpenID Connect`
   - Application type: `Web Application`

2. **Configure Application**
   ```
   App name: Rocketship Enterprise
   Grant type: Authorization Code
   Sign-in redirect URIs: https://rocketship.your-domain.com/auth/callback
   Sign-out redirect URIs: https://rocketship.your-domain.com/logout
   ```

3. **Configure Group Claims**
   - Sign On tab → OpenID Connect ID Token
   - Groups claim type: `Filter`
   - Groups claim filter: `groups Matches regex .*`

#### 2.2 Create Kubernetes Secrets

```bash
# Create OIDC secret
kubectl create secret generic rocketship-oidc-secret \
  --from-literal=issuer="https://your-enterprise.okta.com" \
  --from-literal=client-id="your-client-id" \
  --from-literal=client-secret="your-client-secret" \
  --namespace rocketship
```

### Step 3: Configure TLS/HTTPS

#### Option A: cert-manager (Automatic)

Create ClusterIssuer for Let's Encrypt:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@your-domain.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

```bash
kubectl apply -f cluster-issuer.yaml
```

#### Option B: Bring Your Own Certificate

```bash
# Create TLS secret with your certificates
kubectl create secret tls rocketship-tls \
  --cert=path/to/your/certificate.crt \
  --key=path/to/your/private.key \
  --namespace rocketship
```

### Step 4: Prepare Helm Chart

```bash
# Clone repository
git clone https://github.com/rocketship-ai/rocketship.git
cd rocketship/helm/rocketship

# Add Helm repositories
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update

# Install chart dependencies
helm dependency update
```

### Step 5: Configure Production Values

Create `values-production.yaml`:

```yaml
# Production configuration for your domain
global:
  imageRegistry: "your-registry.com"  # Optional: your private registry
  storageClass: "fast-ssd"

rocketship:
  engine:
    replicas: 3
    resources:
      requests:
        memory: "512Mi"
        cpu: "500m"
      limits:
        memory: "1Gi"
        cpu: "1000m"
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
                - key: app.kubernetes.io/name
                  operator: In
                  values: [rocketship-engine]
            topologyKey: kubernetes.io/hostname
  
  worker:
    replicas: 5
    resources:
      requests:
        memory: "1Gi"
        cpu: "750m"
      limits:
        memory: "2Gi"
        cpu: "1500m"

# Authentication configuration
auth:
  oidc:
    existingSecret: "rocketship-oidc-secret"
  adminEmails: "admin@your-domain.com,devops@your-domain.com"

# TLS configuration
tls:
  enabled: true
  domain: "rocketship.your-domain.com"
  certificate:
    existingSecret: "rocketship-tls"  # If using BYOC
    # Use cert-manager instead:
    # certManager:
    #   issuer: "letsencrypt-prod"

# Ingress configuration
ingress:
  enabled: true
  className: "nginx"
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
    # For cert-manager:
    # cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: "rocketship.your-domain.com"
      paths:
        - path: /
          pathType: Prefix
          service:
            name: rocketship-engine
            port: 7700
  tls:
    - secretName: "rocketship-tls"
      hosts:
        - "rocketship.your-domain.com"

# High-availability databases
postgresql:
  enabled: true
  architecture: "replication"
  primary:
    persistence:
      size: 100Gi
      storageClass: "fast-ssd"
    resources:
      requests:
        memory: "1Gi"
        cpu: "500m"
      limits:
        memory: "2Gi"
        cpu: "1000m"
  readReplicas:
    replicaCount: 2

elasticsearch:
  enabled: true
  replicas: 3
  resources:
    requests:
      memory: "2Gi"
      cpu: "500m"
    limits:
      memory: "4Gi"
      cpu: "1000m"

temporal:
  enabled: true
  server:
    replicas: 3
    resources:
      requests:
        memory: "1Gi"
        cpu: "500m"
      limits:
        memory: "2Gi"
        cpu: "1000m"
  web:
    enabled: true
    replicas: 2

# Production monitoring
serviceMonitor:
  enabled: true
  namespace: "monitoring"
  labels:
    release: prometheus

# Autoscaling
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70

# Pod disruption budgets
podDisruptionBudget:
  enabled: true
  minAvailable: 2

# Network policies (adjust for your network setup)
networkPolicy:
  enabled: true
```

### Step 6: Deploy Rocketship

```bash
# Deploy with production configuration
helm install rocketship . \
  -f values-production.yaml \
  --namespace rocketship \
  --timeout=15m

# Verify deployment
helm status rocketship --namespace rocketship
kubectl get pods --namespace rocketship
kubectl get services --namespace rocketship
kubectl get ingress --namespace rocketship
```

### Step 7: Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n rocketship

# Check ingress has external IP
kubectl get ingress -n rocketship

# Check TLS certificate (if using cert-manager)
kubectl describe certificate rocketship-tls -n rocketship

# Test HTTPS connection
curl -v https://rocketship.your-domain.com/health

# Check engine logs
kubectl logs -n rocketship deployment/rocketship-engine
```

### Step 8: Configure DNS

Point your domain to the ingress controller's external IP:

```bash
# Get external IP
kubectl get service -n ingress-nginx ingress-nginx-controller

# Create DNS A record:
# rocketship.your-domain.com → [EXTERNAL-IP]
```

## Team Management and Usage

### Initial Admin Setup

```bash
# Install Rocketship CLI
curl -L https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64 -o rocketship
chmod +x rocketship
sudo mv rocketship /usr/local/bin/

# Connect to your deployment
rocketship connect https://rocketship.your-domain.com --name production

# Login as admin
rocketship auth login

# Verify admin status
rocketship auth status
```

### Create Teams and Users

```bash
# Create organizational teams
rocketship team create "Platform Engineering"
rocketship team create "Backend Development"
rocketship team create "QA Engineering"

# Add team members with permissions
rocketship team add-member "Platform Engineering" "platform-lead@your-domain.com" admin \
  --permissions "tests:*,repositories:*,team:members:*"

rocketship team add-member "Backend Development" "backend-dev@your-domain.com" member \
  --permissions "tests:read,tests:write,repositories:read"

# Create API tokens for CI/CD
rocketship token create "Production-CI" \
  --team "Platform Engineering" \
  --permissions "tests:write" \
  --expires-in 90d
```

## Monitoring and Observability

### Prometheus Integration

If you have Prometheus installed:

```yaml
# Add to values-production.yaml
serviceMonitor:
  enabled: true
  namespace: "monitoring"
  labels:
    release: prometheus  # Match your Prometheus operator labels

# Available metrics endpoints:
# - rocketship-engine:7701/metrics (engine metrics)
# - Temporal metrics (via Temporal configuration)
```

### Logging with Grafana Loki

Example logging configuration:

```yaml
# logging-config.yaml
apiVersion: logging.coreos.com/v1
kind: ClusterLogForwarder
metadata:
  name: rocketship-logs
spec:
  outputs:
    - name: loki-output
      type: loki
      url: http://loki.monitoring:3100
      loki:
        labelKeys:
          - kubernetes.namespace_name
          - kubernetes.pod_name
  pipelines:
    - name: rocketship-pipeline
      inputRefs:
        - application
      filterRefs:
        - rocketship-filter
      outputRefs:
        - loki-output
```

## Scaling and Performance

### Horizontal Scaling

```bash
# Scale workers for increased throughput
kubectl scale deployment rocketship-worker --replicas=10 -n rocketship

# Scale engines for high availability
kubectl scale deployment rocketship-engine --replicas=5 -n rocketship

# Enable horizontal pod autoscaling
kubectl autoscale deployment rocketship-worker \
  --cpu-percent=70 --min=3 --max=20 -n rocketship
```

### Resource Optimization

Monitor resource usage and adjust:

```bash
# Check resource usage
kubectl top pods -n rocketship
kubectl top nodes

# Update resource limits in values file and upgrade
helm upgrade rocketship . \
  -f values-production.yaml \
  --set rocketship.worker.resources.limits.memory=4Gi \
  --namespace rocketship
```

## Maintenance and Updates

### Upgrading Rocketship

```bash
# Update Helm repository
helm repo update

# Upgrade to latest version
helm upgrade rocketship . \
  -f values-production.yaml \
  --namespace rocketship

# Verify upgrade
helm history rocketship -n rocketship
kubectl get pods -n rocketship
```

### Backup Procedures

```bash
# Backup PostgreSQL databases
kubectl exec -n rocketship rocketship-postgresql-primary-0 -- \
  pg_dump -U temporal temporal > temporal-backup.sql

kubectl exec -n rocketship rocketship-auth-postgresql-0 -- \
  pg_dump -U authuser auth > auth-backup.sql

# Backup Helm configuration
helm get values rocketship -n rocketship > rocketship-values-backup.yaml
```

### Security Updates

```bash
# Update base images (rebuild and push)
docker build --no-cache -f .docker/Dockerfile.engine -t your-registry/rocketship-engine:latest .
docker push your-registry/rocketship-engine:latest

# Update deployment
helm upgrade rocketship . \
  -f values-production.yaml \
  --set rocketship.engine.image.tag=latest \
  --namespace rocketship
```

## Troubleshooting

### Minikube-Specific Issues

**Engine/Worker pods showing "Error" or "CrashLoopBackOff":**
```bash
# Check pod status
kubectl get pods | grep -E "(engine|worker)"

# Check pod logs for errors
kubectl logs deployment/rocketship-test-engine
kubectl logs deployment/rocketship-test-worker

# Common issues: database connection, authentication config, or resource limits
```

**Elasticsearch pods pending or not ready:**
```bash
# Check resource constraints
kubectl top nodes
kubectl describe nodes

# Elasticsearch needs sufficient memory - check if minikube has enough resources
minikube status
minikube addons list | grep metrics-server

# If needed, restart minikube with more resources
minikube stop
minikube start --memory=10240 --cpus=6
```

**Temporal schema initialization failing:**
```bash
# Check temporal schema job logs
kubectl logs job/rocketship-test-temporal-schema

# Common fix: wait for Elasticsearch to be fully ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=elasticsearch --timeout=300s

# If still failing, restart the schema job
kubectl delete job rocketship-test-temporal-schema
helm upgrade rocketship-test . -f values-minikube.yaml
```

**"ImagePullBackOff" for rocketshipio/engine:latest:**
```bash
# Images may not be available yet in the registry
# Build them locally instead:

eval $(minikube docker-env)
make install
docker build -f .docker/Dockerfile.engine -t rocketshipio/engine:latest .
docker build -f .docker/Dockerfile.worker -t rocketshipio/worker:latest .

# Restart deployments
kubectl rollout restart deployment rocketship-test-engine
kubectl rollout restart deployment rocketship-test-worker
```

### General Issues

**Pods not starting:**
```bash
kubectl describe pod [pod-name]
kubectl logs [pod-name]
```

**Ingress not working:**
```bash
kubectl describe ingress rocketship-test
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
```

**Database connection issues:**
```bash
kubectl exec deployment/rocketship-test-engine -- nc -zv rocketship-test-postgresql 5432
kubectl logs deployment/rocketship-test-engine | grep -i database
```

**Certificate issues:**
```bash
kubectl describe certificate rocketship-tls
kubectl logs -n cert-manager deployment/cert-manager
```

### Health Checks

```bash
# Comprehensive health check script
#!/bin/bash
echo "=== Rocketship Health Check ==="

echo "1. Checking pods..."
kubectl get pods -n rocketship

echo "2. Checking services..."
kubectl get services -n rocketship

echo "3. Checking ingress..."
kubectl get ingress -n rocketship

echo "4. Checking TLS certificate..."
kubectl get certificate -n rocketship

echo "5. Testing engine connectivity..."
kubectl port-forward -n rocketship service/rocketship-engine 7700:7700 &
sleep 2
curl -k https://localhost:7700/health
kill %1

echo "6. Checking recent events..."
kubectl get events -n rocketship --sort-by='.metadata.creationTimestamp' | tail -10
```

## CI/CD Integration

### GitHub Actions Example

```yaml
# .github/workflows/rocketship-tests.yml
name: Rocketship Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run Rocketship Tests
        env:
          ROCKETSHIP_API_TOKEN: ${{ secrets.ROCKETSHIP_TOKEN }}
        run: |
          # Install Rocketship CLI
          curl -L https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64 -o rocketship
          chmod +x rocketship
          
          # Run tests against production deployment
          ./rocketship run -f tests/integration.yaml \
            --engine https://rocketship.your-domain.com
```

### Jenkins Pipeline Example

```groovy
pipeline {
    agent any
    environment {
        ROCKETSHIP_API_TOKEN = credentials('rocketship-token')
        ROCKETSHIP_ENGINE = 'https://rocketship.your-domain.com'
    }
    stages {
        stage('Test') {
            steps {
                sh '''
                    curl -L https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64 -o rocketship
                    chmod +x rocketship
                    ./rocketship run -f tests/api-tests.yaml
                '''
            }
        }
    }
    post {
        always {
            sh './rocketship list'
        }
    }
}
```

This Kubernetes deployment guide provides a complete, production-ready setup for Rocketship with enterprise features, security, monitoring, and scalability built-in.