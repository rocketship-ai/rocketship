#!/usr/bin/env bash
set -euo pipefail

log() {
  printf "[%s] %s\n" "profile-tests" "$1"
}

CONFIG_DIR="${HOME}/.rocketship"
CONFIG_FILE="${CONFIG_DIR}/config.json"

log "Resetting CLI config for deterministic profile tests"
rm -f "${CONFIG_FILE}"

log "Ensuring default profile is present"
DEFAULT_LIST_OUTPUT=$(rocketship profile list)
if ! grep -q "default" <<<"${DEFAULT_LIST_OUTPUT}"; then
  echo "❌ default profile missing after reset"
  exit 1
fi
if ! grep -q "cli.rocketship.sh" <<<"${DEFAULT_LIST_OUTPUT}"; then
  echo "❌ default profile should point to cli.rocketship.sh"
  exit 1
fi
log "✅ default profile detected"

log "Creating globalbank profile"
rocketship profile create globalbank grpcs://globalbank.rocketship.sh >/dev/null
rocketship profile use globalbank >/dev/null

PROFILE_LIST=$(rocketship profile list)
if ! grep -q "globalbank.*\\*" <<<"${PROFILE_LIST}"; then
  echo "❌ globalbank profile not marked active"
  echo "${PROFILE_LIST}"
  exit 1
fi
if ! grep -q "enabled (globalbank.rocketship.sh)" <<<"${PROFILE_LIST}"; then
  echo "❌ TLS expectation mismatch for globalbank"
  echo "${PROFILE_LIST}"
  exit 1
fi
log "✅ globalbank profile active with TLS"

log "Running list against globalbank cluster (should require token)"
set +e
LIST_OUTPUT=$(ROCKETSHIP_LOG=DEBUG rocketship list 2>&1)
STATUS=$?
set -e
if [ ${STATUS} -eq 0 ]; then
  echo "❌ expected token enforcement to fail without ROCKETSHIP_TOKEN"
  exit 1
fi
if ! grep -q "requires a token" <<<"${LIST_OUTPUT}"; then
  echo "❌ missing token guidance in failure output"
  echo "${LIST_OUTPUT}"
  exit 1
fi
log "✅ cloud profile correctly demands a token"

log "Starting local engine for discovery checks"
rocketship start server --background >/tmp/rocketship-profile-test.log 2>&1

log "Creating local profile for discovery"
rocketship profile create local grpc://localhost:7700 >/dev/null
rocketship profile use local >/dev/null

# Give the background engine time to accept connections
DISCOVERY_OUTPUT=""
for attempt in {1..10}; do
  if DISCOVERY_OUTPUT=$(rocketship profile show local 2>&1); then
    break
  fi
  sleep 2
done

if [ -z "${DISCOVERY_OUTPUT}" ]; then
  echo "❌ failed to query discovery info from local engine"
  rocketship stop server >/dev/null 2>&1 || true
  exit 1
fi
if ! grep -q "Server Discovery:" <<<"${DISCOVERY_OUTPUT}"; then
  echo "❌ discovery section missing from profile show"
  echo "${DISCOVERY_OUTPUT}"
  rocketship stop server >/dev/null 2>&1 || true
  exit 1
fi
if ! grep -q "Capabilities: .*discovery.v2" <<<"${DISCOVERY_OUTPUT}"; then
  echo "❌ discovery capabilities not reported"
  echo "${DISCOVERY_OUTPUT}"
  rocketship stop server >/dev/null 2>&1 || true
  exit 1
fi
if ! grep -q "Version:" <<<"${DISCOVERY_OUTPUT}"; then
  echo "⚠️ discovery response missing version"
fi
log "✅ discovery v2 surfaced via profile show"

log "Stopping local engine"
rocketship stop server >/dev/null 2>&1 || true

rocketship profile use globalbank >/dev/null 2>&1 || true
rocketship profile delete local >/dev/null 2>&1 || true

log "Starting token-protected engine"
ROCKETSHIP_ENGINE_TOKEN=local-secret rocketship start server --background >/tmp/rocketship-profile-test-token.log 2>&1

log "Creating profile targeting token-protected engine"
rocketship profile create local-token grpc://localhost:7700 >/dev/null
rocketship profile use local-token >/dev/null

log "Validating token enforcement"
set +e
TOKENLESS_OUTPUT=$(rocketship list 2>&1)
STATUS=$?
set -e
if [ ${STATUS} -eq 0 ]; then
  echo "❌ expected list to fail without ROCKETSHIP_TOKEN"
  rocketship stop server >/dev/null 2>&1 || true
  exit 1
fi
if ! grep -q "requires a token" <<<"${TOKENLESS_OUTPUT}"; then
  echo "❌ missing token guidance in failure output"
  echo "${TOKENLESS_OUTPUT}"
  rocketship stop server >/dev/null 2>&1 || true
  exit 1
fi
log "✅ missing token produces helpful error"

log "Listing with ROCKETSHIP_TOKEN provided"
set +e
TOKEN_OUTPUT=$(ROCKETSHIP_TOKEN=local-secret rocketship list 2>&1)
STATUS=$?
set -e
if [ ${STATUS} -ne 0 ]; then
  echo "❌ list failed even with ROCKETSHIP_TOKEN set"
  echo "${TOKEN_OUTPUT}"
  rocketship stop server >/dev/null 2>&1 || true
  exit 1
fi
if ! grep -q "No test runs found." <<<"${TOKEN_OUTPUT}"; then
  echo "⚠️ token-auth list returned unexpected output"
  echo "${TOKEN_OUTPUT}"
fi
log "✅ command succeeds when token supplied"

log "Stopping token-protected engine"
rocketship stop server >/dev/null 2>&1 || true

rocketship profile use globalbank >/dev/null 2>&1 || true
rocketship profile delete local-token >/dev/null 2>&1 || true

log "Testing failure path with unreachable profile"
rocketship profile create unreachable grpc://127.0.0.1:65530 >/dev/null
rocketship profile use unreachable >/dev/null
set +e
UNREACHABLE_OUTPUT=$(rocketship list 2>&1)
STATUS=$?
set -e
if [ ${STATUS} -eq 0 ]; then
  echo "❌ expected failure when connecting to unreachable profile"
  exit 1
fi
if ! grep -qiE "failed to (connect|list runs)" <<<"${UNREACHABLE_OUTPUT}"; then
  echo "❌ unexpected error message for unreachable profile"
  echo "${UNREACHABLE_OUTPUT}"
  exit 1
fi
if ! grep -q "unreachable" <<<"${UNREACHABLE_OUTPUT}" && ! grep -q "127.0.0.1" <<<"${UNREACHABLE_OUTPUT}"; then
  echo "⚠️ failure message missing host context"
fi
log "✅ unreachable profile produces explicit failure"

log "Restoring globalbank profile for downstream tests"
rocketship profile use globalbank >/dev/null
log "✅ profile tests complete"
