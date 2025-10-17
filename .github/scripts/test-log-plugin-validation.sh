#!/bin/bash

set -e

echo "Testing log plugin validation..."

# Create a temporary invalid test file
mkdir -p /tmp/invalid-log-test
cat > /tmp/invalid-log-test/rocketship.yaml << 'EOF'
name: "Invalid Log Test"
tests:
  - name: "Test invalid log config"
    steps:
      - name: "Log without message"
        plugin: "log"
        config:
          # Missing required message field
          level: "INFO"
EOF

# Test that validation catches missing message field
echo "Testing validation of invalid log plugin config (missing message)..."
if rocketship validate /tmp/invalid-log-test.yaml 2>&1 | grep -q "validation failed"; then
  echo "✅ Log plugin validation test passed: missing message field caught"
else
  echo "❌ Log plugin validation test failed: missing message field not caught"
  rm -rf /tmp/invalid-log-test /tmp/valid-log-test
  exit 1
fi

# Clean up
rm -rf /tmp/invalid-log-test /tmp/valid-log-test

# Create a valid test file to ensure validation passes
mkdir -p /tmp/valid-log-test
cat > /tmp/valid-log-test/rocketship.yaml << 'EOF'
name: "Valid Log Test"
tests:
  - name: "Test valid log config"
    steps:
      - name: "Valid log step"
        plugin: "log"
        config:
          message: "This is a valid log message"
EOF

# Test that validation passes for valid config
echo "Testing validation of valid log plugin config..."
if rocketship validate /tmp/valid-log-test.yaml 2>&1 | grep -q "validation passed"; then
  echo "✅ Log plugin validation test passed: valid config accepted"
else
  echo "❌ Log plugin validation test failed: valid config rejected"
  rm -rf /tmp/invalid-log-test /tmp/valid-log-test
  exit 1
fi

# Clean up
rm -rf /tmp/invalid-log-test /tmp/valid-log-test

echo "✅ All log plugin validation tests passed!"