#!/bin/bash
set -e

echo "Testing browser plugin assertion failure behavior..."

# Create a mock browser test file that should fail due to assertion failure
cat > /tmp/browser-assertion-test.yaml << 'EOF'
name: "Browser Assertion Failure Test"
tests:
  - name: "Test that should fail assertion"
    steps:
      - name: "Mock browser step with failing assertion"
        plugin: browser
        config:
          task: |
            This is a mock browser task for testing assertion failure behavior.
            The browser automation will not actually run - this test should fail at the assertion step.
            Return success: false to test assertion validation.
          llm:
            provider: "openai"  
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "mock-key-for-testing"
          executor_type: "python"
          headless: true
          timeout: "30s"
          max_steps: 1
          use_vision: false
        save:
          - json_path: ".success"
            as: "browser_success"
        assertions:
          - type: "json_path"
            path: ".success"
            expected: true  # This should fail because the mock will return success: false
EOF

echo "Running assertion failure test..."
OUTPUT=$(rocketship run -af /tmp/browser-assertion-test.yaml 2>&1 || true)

echo "Test output:"
echo "$OUTPUT"

# Check if the test properly failed with assertion error
if echo "$OUTPUT" | grep -q "assertion failed"; then
    echo "✅ Browser plugin assertion failure test PASSED - assertions are properly validated"
    
    # Also check that the test was marked as failed
    if echo "$OUTPUT" | grep -q "✗ Failed Tests: 1"; then
        echo "✅ Test was correctly marked as failed"
    else
        echo "❌ Test was not correctly marked as failed"
        exit 1
    fi
else
    echo "❌ Browser plugin assertion failure test FAILED - assertions are not being validated"
    echo "Expected to see 'assertion failed' in the output but didn't find it"
    exit 1
fi

echo "✅ Browser assertion validation test completed successfully"