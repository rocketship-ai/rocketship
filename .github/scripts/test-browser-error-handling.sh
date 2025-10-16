#!/bin/bash
set -e

log() {
  printf '[browser-errors] %s\n' "$1"
}

log "Running browser plugin error handling tests (these should fail)..."

# Create temporary test file for error scenarios
TEMP_DIR=$(mktemp -d)
TEMP_FILE="$TEMP_DIR/browser-error-tests.yaml"

cat > "$TEMP_FILE" << 'EOF'
name: "Browser Error Scenarios"
description: "Intentional failures to verify error handling"

tests:
  # Test 1: browser_use timeout (too short for complex task)
  - name: "Test 1: browser_use timeout error"
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

  # Test 2: browser_use fails impossible task (max_steps too low)
  - name: "Test 2: browser_use max_steps exceeded"
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

  # Test 3: playwright expect() assertion failure
  - name: "Test 3: playwright assertion failure"
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

  # Test 4: invalid session ID (session not started)
  - name: "Test 4: invalid session error"
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

# Run tests and capture output
OUTPUT=$(rocketship run -af "$TEMP_FILE" 2>&1 || true)

log "Test output:"
echo "$OUTPUT"
echo ""

# Check that exactly 4 tests failed
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 4"; then
    log "✅ Exactly 4 tests failed as expected"
else
    log "❌ Expected exactly 4 tests to fail"
    log "Output should contain '✗ Failed Tests: 4'"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Track which errors we found
ERRORS_FOUND=0

# Test 1: Timeout error (signal: killed or context deadline exceeded)
if echo "$OUTPUT" | grep -qE "(signal: killed|context deadline exceeded|timeout)"; then
    log "✅ Test 1: Found timeout/killed error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 1: Missing timeout error message"
fi

# Test 2: browser_use task failure (max steps or cannot complete)
if echo "$OUTPUT" | grep -qE "(browser-use execution failed|Task failed|Max steps reached|AgentError)"; then
    log "✅ Test 2: Found browser_use task failure error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 2: Missing browser_use task failure error"
fi

# Test 3: Playwright assertion failure (expect())
if echo "$OUTPUT" | grep -qE "(AssertionError|Assertion failed|expect.*to_have_title.*failed|Timeout.*waiting for)"; then
    log "✅ Test 3: Found playwright assertion error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 3: Missing playwright assertion error"
fi

# Test 4: Invalid session error
if echo "$OUTPUT" | grep -q 'session "this-session-was-never-started-12345" is not active'; then
    log "✅ Test 4: Found invalid session error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 4: Missing invalid session error"
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
