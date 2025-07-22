# Profile System Testing Guide

This guide provides step-by-step instructions for testing the complete connection profile system implementation in Rocketship.

## Overview

The profile system enables users to manage connections to multiple Rocketship deployments (local, self-hosted, cloud) without specifying connection details on every command. This guide tests all key functionality:

- ✅ **Profile Management**: Create, list, delete, use, show profiles
- ✅ **Connection Types**: Local development, self-hosted HTTPS, cloud (future)
- ✅ **Authentication Integration**: Profile-specific token storage
- ✅ **Command Integration**: All commands use profiles by default
- ✅ **TLS Support**: Automatic TLS detection and configuration
- ✅ **Backward Compatibility**: Environment variables still work

## Prerequisites

1. **Clean Testing Environment**: Remove existing config to start fresh
   ```bash
   rm -rf ~/.rocketship/config.json
   ```

2. **Self-Hosted Environment**: Have an HTTPS-enabled Rocketship deployment ready
   - We'll use the Docker environment from previous testing
   - Domain: `globalbank.rocketship.sh`
   - TLS certificates generated and working

3. **Build Latest CLI**: Ensure you have the latest code
   ```bash
   make install
   ```

## Test Plan

### Phase 1: Basic Profile Management

#### Test 1.1: Initial State (No Profiles)
```bash
# Should show no profiles found
rocketship profile list
```
**Expected**: "No profiles found."

#### Test 1.2: Auto-Generated Local Profile  
```bash
# First command should auto-create local profile
rocketship --help
rocketship profile list
```
**Expected**: Should show `local` profile with `localhost:7700` and TLS disabled

#### Test 1.3: Create Self-Hosted Profile
```bash
# Create profile for our self-hosted deployment
rocketship profile create enterprise https://globalbank.rocketship.sh:12100

# List profiles to verify
rocketship profile list
```
**Expected**: 
- Two profiles: `local` and `enterprise`
- `enterprise` profile shows TLS enabled with domain `globalbank.rocketship.sh`
- Default profile should still be `local`

#### Test 1.4: Connect Command (Auto-Generation)
```bash
# Test auto-generation from URL
rocketship connect https://staging.example.com:9443

# List profiles to verify
rocketship profile list
```
**Expected**: 
- Three profiles now: `local`, `enterprise`, `staging-example-com`
- New profile shows TLS enabled
- Default profile set to new profile

#### Test 1.5: Profile Use/Switch
```bash
# Switch to enterprise profile
rocketship profile use enterprise

# Verify it's now default
rocketship profile list
```
**Expected**: `enterprise` profile marked with `*` as default

#### Test 1.6: Profile Show Details
```bash
# Show current active profile details
rocketship profile show

# Show specific profile details  
rocketship profile show local
```
**Expected**: Detailed information including TLS settings, engine address

### Phase 2: Docker Environment Integration

#### Test 2.1: Start Self-Hosted Environment
```bash
# Navigate to our repository and start the Docker stack
cd /path/to/rocketship
./.docker/rocketship start

# Check status
./.docker/rocketship status
```
**Expected**: All containers running with HTTPS configuration

#### Test 2.2: Test Connection (Without Profile)
```bash
# Test direct connection to verify environment is working
rocketship run -f examples/simple-http/rocketship.yaml --engine globalbank.rocketship.sh:12100

# If TLS errors, temporarily test with environment variables
ROCKETSHIP_TLS_ENABLED=true ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh rocketship run -f examples/simple-http/rocketship.yaml --engine globalbank.rocketship.sh:12100
```
**Expected**: Test runs successfully with HTTPS connection

### Phase 3: Profile-Aware Commands

#### Test 3.1: Run Command with Profiles
```bash
# Use profile via flag
rocketship run -f examples/simple-http/rocketship.yaml --profile enterprise

# Use active profile (should be enterprise from Test 1.5)
rocketship run -f examples/simple-http/rocketship.yaml

# Switch to local and test
rocketship profile use local
rocketship run -f examples/simple-http/rocketship.yaml --auto
```
**Expected**: 
- Enterprise profile connects to HTTPS deployment
- Local profile uses localhost (auto mode)

#### Test 3.2: List Command with Profiles
```bash
# Use enterprise profile to list runs from HTTPS deployment
rocketship profile use enterprise
rocketship list

# Compare with local
rocketship profile use local
rocketship list
```
**Expected**: Different run histories for different deployments

#### Test 3.3: Get Command with Profiles
```bash
# Get a run from enterprise deployment
rocketship profile use enterprise
rocketship list
# Copy a run ID from the list
rocketship get <run-id>

# Test with profile override
rocketship get <run-id> --profile local
```
**Expected**: Run details retrieved from correct deployment

### Phase 4: Authentication Integration

#### Test 4.1: Profile-Specific Auth Storage
```bash
# Test authentication if OIDC is configured
# This would typically be configured for cloud profile
rocketship profile create cloud https://app.rocketship.sh

# Auth login should use profile-specific storage
rocketship auth login --profile cloud

# Check auth status
rocketship auth status --profile cloud
```
**Expected**: Token stored separately per profile

#### Test 4.2: Cloud Auto-Connection
```bash
# Clear profiles to test auto-connection
rm -rf ~/.rocketship/config.json

# Auth login without setup should auto-create cloud profile
rocketship auth login
```
**Expected**: Cloud profile auto-created and set as default

### Phase 5: Backward Compatibility

#### Test 5.1: Environment Variables Override
```bash
# Test that environment variables still work
ROCKETSHIP_TLS_ENABLED=false rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:7700

# Test TLS environment override
ROCKETSHIP_TLS_ENABLED=true ROCKETSHIP_TLS_DOMAIN=globalbank.rocketship.sh rocketship run -f examples/simple-http/rocketship.yaml --engine globalbank.rocketship.sh:12100
```
**Expected**: Environment variables take precedence over profile settings

#### Test 5.2: Explicit Engine Flag Override
```bash
# Explicit --engine flag should bypass profiles
rocketship profile use enterprise  
rocketship run -f examples/simple-http/rocketship.yaml --engine localhost:7700 --auto
```
**Expected**: Uses localhost despite enterprise profile being active

### Phase 6: Edge Cases and Error Handling

#### Test 6.1: Invalid Profiles
```bash
# Try to use non-existent profile
rocketship profile use nonexistent

# Try to show non-existent profile
rocketship profile show nonexistent
```
**Expected**: Clear error messages

#### Test 6.2: Profile Deletion
```bash
# Delete non-default profile
rocketship profile delete staging-example-com

# Try to delete default profile
rocketship profile delete enterprise

# Try to delete local profile
rocketship profile delete local
```
**Expected**: 
- Non-default profiles delete successfully
- Cannot delete default profile without switching first
- Cannot delete local profile (protected)

#### Test 6.3: Connection Errors
```bash
# Test with invalid domain
rocketship profile create invalid https://nonexistent.domain.com

rocketship run -f examples/simple-http/rocketship.yaml --profile invalid
```
**Expected**: Clear connection error messages

### Phase 7: End-to-End Workflow

#### Test 7.1: Complete Multi-Environment Workflow
```bash
# Start fresh
rm -rf ~/.rocketship/config.json

# 1. Local development
rocketship run -f examples/simple-http/rocketship.yaml --auto

# 2. Connect to staging environment
rocketship connect https://globalbank.rocketship.sh:12100 --name staging
rocketship run -f examples/simple-http/rocketship.yaml

# 3. List and compare runs
rocketship profile use local
echo "Local runs:"
rocketship list

rocketship profile use staging  
echo "Staging runs:"
rocketship list

# 4. Get detailed run info
rocketship list | head -2 | tail -1 | awk '{print $1}' | cut -c1-12 | head -1 > /tmp/run_id
rocketship get $(cat /tmp/run_id)
```

#### Test 7.2: Profile Persistence
```bash
# Restart terminal/shell session
exec $SHELL

# Verify profiles persist
rocketship profile list

# Verify active profile persists
rocketship profile show
```

## Success Criteria

### ✅ Profile Management
- [ ] Profile creation works for all URL types (HTTP/HTTPS)
- [ ] TLS settings auto-detected from URL scheme
- [ ] Profile listing shows correct status and details  
- [ ] Profile switching updates default profile
- [ ] Profile deletion works with proper protections

### ✅ Command Integration
- [ ] All commands (run, list, get) use profiles by default
- [ ] `--profile` flag overrides active profile
- [ ] `--engine` flag bypasses profile system
- [ ] Auto mode works with local profile

### ✅ TLS and Authentication
- [ ] HTTPS connections work through profiles
- [ ] Authentication tokens stored per profile
- [ ] Certificate loading works for self-signed certs
- [ ] Environment variables override profile settings

### ✅ User Experience  
- [ ] Intuitive commands with helpful error messages
- [ ] Auto-generation of profile names from URLs
- [ ] Seamless switching between environments
- [ ] Backward compatibility with existing workflows

### ✅ Configuration Persistence
- [ ] Profiles persist across CLI restarts
- [ ] Configuration stored in `~/.rocketship/config.json`
- [ ] Active profile remembered between sessions

## Troubleshooting

### Common Issues

1. **TLS Certificate Errors**
   ```bash
   # Verify certificates exist
   ls -la ~/.rocketship/certs/

   # Check permissions
   ls -la ~/.rocketship/certs/globalbank.rocketship.sh/
   ```

2. **Connection Refused**
   ```bash
   # Verify Docker stack is running
   ./.docker/rocketship status
   
   # Check port allocation
   ./.docker/rocketship info
   ```

3. **Profile Not Found**
   ```bash
   # List all profiles
   rocketship profile list
   
   # Check config file
   cat ~/.rocketship/config.json
   ```

4. **Environment Variable Conflicts**
   ```bash
   # Check for conflicting env vars
   env | grep ROCKETSHIP
   
   # Unset if needed
   unset ROCKETSHIP_TLS_ENABLED ROCKETSHIP_TLS_DOMAIN
   ```

## Next Steps

After completing this testing guide:

1. **Document any issues found** in GitHub issues
2. **Update CLI help text** based on testing feedback  
3. **Create user-facing documentation** for the profile system
4. **Plan cloud deployment integration** for the auto-generated cloud profile
5. **Consider additional profile features** based on testing experience

This comprehensive testing ensures the profile system works reliably across all supported use cases and provides a smooth user experience for managing multiple Rocketship deployments.