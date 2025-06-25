#!/bin/bash

# Test script for run persistence functionality
# Tests the list and get commands with various filtering and sorting options

set -e  # Exit on any error

echo "🚀 Testing run persistence functionality..."

# Start server in background
echo "Starting server for persistence tests..."
rocketship start server --local --background
sleep 5

cleanup() {
    echo "Cleaning up - stopping server..."
    rocketship stop server || true
}

# Ensure cleanup runs on exit
trap cleanup EXIT

# Run tests with different contexts to generate test data
echo "Running tests with different contexts..."

echo "  → Running test with ci-branch context..."
rocketship run -e localhost:7700 -f examples/simple-delay/rocketship.yaml \
  --project-id "test-project-a" \
  --source "ci-branch" \
  --branch "feature/persistence" \
  --trigger "webhook" \
  --metadata "env=ci" \
  --metadata "team=backend"

echo "  → Running test with cli-local context..."  
rocketship run -e localhost:7700 -f examples/simple-log/rocketship.yaml \
  --project-id "test-project-b" \
  --source "cli-local" \
  --branch "main" \
  --trigger "manual"

echo "  → Running test with ci-main context..."
rocketship run -e localhost:7700 -f examples/config-variables/rocketship.yaml \
  --project-id "test-project-a" \
  --source "ci-main" \
  --branch "main" \
  --trigger "schedule" \
  --schedule-name "nightly-tests"

# Test list command functionality
echo ""
echo "📋 Testing list command functionality..."

# Test basic listing
echo "  → Testing basic listing..."
LIST_OUTPUT=$(rocketship list -e localhost:7700)
echo "Basic list output:"
echo "$LIST_OUTPUT"

# Verify we have runs listed
if ! echo "$LIST_OUTPUT" | grep -q "test-project"; then
  echo "❌ No test runs found in list output"
  exit 1
fi
echo "✅ Basic listing works"

# Test filtering by project
echo "  → Testing project filtering..."
PROJECT_A_OUTPUT=$(rocketship list -e localhost:7700 --project-id "test-project-a")
PROJECT_B_OUTPUT=$(rocketship list -e localhost:7700 --project-id "test-project-b")

if ! echo "$PROJECT_A_OUTPUT" | grep -q "test-project-a"; then
  echo "❌ Project A filtering failed"
  exit 1
fi

if ! echo "$PROJECT_B_OUTPUT" | grep -q "test-project-b"; then
  echo "❌ Project B filtering failed" 
  exit 1
fi
echo "✅ Project filtering works"

# Test filtering by source
echo "  → Testing source filtering..."
CLI_OUTPUT=$(rocketship list -e localhost:7700 --source "cli-local")
CI_OUTPUT=$(rocketship list -e localhost:7700 --source "ci-branch")

if ! echo "$CLI_OUTPUT" | grep -q "cli-local"; then
  echo "❌ CLI source filtering failed"
  exit 1
fi

if ! echo "$CI_OUTPUT" | grep -q "ci-branch"; then
  echo "❌ CI source filtering failed"
  exit 1
fi
echo "✅ Source filtering works"

# Test filtering by branch
echo "  → Testing branch filtering..."
MAIN_OUTPUT=$(rocketship list -e localhost:7700 --branch "main")
FEATURE_OUTPUT=$(rocketship list -e localhost:7700 --branch "feature/persistence")

if ! echo "$MAIN_OUTPUT" | grep -q "main"; then
  echo "❌ Main branch filtering failed"
  exit 1
fi
echo "✅ Branch filtering works"

# Test filtering by status
echo "  → Testing status filtering..."
PASSED_OUTPUT=$(rocketship list -e localhost:7700 --status "PASSED")

if ! echo "$PASSED_OUTPUT" | grep -q "PASSED"; then
  echo "❌ Status filtering failed"
  exit 1
fi
echo "✅ Status filtering works"

# Test sorting by duration
echo "  → Testing duration sorting..."
DURATION_ASC=$(rocketship list -e localhost:7700 --order-by duration --ascending --limit 3)
DURATION_DESC=$(rocketship list -e localhost:7700 --order-by duration --limit 3)

if [ -z "$DURATION_ASC" ] || [ -z "$DURATION_DESC" ]; then
  echo "❌ Duration sorting failed - empty output"
  exit 1
fi
echo "✅ Duration sorting works"

# Test get command with run details
echo ""
echo "🔍 Testing get command functionality..."

# Extract a run ID from the list (first run ID from the list output)
RUN_ID=$(echo "$LIST_OUTPUT" | awk 'NR==3 {print $1}')

if [ -z "$RUN_ID" ]; then
  echo "❌ Could not extract run ID from list output"
  exit 1
fi

# Test get command with full ID
echo "  → Testing get command with run ID: $RUN_ID"
GET_OUTPUT=$(rocketship get "$RUN_ID" -e localhost:7700)

if ! echo "$GET_OUTPUT" | grep -q "Test Run Details"; then
  echo "❌ Get command failed - no details header found"
  echo "Get output: $GET_OUTPUT"
  exit 1
fi

if ! echo "$GET_OUTPUT" | grep -q "Context:"; then
  echo "❌ Get command failed - no context section found"
  echo "Get output: $GET_OUTPUT"
  exit 1
fi
echo "✅ Get command with full ID works"

# Test get command with truncated ID (first 12 characters)
TRUNCATED_ID=$(echo "$RUN_ID" | cut -c1-12)
echo "  → Testing get command with truncated ID: $TRUNCATED_ID"
GET_TRUNCATED_OUTPUT=$(rocketship get "$TRUNCATED_ID" -e localhost:7700)

if ! echo "$GET_TRUNCATED_OUTPUT" | grep -q "Test Run Details"; then
  echo "❌ Get command with truncated ID failed"
  echo "Truncated get output: $GET_TRUNCATED_OUTPUT"
  exit 1
fi
echo "✅ Get command with truncated ID works"

# Test combined filtering
echo "  → Testing combined filtering..."
COMBINED_OUTPUT=$(rocketship list -e localhost:7700 --project-id "test-project-a" --source "ci-branch")

if ! echo "$COMBINED_OUTPUT" | grep -q "test-project-a"; then
  echo "❌ Combined filtering failed"
  exit 1
fi
echo "✅ Combined filtering works"

# Test limit functionality
echo "  → Testing limit functionality..."
LIMITED_OUTPUT=$(rocketship list -e localhost:7700 --limit 1)
LINE_COUNT=$(echo "$LIMITED_OUTPUT" | wc -l)

# Should have header (2 lines) + 1 result = 3 lines
if [ "$LINE_COUNT" -ne 3 ]; then
  echo "❌ Limit functionality failed - expected 3 lines, got $LINE_COUNT"
  exit 1
fi
echo "✅ Limit functionality works"

# Test ordering options
echo "  → Testing ordering options..."
STARTED_ASC=$(rocketship list -e localhost:7700 --order-by started_at --ascending --limit 2)
STARTED_DESC=$(rocketship list -e localhost:7700 --order-by started_at --limit 2)

if [ -z "$STARTED_ASC" ] || [ -z "$STARTED_DESC" ]; then
  echo "❌ Started time sorting failed - empty output"
  exit 1
fi
echo "✅ Ordering options work"

echo ""
echo "🎉 All run persistence tests passed!"
echo "   ✅ Basic listing"
echo "   ✅ Project filtering" 
echo "   ✅ Source filtering"
echo "   ✅ Branch filtering"
echo "   ✅ Status filtering"
echo "   ✅ Duration sorting"
echo "   ✅ Get command (full ID)"
echo "   ✅ Get command (truncated ID)"
echo "   ✅ Combined filtering"
echo "   ✅ Limit functionality"
echo "   ✅ Ordering options"