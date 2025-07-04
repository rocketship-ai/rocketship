#!/bin/bash

# Integration test for auto mode cancellation functionality
# Tests that Ctrl+C properly stops the server in auto mode

set -e  # Exit on any error

echo "🚀 Testing auto mode cancellation functionality..."

cleanup() {
    echo "Cleanup: Ensuring no stray test processes remain..."
    # Kill any rocketship processes specifically started by this test
    pkill -f "rocketship.*start.*server" || true
    pkill -f "rocketship.*run.*test-auto" || true
    sleep 1
}

# Ensure cleanup runs on exit
trap cleanup EXIT

# Test 1: Auto mode cancellation with SIGINT
echo ""
echo "📋 Test 1: Auto mode cancellation with SIGINT (Ctrl+C simulation)..."

echo "  → Starting test with a long-running delay (will be cancelled)..."

# Create a test YAML file with a long delay that we can cancel
cat > /tmp/test-auto-cancel.yaml << 'EOF'
version: v1.0.0
name: Auto Mode Cancellation Test
tests:
  - name: Long running test
    steps:
      - name: Long delay step
        plugin: delay
        config:
          duration: 30s  # Long enough to cancel before completion
EOF

# Run rocketship in auto mode in background and capture its PID
echo "  → Running rocketship in auto mode with long delay..."
rocketship run -af /tmp/test-auto-cancel.yaml &
ROCKETSHIP_PID=$!

# Wait for the test to start (give it a few seconds to begin)
echo "  → Waiting for test to start..."
sleep 3

# Send SIGINT to simulate Ctrl+C
echo "  → Sending SIGINT (Ctrl+C) to rocketship process..."
kill -INT $ROCKETSHIP_PID

# Wait for the process to exit
echo "  → Waiting for process to exit..."
if wait $ROCKETSHIP_PID 2>/dev/null; then
    echo "❌ Process exited with status 0 (unexpected - should be cancelled)"
    exit 1
else
    echo "✅ Process exited with non-zero status (expected for cancellation)"
fi

# Verify that no rocketship server processes are still running
echo "  → Checking for any remaining rocketship processes..."
sleep 2  # Give time for cleanup

# Check if any rocketship server processes are still running (more specific check)
REMAINING_PROCESSES=$(pgrep -f "rocketship.*start.*server" || true)
if [ -n "$REMAINING_PROCESSES" ]; then
    echo "❌ Found remaining rocketship server processes after cancellation:"
    ps -p $REMAINING_PROCESSES || true
    exit 1
else
    echo "✅ No remaining rocketship server processes found"
fi

# Test 2: Verify server is not accessible after cancellation
echo ""
echo "📋 Test 2: Verify server cleanup after cancellation..."

echo "  → Attempting to connect to server (should fail)..."
if rocketship list -e localhost:7700 2>/dev/null; then
    echo "❌ Server is still accessible after cancellation"
    exit 1
else
    echo "✅ Server is not accessible (expected after cleanup)"
fi

# Test 3: Verify normal auto mode still works
echo ""
echo "📋 Test 3: Verify normal auto mode still works after cancellation test..."

# Create a short test that should complete normally
cat > /tmp/test-auto-normal.yaml << 'EOF'
version: v1.0.0
name: Normal Auto Mode Test
tests:
  - name: Quick test
    steps:
      - name: Quick delay step
        plugin: delay
        config:
          duration: 100ms
EOF

echo "  → Running quick test in auto mode (should complete normally)..."
OUTPUT=$(rocketship run -af /tmp/test-auto-normal.yaml)

# Check if test completed successfully
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 0"; then
    echo "✅ Normal auto mode works correctly after cancellation test"
else
    echo "❌ Normal auto mode failed after cancellation test"
    echo "Output: $OUTPUT"
    exit 1
fi

# Cleanup test files
rm -f /tmp/test-auto-cancel.yaml /tmp/test-auto-normal.yaml

echo ""
echo "🎉 All auto mode cancellation tests passed!"
echo "   ✅ SIGINT cancellation works properly"
echo "   ✅ Server cleanup occurs on cancellation"
echo "   ✅ No lingering processes after cancellation"
echo "   ✅ Server is inaccessible after cleanup"
echo "   ✅ Normal auto mode still works correctly"