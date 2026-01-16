#!/bin/bash
set -e

echo "Running Supabase error handling tests (these should fail)..."
# Use env file for local testing; in CI the env var is set by workflow
if [ -f ".rocketship/failure-cases/tmp-env/supabase-error-handling.env" ]; then
    OUTPUT=$(rocketship run -af .rocketship/failure-cases/.rocketship/supabase-error-handling.yaml --env-file .rocketship/failure-cases/tmp-env/supabase-error-handling.env 2>&1 || true)
else
    OUTPUT=$(rocketship run -af .rocketship/failure-cases/.rocketship/supabase-error-handling.yaml 2>&1 || true)
fi

echo "Test output:"
echo "$OUTPUT"
echo ""

# Check that exactly 5 tests failed
if echo "$OUTPUT" | grep -q "✗ Failed Tests: 5"; then
    echo "✅ Exactly 5 tests failed as expected"
else
    echo "❌ Expected exactly 5 tests to fail"
    echo "Output should contain '✗ Failed Tests: 5'"
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

# Test 5: Failed save operation (non-existent JSON path)
if echo "$OUTPUT" | grep -q 'error evaluating JSON path .does.not.exist.at.all'; then
    echo "✅ Test 5: Found 'error evaluating JSON path' error (save failure)"
    ERRORS_FOUND=$((ERRORS_FOUND + 1))
else
    echo "❌ Test 5: Missing 'error evaluating JSON path' error message"
fi

echo ""

# Verify we found all 5 error types
if [ "$ERRORS_FOUND" -eq 5 ]; then
    echo "✅ All 5 error types properly surfaced by Supabase plugin"
    echo "   - 401 Unauthorized (Invalid API key)"
    echo "   - 404 Not Found (Non-existent table)"
    echo "   - 404 Not Found (Non-existent RPC function)"
    echo "   - 409 Conflict (Duplicate key constraint)"
    echo "   - Save failure (Non-existent JSON path)"
    echo ""
    echo "✅ Supabase error handling test completed successfully"
    echo "   The plugin correctly fails when Supabase API returns errors"
    echo "   and when required save operations cannot extract values"
else
    echo "❌ Only found $ERRORS_FOUND/5 expected error types"
    echo "All 5 error types must be properly surfaced"
    exit 1
fi
