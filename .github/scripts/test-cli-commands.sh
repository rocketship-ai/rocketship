#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[cli-tests] %s\n' "$1"
}

log "Running CLI smoke tests"
rocketship --help >/dev/null
rocketship version

log "Validating .rocketship test suites"
rocketship validate .rocketship/
rocketship validate .rocketship/simple-http.yaml

log "Running targeted script tests"
./.github/scripts/test-log-plugin.sh
./.github/scripts/test-log-plugin-validation.sh
./.github/scripts/test-env-file.sh
./.github/scripts/test-retry-functionality.sh
./.github/scripts/test-http-openapi-form.sh

log "Testing Supabase error handling"
./.github/scripts/test-supabase-error-handling.sh

log "Testing browser error handling"
./.github/scripts/test-browser-error-handling.sh

log "Executing .rocketship test suites"
set +e  # Temporarily disable exit on error to capture output
# Use a unique tryme session for isolation when suites include the X-Test-Session header.
TEST_SESSION="${GITHUB_RUN_ID:-cli-integration}"
OUTPUT=$(rocketship run -ad .rocketship \
  --var mysql_dsn="root:testpass@tcp(127.0.0.1:3306)/testdb" \
  --var test_session="$TEST_SESSION" \
  2>&1)
EXIT_CODE=$?
set -e  # Re-enable exit on error

# Always show the output so we can see which test failed
echo "$OUTPUT"

# Check if any tests failed
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 0"; then
  log "All test suites passed"
else
  echo "❌ Test suite failures detected (exit code: $EXIT_CODE)"
  exit 1
fi

log "Testing start/stop commands"
rocketship start server --background &
SERVER_PID=$!
sleep 5
rocketship stop server
wait $SERVER_PID || true
log "CLI smoke tests complete"
