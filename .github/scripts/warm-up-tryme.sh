#!/usr/bin/env bash
set -euo pipefail

TARGET=${1:-https://tryme.rocketship.sh}
MAX_ATTEMPTS=${MAX_ATTEMPTS:-10}
SLEEP_SECONDS=${SLEEP_SECONDS:-3}

log() {
  printf '[warmup] %s\n' "$1"
}

log "Warming up test server at $TARGET"

# Wake up backend (ignore failures)
curl -s "$TARGET" >/dev/null 2>&1 || log "Initial probe completed"

attempt=1
while (( attempt <= MAX_ATTEMPTS )); do
  log "Attempt $attempt/$MAX_ATTEMPTS"
  if curl -s -f "$TARGET/users" >/dev/null 2>&1; then
    log "✅ Test server is ready"
    exit 0
  fi
  if (( attempt == MAX_ATTEMPTS )); then
    log "❌ Test server failed to respond after $MAX_ATTEMPTS attempts"
    exit 1
  fi
  log "Server not ready yet, waiting ${SLEEP_SECONDS}s..."
  sleep "$SLEEP_SECONDS"
  (( attempt++ ))
done
