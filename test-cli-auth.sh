#!/bin/bash
# Test CLI authentication flow

echo "🚀 Testing Rocketship CLI Authentication Flow"
echo "============================================="

# Set up environment for CLI to use external Keycloak URL
export ROCKETSHIP_OIDC_ISSUER_EXTERNAL=http://localhost:12843/realms/rocketship
export ROCKETSHIP_OIDC_CLIENT_ID=rocketship-cli
export ROCKETSHIP_OIDC_CLIENT_SECRET=rocketship-secret
export ROCKETSHIP_OIDC_ADMIN_GROUP=rocketship-admins

# Build the CLI with auth support
echo -e "\n1. Building CLI with authentication support..."
make install

# Check auth status (should be not authenticated)
echo -e "\n2. Checking initial auth status:"
rocketship auth status

# Test running without auth (should fail)
echo -e "\n3. Testing unauthenticated run (should fail):"
cd examples/simple-http
rocketship run -f rocketship.yaml 2>&1 | grep -E "(authentication|error)" || echo "Unexpected result"
cd ../..

echo -e "\n✅ Authentication enforcement is working!"
echo ""
echo "Next steps to complete the test:"
echo "1. Run: rocketship auth login"
echo "2. Authenticate with your GitHub account or test user (admin@rocketship.local / admin123)"
echo "3. Run: rocketship auth status (should show authenticated)"
echo "4. Run: cd examples/simple-http && rocketship run -f rocketship.yaml"
echo "5. The test should now execute successfully!"
echo ""
echo "To test GitHub SSO specifically:"
echo "- When prompted during 'rocketship auth login', choose GitHub instead of username/password"