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
# Test 2: browser_use task failure (max steps or cannot complete)
# ==============================================================================
log "Test 2: browser_use max_steps exceeded"

cat > "$TEMP_DIR/test2.yaml" << 'EOF'
name: "Test 2: Max Steps Error"
tests:
  - name: "browser_use max_steps exceeded"
    cleanup:
      always:
        - name: "cleanup browser"
          plugin: playwright
          config:
            role: stop
            session_id: "max-steps-test"
    steps:
      - name: "start browser"
        plugin: playwright
        config:
          role: start
          session_id: "max-steps-test"
          headless: true

      - name: "navigate to YouTube"
        plugin: playwright
        config:
          role: script
          session_id: "max-steps-test"
          language: python
          script: |
            from playwright.sync_api import expect

            page.goto("https://www.youtube.com")
            expect(page).to_have_url("https://www.youtube.com/")
            result = {"status": "ready"}

      - name: "browser_use attempts impossible task"
        plugin: browser_use
        config:
          session_id: "max-steps-test"
          task: |
            Navigate to https://example.com, scroll down 50 times, find a blue button
            with text "Submit Rocketship Test Form XYZ", click it, and verify success.
            You must complete ALL steps.
          max_steps: 2
          use_vision: true
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
EOF

OUTPUT2=$(rocketship run -af "$TEMP_DIR/test2.yaml" 2>&1 || true)

if echo "$OUTPUT2" | grep -qE "(browser-use execution failed|Task failed|Max steps reached|AgentError|Failed to complete task)"; then
    log "✅ Test 2: Found browser_use task failure error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 2: Missing browser_use task failure error"
    echo "$OUTPUT2" | tail -20
fi

# ==============================================================================
# Test 3: Playwright assertion failure (expect())
# ==============================================================================
log "Test 3: playwright assertion failure"

cat > "$TEMP_DIR/test3.yaml" << 'EOF'
name: "Test 3: Assertion Error"
tests:
  - name: "playwright assertion failure"
    cleanup:
      always:
        - name: "cleanup browser"
          plugin: playwright
          config:
            role: stop
            session_id: "assertion-test"
    steps:
      - name: "start browser"
        plugin: playwright
        config:
          role: start
          session_id: "assertion-test"
          headless: true

      - name: "navigate and fail assertion"
        plugin: playwright
        config:
          role: script
          session_id: "assertion-test"
          language: python
          script: |
            from playwright.sync_api import expect

            page.goto("https://example.com")

            # This assertion will fail - page title is "Example Domain" not "Wrong Title"
            expect(page).to_have_title("Wrong Title That Does Not Exist")

            result = {"should": "not reach here"}
EOF

OUTPUT3=$(rocketship run -af "$TEMP_DIR/test3.yaml" 2>&1 || true)

if echo "$OUTPUT3" | grep -qE "(AssertionError|Assertion failed|expect.*to_have_title.*failed|Timeout.*waiting for)"; then
    log "✅ Test 3: Found playwright assertion error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 3: Missing playwright assertion error"
    echo "$OUTPUT3" | tail -20
fi

# ==============================================================================
# Test 4: Invalid session error
# ==============================================================================
log "Test 4: invalid session error"

cat > "$TEMP_DIR/test4.yaml" << 'EOF'
name: "Test 4: Invalid Session"
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

OUTPUT4=$(rocketship run -af "$TEMP_DIR/test4.yaml" 2>&1 || true)

if echo "$OUTPUT4" | grep -q 'session "this-session-was-never-started-12345" is not active'; then
    log "✅ Test 4: Found invalid session error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 4: Missing invalid session error"
    echo "$OUTPUT4" | tail -20
fi

echo ""

# Verify we found all 4 error types
if [ "$ERRORS_FOUND" -eq 4 ]; then
    log "✅ All 4 error types properly surfaced by browser plugins"
    log "   - Timeout (browser_use 3s timeout)"
    log "   - Task failure (max_steps exceeded / impossible task)"
    log "   - Assertion failure (playwright expect())"
    log "   - Invalid session (session not started)"
    echo ""
    log "✅ Browser error handling test completed successfully"
    log "   The plugins correctly fail when errors occur and provide"
    log "   clear error messages for debugging"
else
    log "❌ Only found $ERRORS_FOUND/4 expected error types"
    log "All 4 error types must be properly surfaced"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Cleanup
rm -rf "$TEMP_DIR"
