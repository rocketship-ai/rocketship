#!/bin/bash

# External shell script demonstrating file support and features
set -e

# Test variable substitution from config
PROJECT="{{ .vars.project_name }}"
TARGET="{{ .vars.build_target }}"

# Test environment variables from previous steps
test -n "${ROCKETSHIP_VAR_PROJECT_NAME:-}"
test -n "${ROCKETSHIP_VAR_BUILD_TARGET:-}"

# Test file operations
TEMP_FILE="/tmp/rocketship_external_test.txt"
echo "External script test content - $PROJECT" > "$TEMP_FILE"
test -f "$TEMP_FILE"

# Test conditional logic
if [ "$TARGET" = "production" ]; then
    ENVIRONMENT_TYPE="production"
else
    ENVIRONMENT_TYPE="development"
fi

# Test command substitution and variables
CURRENT_TIME=$(date +"%Y-%m-%d %H:%M:%S")
SCRIPT_PID=$$
WORKING_DIR=$(pwd)

# Test arithmetic
FILE_SIZE=$(stat -f%z "$TEMP_FILE" 2>/dev/null || stat -c%s "$TEMP_FILE" 2>/dev/null || echo "0")
test "$FILE_SIZE" -gt 0

# Clean up
rm -f "$TEMP_FILE"

# Test multi-line processing
for env in development staging production; do
    if [ "$env" = "$TARGET" ]; then
        MATCHED_ENV="$env"
        break
    fi
done

test -n "$MATCHED_ENV"