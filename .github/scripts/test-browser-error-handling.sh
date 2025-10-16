#!/bin/bash
set -e

log() {
  printf '[browser-errors] %s\n' "$1"
}

log "Running browser plugin error handling tests (these should fail)..."

# Create temporary directory for test files
TEMP_DIR=$(mktemp -d)

# Track which errors we found
ERRORS_FOUND=0

# ==============================================================================
# Test 1: browser_use timeout (too short for complex task)
# ==============================================================================
log "Test 1: browser_use timeout error"

cat > "$TEMP_DIR/test1.yaml" << 'EOF'
name: "Test 1: Timeout Error"
tests:
  - name: "browser_use timeout error"
    cleanup:
      always:
        - name: "cleanup browser"
          plugin: playwright
          config:
            role: stop
            session_id: "timeout-test"
    steps:
      - name: "start browser"
        plugin: playwright
        config:
          role: start
          session_id: "timeout-test"
          headless: true

      - name: "navigate to test page"
        plugin: playwright
        config:
          role: script
          session_id: "timeout-test"
          language: python
          script: |
            page.goto("https://example.com")
            result = {"status": "ready"}

      - name: "browser_use with 3s timeout (will fail)"
        plugin: browser_use
        config:
          session_id: "timeout-test"
          timeout: "3s"
          task: |
            Navigate to at least 5 different websites, take screenshots of each,
            analyze their content in detail, and write a comprehensive report.
            This task is impossible to complete in 3 seconds.
          max_steps: 20
          use_vision: true
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
EOF

OUTPUT1=$(rocketship run -af "$TEMP_DIR/test1.yaml" 2>&1 || true)

if echo "$OUTPUT1" | grep -qE "(signal: killed|context deadline exceeded|timeout)"; then
    log "✅ Test 1: Found timeout/killed error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 1: Missing timeout error"
    echo "$OUTPUT1" | tail -20
fi

# ==============================================================================
# Test 2: Invalid session error (no browser startup needed)
# ==============================================================================
log "Test 2: invalid session error"

cat > "$TEMP_DIR/test2.yaml" << 'EOF'
name: "Test 2: Invalid Session"
tests:
  - name: "invalid session error"
    steps:
      - name: "attempt to use non-existent session"
        plugin: playwright
        config:
          role: script
          session_id: "this-session-was-never-started-12345"
          language: python
          script: |
            page.goto("https://example.com")
            result = {"should": "not reach here"}
EOF

OUTPUT2=$(rocketship run -af "$TEMP_DIR/test2.yaml" 2>&1 || true)

if echo "$OUTPUT2" | grep -q 'session "this-session-was-never-started-12345" is not active'; then
    log "✅ Test 2: Found invalid session error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 2: Missing invalid session error"
    echo "$OUTPUT2" | tail -20
fi

echo ""

# Verify we found both error types
if [ "$ERRORS_FOUND" -eq 2 ]; then
    log "✅ Both error types properly surfaced by browser plugins"
    log "   - Timeout (browser_use 3s timeout → signal: killed)"
    log "   - Invalid session (session not started)"
    echo ""
    log "✅ Browser error handling test completed successfully"
    log "   The plugins correctly fail when errors occur and provide"
    log "   clear error messages for debugging"
    echo ""
    log "Note: Additional error scenarios (max_steps, assertion failures) are"
    log "validated by the passing tests in examples/browser/ which run in CI."
else
    log "❌ Only found $ERRORS_FOUND/2 expected error types"
    log "Both error types must be properly surfaced"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Cleanup
rm -rf "$TEMP_DIR"
