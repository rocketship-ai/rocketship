#!/bin/bash
set -e

echo "Testing Supabase plugin error handling behavior..."

# Determine rocketship binary path
if command -v rocketship &> /dev/null; then
    ROCKETSHIP_CMD="rocketship"
elif [ -f "$HOME/go/bin/rocketship" ]; then
    ROCKETSHIP_CMD="$HOME/go/bin/rocketship"
else
    echo "❌ rocketship binary not found"
    exit 1
fi

echo "Running Supabase error handling tests (these should fail)..."
OUTPUT=$($ROCKETSHIP_CMD run -af examples/supabase-testing/error-handling.yaml 2>&1 || true)

echo "Test output:"
echo "$OUTPUT"
echo ""

# Check that exactly 4 tests failed
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 4"; then
    echo "✅ Exactly 4 tests failed as expected"
else
    echo "❌ Expected exactly 4 tests to fail"
    echo "Output should contain '✗ Failed Tests: 4'"
    exit 1
fi

# Track which errors we found
ERRORS_FOUND=0

# Test 1: Invalid API key error (401)
if echo "$OUTPUT" | grep -q "supabase api error (status 401): Invalid API key"; then
    echo "✅ Test 1: Found 'Invalid API key' error (401)"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    echo "❌ Test 1: Missing 'Invalid API key' error message"
fi

# Test 2: Non-existent table error (404)
if echo "$OUTPUT" | grep -q 'relation "public.this_table_does_not_exist_12345" does not exist'; then
    echo "✅ Test 2: Found 'relation does not exist' error (404)"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    echo "❌ Test 2: Missing 'relation does not exist' error message"
fi

# Test 3: Non-existent RPC function error (404)
if echo "$OUTPUT" | grep -q 'Could not find the function public.this_function_does_not_exist_12345'; then
    echo "✅ Test 3: Found 'Could not find the function' error (404)"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    echo "❌ Test 3: Missing 'Could not find the function' error message"
fi

# Test 4: Duplicate key constraint error (409)
if echo "$OUTPUT" | grep -q 'duplicate key value violates unique constraint "companies_pkey"'; then
    echo "✅ Test 4: Found 'duplicate key constraint' error (409)"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    echo "❌ Test 4: Missing 'duplicate key constraint' error message"
fi

echo ""

# Verify we found all 4 error types
if [ "$ERRORS_FOUND" -eq 4 ]; then
    echo "✅ All 4 error types properly surfaced by Supabase plugin"
    echo "   - 401 Unauthorized (Invalid API key)"
    echo "   - 404 Not Found (Non-existent table)"
    echo "   - 404 Not Found (Non-existent RPC function)"
    echo "   - 409 Conflict (Duplicate key constraint)"
    echo ""
    echo "✅ Supabase error handling test completed successfully"
    echo "   The plugin correctly fails when Supabase API returns errors"
else
    echo "❌ Only found $ERRORS_FOUND/4 expected error types"
    echo "All 4 error types must be properly surfaced"
    exit 1
fi
