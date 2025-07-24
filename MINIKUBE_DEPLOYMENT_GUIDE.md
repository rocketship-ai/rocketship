# Minikube Deployment Guide for globalbank.rocketship.sh

This guide provides step-by-step instructions to deploy Rocketship on minikube with HTTPS using the existing globalbank.rocketship.sh certificates.

## Prerequisites Check

First, verify you have all required tools installed:

```bash
# Check if minikube is installed
minikube version

# Check if helm is installed
helm version

# Check if kubectl is installed
kubectl version --client
```

## Step 1: Start Minikube

Start minikube with sufficient resources:

```bash
# Start minikube with enough resources for the full stack
minikube start --memory=8192 --cpus=4

# Verify minikube is running
minikube status
```

## Step 2: Enable Required Addons

Enable NGINX ingress controller:

```bash
# Enable ingress addon
minikube addons enable ingress

# Verify ingress controller is running
kubectl get pods -n ingress-nginx
```

Wait for the ingress controller to be ready (all pods should show `Running` status).

## Step 3: Add Helm Repositories

Add the required Helm repositories:

```bash
# Add Bitnami repository for PostgreSQL and Elasticsearch
helm repo add bitnami https://charts.bitnami.com/bitnami

# Add Temporal repository
helm repo add temporal https://go.temporal.io/helm-charts

# Update repositories
helm repo update
```

## Step 4: Prepare Helm Chart Dependencies

Navigate to the Helm chart and install dependencies:

```bash
# Navigate to the Helm chart directory
cd helm/rocketship

# Install chart dependencies
helm dependency update

# Verify dependencies are downloaded
ls charts/
```

You should see `postgresql-*.tgz`, `elasticsearch-*.tgz`, and `temporal-*.tgz` files.

## Step 5: Create TLS Secret

Create a Kubernetes secret with the existing globalbank.rocketship.sh certificates. Since we have a CA bundle, we need to combine the certificate with the CA bundle for proper certificate chain validation:

```bash
# First, combine the certificate with the CA bundle to create the full certificate chain
cat /Users/magius/Downloads/personal_projects/rocketship-ai/globalbank.rocketship.sh/certificate.crt \
    /Users/magius/Downloads/personal_projects/rocketship-ai/globalbank.rocketship.sh/ca_bundle.crt \
    > /tmp/fullchain.crt

# Create TLS secret using the full certificate chain
kubectl create secret tls rocketship-tls \
  --cert="/tmp/fullchain.crt" \
  --key="/Users/magius/Downloads/personal_projects/rocketship-ai/globalbank.rocketship.sh/private.key"

# Verify the secret was created
kubectl get secrets rocketship-tls
kubectl describe secret rocketship-tls

# Clean up temporary file
rm /tmp/fullchain.crt
```

**Why do we need the CA bundle?**
- The `certificate.crt` is your domain certificate signed by ZeroSSL
- The `ca_bundle.crt` contains the intermediate certificates from ZeroSSL up to the root CA
- Browsers need the full certificate chain to validate that your certificate is trusted
- Without the CA bundle, browsers might show "certificate not trusted" warnings

## Step 6: Create Auth Secrets

Create the OIDC authentication secret. Since Rocketship uses PKCE flow with Auth0, the client secret should be empty:

```bash
# Create OIDC secret with your Auth0 configuration
# Note: client-secret is empty for PKCE flow
kubectl create secret generic rocketship-oidc-secret \
  --from-literal=issuer="https://dev-0ankenxegmh7xfjm.us.auth0.com/" \
  --from-literal=client-id="cq3sxA5rupwsvE4XIf86HXXaI7Ymc4aL" \
  --from-literal=client-secret=""

# Verify the secret
kubectl get secrets rocketship-oidc-secret
```

**Why is client-secret empty?**
- Rocketship uses PKCE (Proof Key for Code Exchange) flow for enhanced security
- PKCE is designed for public clients (like SPAs and CLI tools) that can't securely store secrets
- The client secret is replaced by a dynamically generated code verifier/challenge
- This is more secure for CLI applications since there's no static secret to leak

## Step 7: Configure Values File

Edit the minikube values file to match your configuration:

```bash
# Edit the values file with your specific configuration
nano helm/rocketship/values-minikube.yaml
```

**Replace the following placeholders in the file:**

1. **Authentication configuration:**
   ```yaml
   auth:
     oidc:
       existingSecret: "rocketship-oidc-secret"  # ← Replace with your secret name
     adminEmails: "magiusdarrigo@gmail.com"      # ← Replace with your admin email
   ```

2. **Domain configuration:**
   ```yaml
   tls:
     domain: "globalbank.rocketship.sh"          # ← Replace with your domain
     certificate:
       existingSecret: "rocketship-tls"          # ← Replace with your TLS secret name
   
   ingress:
     hosts:
       - host: "globalbank.rocketship.sh"        # ← Replace with your domain
     tls:
       - secretName: "rocketship-tls"            # ← Replace with your TLS secret name
         hosts:
           - "globalbank.rocketship.sh"          # ← Replace with your domain
   ```

3. **If using placeholder images (nginx) for testing:**
   ```yaml
   rocketship:
     engine:
       image:
         repository: "nginx"                     # ← Change to nginx for testing
       livenessProbe:
         enabled: false                          # ← Set to false for nginx
       readinessProbe:
         enabled: false                          # ← Set to false for nginx
     worker:
       image:
         repository: "nginx"                     # ← Change to nginx for testing
   
   ingress:
     annotations:
       nginx.ingress.kubernetes.io/backend-protocol: "HTTP"  # ← Use HTTP for nginx
   ```

**For globalbank.rocketship.sh specifically, your values should be:**
- `<your-oidc-secret-name>` → `rocketship-oidc-secret`
- `<your-admin-email>` → `magiusdarrigo@gmail.com`  
- `<your-domain>` → `globalbank.rocketship.sh`
- `<your-tls-secret-name>` → `rocketship-tls`

## Step 8: Deploy Rocketship

Deploy the Rocketship Helm chart using the configured values file:

```bash
# Install the Helm chart with your configured values
helm install rocketship-test . -f values-minikube.yaml

# Check the installation status
helm status rocketship-test

# Watch the pods starting up
kubectl get pods -w
```

Wait for all pods to reach `Running` status (except for the engine/worker which may error due to placeholder images).

## Step 9: Configure DNS Resolution

Add the domain to your hosts file so it resolves to minikube:

```bash
# Get minikube IP
MINIKUBE_IP=$(minikube ip)
echo "Minikube IP: $MINIKUBE_IP"

# Add domain to hosts file (you'll need to enter your password)
echo "$MINIKUBE_IP globalbank.rocketship.sh temporal.globalbank.rocketship.sh" | sudo tee -a /etc/hosts

# Verify the entry was added
tail -2 /etc/hosts
```

## Step 10: Start Minikube Tunnel

In a **separate terminal**, start the minikube tunnel to enable ingress:

```bash
# This command needs to run in a separate terminal and stay running
minikube tunnel
```

Keep this terminal open - it provides the tunnel for ingress traffic.

## Step 11: Verify Deployment

Check that all components are deployed correctly:

```bash
# Check all pods
kubectl get pods -o wide

# Check services
kubectl get services

# Check ingress
kubectl get ingress

# Check ingress details
kubectl describe ingress rocketship-test
```

## Step 12: Test HTTPS Access

Test the HTTPS connection to globalbank.rocketship.sh:

```bash
# Test HTTPS connectivity (should show nginx welcome page since we're using placeholder images)
curl -v https://globalbank.rocketship.sh

# Test with certificate verification disabled (to see the response)
curl -k https://globalbank.rocketship.sh

# Check certificate details
openssl s_client -connect globalbank.rocketship.sh:443 -servername globalbank.rocketship.sh < /dev/null 2>/dev/null | openssl x509 -text -noout
```

## Step 13: Access via Browser

Open your browser and navigate to:

- **Main Application**: https://globalbank.rocketship.sh
- **Temporal UI**: https://temporal.globalbank.rocketship.sh (if enabled)

You should see:
- ✅ HTTPS connection with valid certificate
- ✅ nginx welcome page (since we're using placeholder images)
- ✅ No certificate warnings in browser

## Step 14: Verify Kubernetes Resources

Check that all Kubernetes resources are properly configured:

```bash
# Check TLS secret is mounted
kubectl describe deployment rocketship-test-engine | grep -A 10 "Volumes:"

# Check environment variables are set
kubectl describe deployment rocketship-test-engine | grep -A 20 "Environment:"

# Check ingress has TLS configuration
kubectl get ingress rocketship-test -o yaml | grep -A 10 tls:
```

## Step 15: View Logs

Check the application logs:

```bash
# View engine logs (may show errors due to placeholder image)
kubectl logs deployment/rocketship-test-engine

# View worker logs
kubectl logs deployment/rocketship-test-worker

# View ingress controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
```

## Troubleshooting

### If HTTPS doesn't work:

```bash
# Check if ingress controller is ready
kubectl get pods -n ingress-nginx

# Check if tunnel is running (in separate terminal)
# Should show: minikube tunnel

# Verify DNS resolution
nslookup globalbank.rocketship.sh
# Should return the minikube IP

# Check certificate secret
kubectl get secret rocketship-tls -o yaml
```

### If pods are failing:

```bash
# Describe failing pods
kubectl describe pod <pod-name>

# Check resource usage
kubectl top nodes
kubectl top pods
```

### To check ingress status:

```bash
# Check ingress events
kubectl describe ingress rocketship-test

# Check ingress controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller --tail=50
```

## Cleanup

When you're done testing:

```bash
# Uninstall the Helm release
helm uninstall rocketship-test

# Stop minikube tunnel (Ctrl+C in the tunnel terminal)

# Stop minikube
minikube stop

# Remove hosts file entry
sudo sed -i '' '/globalbank.rocketship.sh/d' /etc/hosts
```

## Expected Results

After completing all steps, you should have:

✅ **Rocketship deployed on minikube** with Helm chart  
✅ **HTTPS working** with real globalbank.rocketship.sh certificates  
✅ **Ingress configured** with SSL termination  
✅ **All Kubernetes resources** properly created  
✅ **DNS resolution** working locally  
✅ **Certificate validation** passing in browser  

The engine and worker pods may show errors since we're using nginx placeholder images, but the infrastructure, networking, TLS, and Kubernetes configuration will be fully functional and ready for real Rocketship Docker images.

This demonstrates that the Helm chart is production-ready and works correctly with enterprise certificates and HTTPS configuration!