#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[cli-tests] %s\n' "$1"
}

log "Running CLI smoke tests"
rocketship --help >/dev/null
rocketship version

log "Validating examples"
rocketship validate examples/
rocketship validate examples/simple-http/rocketship.yaml

log "Running targeted script tests"
./.github/scripts/test-log-plugin.sh
./.github/scripts/test-log-plugin-validation.sh
./.github/scripts/test-env-file.sh
./.github/scripts/test-retry-functionality.sh
./.github/scripts/test-http-openapi-form.sh

log "Testing Supabase error handling"
./.github/scripts/test-supabase-error-handling.sh

log "Running browser plugin integration"
OUTPUT=$(rocketship run -af examples/browser-testing/rocketship.yaml)
echo "$OUTPUT"
if echo "$OUTPUT" | grep -q "✓ Passed Tests: 2"; then
  log "Browser plugin passed"
  if echo "$OUTPUT" | grep -q "Max steps test success (should be false): false"; then
    log "Max steps guard validated"
  else
    log "⚠️ Max steps guard message missing"
  fi
else
  echo "❌ Browser plugin integration tests failed"
  exit 1
fi

log "Testing browser assertion failure handling"
./.github/scripts/test-browser-assertion-failures.sh

log "Executing examples directory"
OUTPUT=$(rocketship run -ad examples --var mysql_dsn="root:testpass@tcp(127.0.0.1:3306)/testdb")
echo "$OUTPUT"
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 0"; then
  log "All examples passed"
else
  echo "❌ Example suite failures detected"
  exit 1
fi

log "Testing start/stop commands"
rocketship start server --background &
SERVER_PID=$!
sleep 5
rocketship stop server
wait $SERVER_PID || true
log "CLI smoke tests complete"
