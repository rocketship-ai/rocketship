#!/bin/bash

# Test script for --env-file functionality

set -e  # Exit on any error

echo "🚀 Testing --env-file functionality..."

# Test 1: Basic env file loading
echo ""
echo "📋 Test 1: Basic env file loading..."

# Create a test env file
cat > /tmp/test.env << 'EOF'
TEST_API_URL=https://test.example.com
TEST_API_KEY=test-key-123
TEST_ENVIRONMENT=testing
TEST_TIMEOUT=5
EOF

# Create a test YAML that uses these env vars
cat > /tmp/test-env.yaml << 'EOF'
name: Env File Test
tests:
  - name: Test env vars
    steps:
      - name: Log env vars
        plugin: log
        config:
          message: "URL: {{ .env.TEST_API_URL }}, Key: {{ .env.TEST_API_KEY }}, Env: {{ .env.TEST_ENVIRONMENT }}"
      
      - name: Use timeout from env
        plugin: delay
        config:
          duration: "{{ .env.TEST_TIMEOUT }}s"
      
      - name: Verify env vars in script
        plugin: script
        config:
          language: javascript
          script: |
            // These should be replaced with actual values
            let url = "{{ .env.TEST_API_URL }}";
            let key = "{{ .env.TEST_API_KEY }}";
            let env = "{{ .env.TEST_ENVIRONMENT }}";
            
            if (url !== "https://test.example.com") {
              throw new Error(`Expected URL to be https://test.example.com, got ${url}`);
            }
            if (key !== "test-key-123") {
              throw new Error(`Expected key to be test-key-123, got ${key}`);
            }
            if (env !== "testing") {
              throw new Error(`Expected env to be testing, got ${env}`);
            }
            
            console.log("✅ All env vars loaded correctly");
EOF

echo "  → Running test with --env-file..."
OUTPUT=$(rocketship run -af /tmp/test-env.yaml --env-file /tmp/test.env)

# Verify the output contains expected values
if echo "$OUTPUT" | grep -q "URL: https://test.example.com, Key: test-key-123, Env: testing"; then
    echo "✅ Env vars loaded correctly from file"
else
    echo "❌ Env vars not loaded correctly"
    echo "Output: $OUTPUT"
    exit 1
fi

if echo "$OUTPUT" | grep -q "Step completed successfully"; then
    echo "✅ Script validation passed"
else
    echo "❌ Script validation failed"
    echo "Output: $OUTPUT"
    exit 1
fi

# Test 2: Env file with quotes and special characters
echo ""
echo "📋 Test 2: Env file with quotes and special characters..."

cat > /tmp/test-quotes.env << 'EOF'
QUOTED_VALUE="value with spaces"
SINGLE_QUOTED='single quoted'
NO_QUOTES=simple_value
EMPTY_VALUE=
SPECIAL_CHARS=test@example.com#123
PATH_VALUE=/usr/local/bin:/usr/bin
EOF

cat > /tmp/test-quotes.yaml << 'EOF'
name: Quotes Test
tests:
  - name: Test quoted values
    steps:
      - name: Verify quoted values
        plugin: script
        config:
          language: javascript
          script: |
            let quoted = "{{ .env.QUOTED_VALUE }}";
            let single = "{{ .env.SINGLE_QUOTED }}";
            let simple = "{{ .env.NO_QUOTES }}";
            let empty = "{{ .env.EMPTY_VALUE }}";
            let special = "{{ .env.SPECIAL_CHARS }}";
            let path = "{{ .env.PATH_VALUE }}";
            
            console.log("Quoted:", quoted);
            console.log("Single:", single);
            console.log("Simple:", simple);
            console.log("Empty:", empty);
            console.log("Special:", special);
            console.log("Path:", path);
            
            if (quoted !== "value with spaces") throw new Error("Quoted value incorrect");
            if (single !== "single quoted") throw new Error("Single quoted incorrect");
            if (simple !== "simple_value") throw new Error("Simple value incorrect");
            if (empty !== "") throw new Error("Empty value should be empty string");
            if (special !== "test@example.com#123") throw new Error("Special chars incorrect");
            if (!path.includes("/usr/local/bin")) throw new Error("Path value incorrect");
            
            console.log("✅ All quote types handled correctly");
EOF

echo "  → Running quotes test..."
OUTPUT=$(rocketship run -af /tmp/test-quotes.yaml --env-file /tmp/test-quotes.env)

if echo "$OUTPUT" | grep -q "Step completed successfully" && echo "$OUTPUT" | grep -q "passed"; then
    echo "✅ Quotes and special characters handled correctly"
else
    echo "❌ Quote handling failed"
    echo "Output: $OUTPUT"
    exit 1
fi

# Test 3: System env vars take precedence
echo ""
echo "📋 Test 3: System env vars precedence..."

# Set a system env var
export TEST_PRECEDENCE="system_value"

# Create env file with same var
cat > /tmp/test-precedence.env << 'EOF'
TEST_PRECEDENCE=file_value
TEST_FILE_ONLY=file_only_value
EOF

cat > /tmp/test-precedence.yaml << 'EOF'
name: Precedence Test
tests:
  - name: Test precedence
    steps:
      - name: Check values
        plugin: log
        config:
          message: "Precedence: {{ .env.TEST_PRECEDENCE }}, File only: {{ .env.TEST_FILE_ONLY }}"
EOF

echo "  → Running precedence test..."
OUTPUT=$(rocketship run -af /tmp/test-precedence.yaml --env-file /tmp/test-precedence.env)

if echo "$OUTPUT" | grep -q "Precedence: system_value, File only: file_only_value"; then
    echo "✅ System env vars correctly take precedence over file values"
else
    echo "❌ Precedence test failed"
    echo "Output: $OUTPUT"
    exit 1
fi

# Clean up
unset TEST_PRECEDENCE
rm -f /tmp/test*.env /tmp/test*.yaml

echo ""
echo "🎉 All --env-file tests passed!"