#!/bin/bash
# Test environment setup script

export ROCKETSHIP_OIDC_ISSUER="https://dev-0ankenxegmh7xfjm.us.auth0.com/"
export ROCKETSHIP_OIDC_CLIENT_ID="cq3sxA5rupwsvE4XIf86HXXaI7Ymc4aL"
export ROCKETSHIP_ADMIN_EMAILS="magiusdarrigo@gmail.com"
export OIDC_CLIENT_SECRET=""
export ROCKETSHIP_ENGINE="localhost:12100"
export ROCKETSHIP_DB_HOST="localhost"
export ROCKETSHIP_DB_PORT="9834"
export ROCKETSHIP_DB_NAME="auth"
export ROCKETSHIP_DB_USER="authuser"
export ROCKETSHIP_DB_PASSWORD="authpass"

# HTTPS/TLS Configuration
export ROCKETSHIP_TLS_ENABLED="true"
export ROCKETSHIP_TLS_DOMAIN="globalbank.rocketship.sh"

# Run the command passed as arguments
exec "$@"