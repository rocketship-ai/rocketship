#!/bin/bash

set -e

echo "Testing log plugin functionality..."

# Test 1: Normal logging level
echo "Running log plugin test with normal logging..."
OUTPUT=$(rocketship run -af examples/simple-log/rocketship.yaml)
echo "$OUTPUT"

# Check that specific log messages appear in stdout
if echo "$OUTPUT" | grep -q "🚀 Starting user-service tests in staging environment"; then
  echo "✅ Log plugin test 1 passed: startup message found"
else
  echo "❌ Log plugin test 1 failed: startup message not found in output"
  exit 1
fi

if echo "$OUTPUT" | grep -q "Running on.*machine at"; then
  echo "✅ Log plugin test 2 passed: environment variable interpolation working"
else
  echo "❌ Log plugin test 2 failed: environment variable message not found"
  exit 1
fi

if echo "$OUTPUT" | grep -q "Created test data with ID: test_.*Status: active"; then
  echo "✅ Log plugin test 3 passed: runtime variable interpolation working"
else
  echo "❌ Log plugin test 3 failed: runtime variable message not found"
  exit 1
fi

if echo "$OUTPUT" | grep -q "⚠️.*Warning: This is a simulated warning"; then
  echo "✅ Log plugin test 4 passed: warning message found"
else
  echo "❌ Log plugin test 4 failed: warning message not found"
  exit 1
fi

if echo "$OUTPUT" | grep -q "✅.*Test.*completed successfully"; then
  echo "✅ Log plugin test 5 passed: completion message found"
else
  echo "❌ Log plugin test 5 failed: completion message not found"
  exit 1
fi

# Verify tests passed overall
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 0"; then
  echo "✅ Log plugin tests passed overall"
else
  echo "❌ Log plugin tests had failures"
  exit 1
fi

# Test 2: ERROR level logging (to ensure messages still appear)
echo "Testing log plugin with ERROR level logging (to ensure messages still appear)..."
OUTPUT=$(ROCKETSHIP_LOG=ERROR rocketship run -af examples/simple-log/rocketship.yaml)
echo "$OUTPUT"

# Verify log messages still appear with ERROR level logging
if echo "$OUTPUT" | grep -q "🚀 Starting user-service tests in staging environment"; then
  echo "✅ Log plugin ERROR level test passed: messages appear regardless of log level"
else
  echo "❌ Log plugin ERROR level test failed: messages disappeared with ERROR logging"
  exit 1
fi

if echo "$OUTPUT" | grep -q "✗ Failed Tests: 0"; then
  echo "✅ Log plugin ERROR level tests passed overall"
else
  echo "❌ Log plugin ERROR level tests had failures"
  exit 1
fi

echo "✅ All log plugin tests passed!"