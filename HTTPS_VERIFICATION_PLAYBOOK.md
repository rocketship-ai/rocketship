# 🔐 HTTPS/TLS Self-Hosted Rocketship Verification Playbook

This playbook provides step-by-step instructions to verify the complete HTTPS/TLS authentication features for self-hosted Rocketship deployments with **clean environment separation**.

## 🎯 **Verification Objectives**

✅ **Clean Environment Separation**: Host system stays clean, no TLS env var conflicts  
✅ **Docker-Only TLS Config**: All HTTPS config contained within Docker stack  
✅ **Authentication Integration**: RBAC + HTTPS working together  
✅ **Certificate Management**: Self-signed certificates for development/testing  
✅ **End-to-End Flow**: Complete authenticated HTTPS workflow  

## 📋 **Prerequisites**

- Docker and Docker Compose installed
- Git repository in clean state (no TLS environment variables set on host)
- `make install` should work without TLS conflicts

## 🚀 **Step-by-Step Verification**

### **Phase 1: Clean Environment Setup**

#### **Step 1.1: Ensure Clean Host Environment**
```bash
# Verify no TLS environment pollution on host
echo "Checking host environment..."
env | grep ROCKETSHIP_TLS || echo "✅ Host environment is clean"

# Verify make install works without conflicts
echo "Testing clean build..."
make install
echo "✅ Build successful - no TLS conflicts"
```

#### **Step 1.2: Initialize Docker Stack**
```bash
# Clean slate - remove any existing stack
./.docker/rocketship clean 2>/dev/null || true

# Initialize isolated Docker environment
./.docker/rocketship init

# Verify stack initialization
./.docker/rocketship info
```

**Expected Output:**
```
📊 Stack Information:
   Name: add-auth
   Project: rocketship-add-auth
   Branch: add-auth
   
🌐 Access Points:
   Temporal UI: http://localhost:[UNIQUE_PORT]
   Engine API: localhost:[UNIQUE_PORT]
```

### **Phase 2: HTTPS Configuration**

#### **Step 2.1: Generate Self-Signed Certificate**
```bash
# Generate self-signed certificate (works immediately, no DNS required)
rocketship certs generate --domain globalbank.rocketship.sh --self-signed

# Verify certificate created
rocketship certs status

# Fix certificate permissions for Docker
chmod -R 755 ~/.rocketship/certs/
```

#### **Step 2.2: Enable HTTPS in Docker Stack**
```bash
# Update Docker environment to enable TLS
ENV_FILE="./.docker/.env.add-auth"

# Enable TLS in Docker stack configuration
sed -i.bak 's/ROCKETSHIP_TLS_ENABLED=.*/ROCKETSHIP_TLS_ENABLED=true/' "$ENV_FILE"
sed -i.bak 's/ROCKETSHIP_TLS_DOMAIN=.*/ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh/' "$ENV_FILE"

# Clean up backup
rm -f "${ENV_FILE}.bak"

# Verify TLS configuration
grep "ROCKETSHIP_TLS" "$ENV_FILE"
```

**Expected Output:**
```
# Certificate generation
⚠️  Generated self-signed certificate for globalbank.rocketship.sh
📁 Certificate saved to /Users/user/.rocketship/certs/globalbank.rocketship.sh

# Certificate status
● globalbank.rocketship.sh
  Valid: Yes
  Issued: [TIMESTAMP]
  Expires: [TIMESTAMP + 1 year]  
  Days remaining: 364
  Issuer: O=Rocketship Self-Signed

# TLS configuration in Docker
ROCKETSHIP_TLS_ENABLED=true
ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh
```

### **Phase 3: Docker Stack with HTTPS**

#### **Step 3.1: Start HTTPS-Enabled Stack**
```bash
# Start Docker stack with HTTPS configuration
./.docker/rocketship start

# Verify all services are healthy
./.docker/rocketship status
```

#### **Step 3.2: Verify HTTPS in Engine Logs**
```bash
# Check engine logs for TLS initialization
./.docker/rocketship logs engine | grep -i tls

# Should show:
# level=DEBUG msg="TLS environment check" enabled=true domain=globalbank.rocketship.sh
# level=INFO msg="TLS enabled for gRPC server" domain=globalbank.rocketship.sh
# level=INFO msg="grpc server listening with TLS" port=:7700 domain=globalbank.rocketship.sh
```

**Expected Output:**
```
engine-1  | level=DEBUG msg="TLS environment check" raw_enabled=true enabled=true domain=globalbank.rocketship.sh
engine-1  | level=INFO msg="loading TLS certificate" domain=globalbank.rocketship.sh
engine-1  | level=INFO msg="TLS enabled for gRPC server" domain=globalbank.rocketship.sh
engine-1  | level=INFO msg="grpc server listening with TLS" port=:7700 domain=globalbank.rocketship.sh
```

### **Phase 4: HTTPS Client Connection**

#### **Step 4.1: Test HTTPS Validation**
```bash
# Get engine port from stack configuration
ENGINE_PORT=$(grep ENGINE_PORT ./.docker/.env.add-auth | cut -d'=' -f2)
echo "Engine running on port: $ENGINE_PORT"

# Test HTTPS validation (note: using explicit flags, no host env vars)
ROCKETSHIP_TLS_ENABLED=true ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh \
rocketship validate examples/simple-http/rocketship.yaml
```

**Expected Output:**
```
time=... level=INFO msg="validating files" count=1
time=... level=INFO msg="validation passed" file=examples/simple-http/rocketship.yaml
time=... level=INFO msg="validation complete" valid=1 invalid=0 total=1
✅ All 1 file(s) passed validation
```

#### **Step 4.2: Test HTTPS Connection to Engine**
```bash
# Test HTTPS connection to running engine
ROCKETSHIP_TLS_ENABLED=true ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh \
rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:$ENGINE_PORT
```

**Expected Behavior:**
- ✅ TLS connection established successfully
- ✅ Custom certificate loaded and validated
- ❌ Authentication required error (expected - shows HTTPS is working)

**Expected Output:**
```
time=... level=DEBUG msg="connecting to engine" address=localhost:12100
time=... level=DEBUG msg="TLS enabled for gRPC client" domain=globalbank.rocketship.sh
time=... level=DEBUG msg="loading custom certificate for TLS connection" domain=globalbank.rocketship.sh
time=... level=ERROR msg="failed to create run" error="...authentication required"
```

### **Phase 5: Authentication Integration**

#### **Step 5.1: Setup Authentication**
```bash
# Check current authentication configuration in Docker stack
./.docker/rocketship logs engine | grep -i auth | head -5

# Should show OIDC configuration loaded from Docker environment
```

#### **Step 5.2: Test Complete Flow (Optional)**
```bash
# If you have Auth0 configured, test complete authenticated HTTPS flow
# This step requires valid Auth0 setup in the Docker environment file

# 1. Login via CLI (if auth configured)
# rocketship auth login --engine https://localhost:$ENGINE_PORT

# 2. Run authenticated test
# rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:$ENGINE_PORT
```

### **Phase 6: Clean Environment Verification**

#### **Step 6.1: Verify Host Environment Stays Clean**
```bash
# Confirm host has no TLS pollution after all operations
echo "Verifying clean host environment..."
env | grep ROCKETSHIP_TLS && echo "❌ Host environment polluted!" || echo "✅ Host environment clean"

# Verify make install still works
make install
echo "✅ Build still works - no environment conflicts"
```

#### **Step 6.2: Test Stack Isolation**
```bash
# Stop stack and verify clean shutdown
./.docker/rocketship stop

# Show stack info
./.docker/rocketship info

# Restart stack to verify persistence
./.docker/rocketship start
```

## 🎉 **Success Criteria**

After completing this playbook, you should have verified:

### **✅ Environment Separation**
- Host system has no TLS environment variables
- `make install` works without conflicts throughout
- All TLS configuration contained in Docker stack
- Stack can be started/stopped without affecting host environment

### **✅ HTTPS/TLS Functionality**
- Self-signed certificates generated and working
- Engine serves HTTPS on configured port
- Client can establish TLS connections
- Certificate validation working properly

### **✅ Authentication Integration**  
- Engine starts with authentication configured
- HTTPS and RBAC components work together
- Proper error handling for authentication required

### **✅ Management Tools**
- Direct CLI commands for certificate management
- Simple Docker environment configuration
- Clean stack initialization and management

## 🔧 **Troubleshooting**

### **Build Failures**
```bash
# If make install fails with TLS errors:
unset ROCKETSHIP_TLS_ENABLED ROCKETSHIP_TLS_DOMAIN
make install
# TLS config remains in Docker only
```

### **Certificate Issues**
```bash
# Fix certificate permissions
chmod -R 755 ~/.rocketship/certs/

# Regenerate certificates
rocketship certs generate --domain globalbank.rocketship.sh --self-signed
chmod -R 755 ~/.rocketship/certs/
```

### **Docker Stack Issues**
```bash
# Force clean rebuild
./.docker/rocketship clean
docker system prune -f
./.docker/rocketship init

# Re-enable HTTPS if needed
sed -i.bak 's/ROCKETSHIP_TLS_ENABLED=.*/ROCKETSHIP_TLS_ENABLED=true/' ./.docker/.env.add-auth
sed -i.bak 's/ROCKETSHIP_TLS_DOMAIN=.*/ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh/' ./.docker/.env.add-auth

./.docker/rocketship start
```

### **Connection Issues**
```bash
# Check correct engine port
grep ENGINE_PORT ./.docker/.env.add-auth

# Verify engine logs
./.docker/rocketship logs engine | tail -20

# Test with debug logging
ROCKETSHIP_LOG=DEBUG ROCKETSHIP_TLS_ENABLED=true ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh \
rocketship validate examples/simple-http/rocketship.yaml
```

## 📝 **Summary**

This playbook demonstrates that the Rocketship HTTPS/TLS implementation provides:

1. **Clean Environment Separation** - No host pollution, Docker-contained config
2. **Working HTTPS/TLS** - Self-signed certificates, secure gRPC communication  
3. **Authentication Ready** - RBAC + HTTPS integrated and functional
4. **Production Ready** - Proper certificate management, stack isolation
5. **Developer Friendly** - Easy setup, clear troubleshooting, no conflicts

The solution successfully separates build-time (clean host) from runtime (Docker TLS) concerns while providing a complete, self-hosted HTTPS authentication system.