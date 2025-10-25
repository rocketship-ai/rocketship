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
# Test 1: Playwright Python exception (run FIRST - clean environment)
# ==============================================================================
log "Test 1: playwright Python exception"

mkdir -p "$TEMP_DIR/test1"
cat > "$TEMP_DIR/test1/rocketship.yaml" << 'EOF'
name: "Test 1: Python Exception"
tests:
  - name: "playwright Python exception"
    steps:
      - name: "navigate and throw exception"
        plugin: playwright
        config:
          role: script
          language: python
          headless: true
          script: |
            page.goto("https://example.com")

            # This will throw a ZeroDivisionError
            result = 1 / 0
EOF

OUTPUT1=$(rocketship run -af "$TEMP_DIR/test1/rocketship.yaml" 2>&1 || true)

if echo "$OUTPUT1" | grep -qE "(ZeroDivisionError|division by zero|python execution failed)"; then
    log "✅ Test 1: Found Python exception error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 1: Missing Python exception error"
    echo "$OUTPUT1" | tail -20
fi

# Give CI time to clean up processes before next test
log "Waiting 5s for process cleanup..."
sleep 5

# ==============================================================================
# Test 2: Playwright assertion failure
# ==============================================================================
log "Test 2: playwright assertion failure"

mkdir -p "$TEMP_DIR/test2"
cat > "$TEMP_DIR/test2/rocketship.yaml" << 'EOF'
name: "Test 2: Assertion Error"
tests:
  - name: "playwright assertion failure"
    steps:
      - name: "navigate and fail assertion"
        plugin: playwright
        config:
          role: script
          language: python
          headless: true
          script: |
            from playwright.sync_api import expect

            page.goto("https://example.com")

            # This assertion will fail - page title is "Example Domain" not "Wrong Title"
            expect(page).to_have_title("Wrong Title That Does Not Exist")

            result = {"should": "not reach here"}
EOF

OUTPUT2=$(rocketship run -af "$TEMP_DIR/test2/rocketship.yaml" 2>&1 || true)

if echo "$OUTPUT2" | grep -qE "(AssertionError|Assertion failed|expect.*to_have_title.*failed|Timeout.*waiting for)"; then
    log "✅ Test 2: Found playwright assertion error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 2: Missing playwright assertion error"
    echo "$OUTPUT2" | tail -20
fi

# Give CI time to clean up processes before next test
log "Waiting 5s for process cleanup..."
sleep 5

# ==============================================================================
# Test 3: Invalid session error (no browser startup needed)
# ==============================================================================
log "Test 3: invalid session error"

mkdir -p "$TEMP_DIR/test3"
cat > "$TEMP_DIR/test3/rocketship.yaml" << 'EOF'
name: "Test 3: Invalid Session"
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

OUTPUT3=$(rocketship run -af "$TEMP_DIR/test3/rocketship.yaml" 2>&1 || true)

if echo "$OUTPUT3" | grep -q 'session "this-session-was-never-started-12345" is not active'; then
    log "✅ Test 3: Found invalid session error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 3: Missing invalid session error"
    echo "$OUTPUT3" | tail -20
fi

# Give CI time to clean up before browser_use tests
log "Waiting 5s for process cleanup..."
sleep 5

# ==============================================================================
# Test 4: browser_use task failure (max_steps exceeded)
# ==============================================================================
log "Test 4: browser_use task failure"

mkdir -p "$TEMP_DIR/test4"
cat > "$TEMP_DIR/test4/rocketship.yaml" << 'EOF'
name: "Test 4: Task Failure"
tests:
  - name: "browser_use task failure"
    steps:
      - name: "navigate to example.com"
        plugin: playwright
        config:
          role: script
          language: python
          headless: true
          script: |
            page.goto("https://example.com")
            result = {"status": "ready"}

      - name: "browser_use with impossible task"
        plugin: browser_use
        config:
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

OUTPUT4=$(rocketship run -af "$TEMP_DIR/test4/rocketship.yaml" 2>&1 || true)

if echo "$OUTPUT4" | grep -qE "(browser-use execution failed|Task failed|Max steps reached|AgentError|Failed to complete task)"; then
    log "✅ Test 4: Found browser_use task failure error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 4: Missing browser_use task failure error"
    echo "$OUTPUT4" | tail -20
fi

# Give CI time to clean up before timeout test
log "Waiting 5s for process cleanup..."
sleep 5

# ==============================================================================
# Test 5: browser_use timeout (run LAST - kills processes)
# ==============================================================================
log "Test 5: browser_use timeout error"

mkdir -p "$TEMP_DIR/test5"
cat > "$TEMP_DIR/test5/rocketship.yaml" << 'EOF'
name: "Test 5: Timeout Error"
tests:
  - name: "browser_use timeout error"
    steps:
      - name: "navigate to test page"
        plugin: playwright
        config:
          role: script
          language: python
          headless: true
          script: |
            page.goto("https://example.com")
            result = {"status": "ready"}

      - name: "browser_use with 3s timeout (will fail)"
        plugin: browser_use
        config:
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

OUTPUT5=$(rocketship run -af "$TEMP_DIR/test5/rocketship.yaml" 2>&1 || true)

if echo "$OUTPUT5" | grep -qE "(signal: killed|context deadline exceeded|timeout)"; then
    log "✅ Test 5: Found timeout/killed error"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    log "❌ Test 5: Missing timeout error"
    echo "$OUTPUT5" | tail -20
fi

echo ""

# Verify we found all 5 error types
if [ "$ERRORS_FOUND" -eq 5 ]; then
    log "✅ All 5 error types properly surfaced by browser plugins"
    log "   - Python exception (ZeroDivisionError in playwright script)"
    log "   - Assertion failure (playwright expect())"
    log "   - Invalid session (session not started)"
    log "   - Task failure (browser_use max_steps exceeded)"
    log "   - Timeout (browser_use 3s timeout → signal: killed)"
    echo ""
    log "✅ Browser error handling test completed successfully"
    log "   The plugins correctly fail when errors occur and provide"
    log "   clear error messages for debugging"
else
    log "❌ Only found $ERRORS_FOUND/5 expected error types"
    log "All 5 error types must be properly surfaced"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Cleanup
rm -rf "$TEMP_DIR"
