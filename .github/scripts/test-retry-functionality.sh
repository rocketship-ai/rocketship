#!/bin/bash

# Test script for retry functionality across all plugin types

set -e  # Exit on any error

# Add debug output for CI troubleshooting
echo "🔧 Debug: Current working directory: $(pwd)"
echo "🔧 Debug: Available rocketship version: $(rocketship version 2>/dev/null || echo 'not found')"

echo "🔄 Testing retry functionality..."

# Test 1: Verify retry configuration is properly parsed and applied
echo ""
echo "📋 Test 1: Retry configuration validation..."

# Test that retry policy examples validate correctly
echo "  → Validating retry policy example..."
rocketship validate examples/retry-policy/rocketship.yaml
echo "✅ Retry policy example validates correctly"

# Test that retry configuration doesn't break normal operation
echo "  → Running retry policy example (should pass without retries)..."
OUTPUT=$(rocketship run -af examples/retry-policy/rocketship.yaml)
if echo "$OUTPUT" | grep -q "✓ Passed Tests: 4"; then
    echo "✅ Retry policy example runs successfully"
else
    echo "❌ Retry policy example failed to run"
    echo "Output: $OUTPUT"
    exit 1
fi

# Test 2: HTTP Plugin with Retry - Using failing endpoint
echo ""
echo "📋 Test 2: HTTP plugin retry with failing endpoint..."

cat > /tmp/test-http-retry.yaml << 'EOF'
name: HTTP Retry Test
tests:
  - name: HTTP with retries
    steps:
      - name: Failing HTTP request
        plugin: http
        config:
          method: GET
          url: "https://httpbin.org/status/503"
        retry:
          initial_interval: "50ms"
          maximum_interval: "200ms"
          maximum_attempts: 3
          backoff_coefficient: 1.5
        assertions:
          - type: status_code
            expected: 200  # This will fail since we get 503
EOF

echo "  → Running HTTP retry test with debug logging..."
set +e  # Don't exit on failure for this test

OUTPUT=$(ROCKETSHIP_LOG=DEBUG rocketship run -af /tmp/test-http-retry.yaml 2>&1)
EXIT_CODE=$?

set -e

# Check if the test run shows failures (which is what we want)
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 1"; then
    echo "✅ HTTP retry test failed as expected"
else
    echo "❌ HTTP retry test should have failed but didn't"
    echo "Output: $OUTPUT"
    exit 1
fi

# Check that multiple attempts were made by counting actual activity attempts
ERROR_COUNT=$(echo "$OUTPUT" | grep -E "Activity error.*ActivityType http.*Attempt [0-9]" | wc -l | tr -d ' ')
if [ "$ERROR_COUNT" -ge 3 ]; then
    echo "✅ HTTP retry test: Found $ERROR_COUNT retry attempts (≥3 as configured)"
else
    echo "❌ HTTP retry test: Only found $ERROR_COUNT retry attempts, expected at least 3"
    echo "Let's check what error patterns we have:"
    echo "$OUTPUT" | grep -E "Activity error.*ActivityType http.*Attempt [0-9]" | head -10
    exit 1
fi

# Test 3: Script Plugin with Retry - Using script that always fails
echo ""
echo "📋 Test 3: Script plugin retry with failing script..."

cat > /tmp/test-script-retry.yaml << 'EOF'
name: Script Retry Test
tests:
  - name: Script with retries
    steps:
      - name: Failing script
        plugin: script
        config:
          language: javascript
          script: |
            console.log("Attempt made, but will fail");
            throw new Error("Intentional script failure for retry testing");
        retry:
          initial_interval: "25ms"
          maximum_interval: "100ms"
          maximum_attempts: 4
          backoff_coefficient: 1.0
EOF

echo "  → Running Script retry test with debug logging..."
set +e  # Don't exit on failure for this test
OUTPUT=$(ROCKETSHIP_LOG=DEBUG rocketship run -af /tmp/test-script-retry.yaml 2>&1)
EXIT_CODE=$?
set -e

# Check if the test run shows failures (which is what we want)
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 1"; then
    echo "✅ Script retry test failed as expected"
else
    echo "❌ Script retry test should have failed but didn't"
    echo "Output: $OUTPUT"
    exit 1
fi

# Check that retries actually happened by counting actual activity attempts
RETRY_COUNT=$(echo "$OUTPUT" | grep -E "Activity error.*ActivityType script.*Attempt [0-9]" | wc -l | tr -d ' ')
if [ "$RETRY_COUNT" -ge 4 ]; then
    echo "✅ Script retry test: Found $RETRY_COUNT retry attempts (≥4 as configured)"
else
    echo "❌ Script retry test: Only found $RETRY_COUNT retry attempts, expected at least 4"
    echo "Debug output:"
    echo "$OUTPUT" | grep -E "Activity error.*ActivityType script.*Attempt [0-9]"
    exit 1
fi

# Test 4: Steps without retry should fail immediately (no retries)
echo ""
echo "📋 Test 4: No retry configuration should fail immediately (no retries)..."

cat > /tmp/test-no-retry.yaml << 'EOF'
name: No Retry Test
tests:
  - name: No retry config
    steps:
      - name: Failing HTTP without retry
        plugin: http
        config:
          method: GET
          url: "https://httpbin.org/status/404"
        assertions:
          - type: status_code
            expected: 200  # This will fail
EOF

echo "  → Running no-retry test with debug logging..."
set +e  # Don't exit on failure for this test
OUTPUT=$(ROCKETSHIP_LOG=DEBUG rocketship run -af /tmp/test-no-retry.yaml 2>&1)
EXIT_CODE=$?
set -e

# Check if the test run shows failures (which is what we want)
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 1"; then
    echo "✅ No-retry test failed as expected"
else
    echo "❌ No-retry test should have failed but didn't"
    echo "Output: $OUTPUT"
    exit 1
fi

# Check that exactly one attempt was made (no retries)
# Count actual activity attempts, not error message occurrences
RETRY_COUNT=$(echo "$OUTPUT" | grep -E "Activity error.*ActivityType http.*Attempt [0-9]" | wc -l | tr -d ' ')
if [ "$RETRY_COUNT" -eq 1 ]; then
    echo "✅ No-retry test: Found exactly 1 attempt (no retries) as expected"
else
    echo "❌ No-retry test: Found $RETRY_COUNT attempts, expected exactly 1"
    echo "Debug output:"
    echo "$OUTPUT" | grep -E "Activity error.*ActivityType http.*Attempt [0-9]"
    exit 1
fi

# Test 5: Successful step with retry config should not retry
echo ""
echo "📋 Test 5: Successful step with retry config should not retry..."

cat > /tmp/test-success-no-retry.yaml << 'EOF'
name: Success No Retry Test
tests:
  - name: Success with retry config
    steps:
      - name: Successful HTTP with retry config
        plugin: http
        config:
          method: GET
          url: "https://httpbin.org/status/200"
        retry:
          initial_interval: "100ms"
          maximum_attempts: 5
        assertions:
          - type: status_code
            expected: 200  # This will succeed
EOF

echo "  → Running success test with retry config..."
OUTPUT=$(ROCKETSHIP_LOG=DEBUG rocketship run -af /tmp/test-success-no-retry.yaml 2>&1)

# This should succeed
if echo "$OUTPUT" | grep -q "✓ Passed Tests: 1"; then
    echo "✅ Success test: Test passed as expected"
else
    echo "❌ Success test: Test should have passed"
    echo "Output: $OUTPUT"
    exit 1
fi

# Should not have any retry attempts since it succeeded
if echo "$OUTPUT" | grep -q "activity error"; then
    echo "❌ Success test: Should not have any activity errors/retries"
    echo "Output: $OUTPUT"
    exit 1
else
    echo "✅ Success test: No retries attempted for successful step"
fi

# Clean up
rm -f /tmp/test-*-retry.yaml /tmp/test-no-retry.yaml /tmp/test-success-no-retry.yaml

echo ""
echo "🎉 All retry functionality tests passed!"
echo "✅ Verified retry behavior for HTTP and Script plugins"
echo "✅ Verified retry counts meet or exceed configuration (working as expected)"
echo "✅ Verified no retries occur when retry not configured"
echo "✅ Verified successful steps don't trigger retries"
echo "✅ Retry functionality is truly plugin-agnostic!"