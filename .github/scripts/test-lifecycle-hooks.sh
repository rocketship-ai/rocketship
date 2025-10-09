#!/bin/bash
set -e

echo "Running lifecycle hooks integration tests..."
echo ""

# Helper function for logging
log() {
  printf '[lifecycle-hooks] %s\n' "$1"
}

# Track test results
TESTS_PASSED=0
TESTS_FAILED=0

# Test 1: Suite-level hooks
log "Test 1: Running suite-level hooks example..."
if [ -f "examples/lifecycle-hooks/.env" ]; then
    OUTPUT=$(rocketship run -af examples/lifecycle-hooks/suite-level-hooks.yaml --env-file examples/lifecycle-hooks/.env 2>&1)
else
    OUTPUT=$(rocketship run -af examples/lifecycle-hooks/suite-level-hooks.yaml 2>&1)
fi

echo "$OUTPUT"
echo ""

if echo "$OUTPUT" | grep -q "✓ Passed Tests: 3"; then
    log "✅ Test 1 PASSED: Suite-level hooks working correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))

    # Verify suite init ran
    if echo "$OUTPUT" | grep -q "Suite init completed"; then
        log "  ✅ Suite init executed"
    else
        log "  ⚠️ Suite init log message missing"
    fi

    # Verify cleanup steps ran
    if echo "$OUTPUT" | grep -q "Delete shared company"; then
        log "  ✅ Suite cleanup executed"
    else
        log "  ⚠️ Suite cleanup message missing"
    fi
else
    log "❌ Test 1 FAILED: Suite-level hooks test failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""

# Test 2: Test-level hooks
log "Test 2: Running test-level hooks example..."
if [ -f "examples/lifecycle-hooks/.env" ]; then
    OUTPUT=$(rocketship run -af examples/lifecycle-hooks/test-level-hooks.yaml --env-file examples/lifecycle-hooks/.env 2>&1)
else
    OUTPUT=$(rocketship run -af examples/lifecycle-hooks/test-level-hooks.yaml 2>&1)
fi

echo "$OUTPUT"
echo ""

if echo "$OUTPUT" | grep -q "✓ Passed Tests: 2"; then
    log "✅ Test 2 PASSED: Test-level hooks working correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))

    # Verify test init steps ran
    if echo "$OUTPUT" | grep -q "Setup test-specific company"; then
        log "  ✅ Test init executed"
    else
        log "  ⚠️ Test init message missing"
    fi

    # Verify test cleanup ran
    if echo "$OUTPUT" | grep -q "Delete test company"; then
        log "  ✅ Test cleanup executed"
    else
        log "  ⚠️ Test cleanup message missing"
    fi
else
    log "❌ Test 2 FAILED: Test-level hooks test failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""

# Test 3: Combined hooks
log "Test 3: Running combined suite and test hooks example..."
if [ -f "examples/lifecycle-hooks/.env" ]; then
    OUTPUT=$(rocketship run -af examples/lifecycle-hooks/combined-hooks.yaml --env-file examples/lifecycle-hooks/.env 2>&1)
else
    OUTPUT=$(rocketship run -af examples/lifecycle-hooks/combined-hooks.yaml 2>&1)
fi

echo "$OUTPUT"
echo ""

if echo "$OUTPUT" | grep -q "✓ Passed Tests: 2"; then
    log "✅ Test 3 PASSED: Combined hooks working correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))

    # Verify suite init ran
    if echo "$OUTPUT" | grep -q "Create storage bucket for test suite"; then
        log "  ✅ Suite init executed"
    else
        log "  ⚠️ Suite init message missing"
    fi

    # Verify test init ran
    if echo "$OUTPUT" | grep -q "Create test-specific company"; then
        log "  ✅ Test init executed"
    else
        log "  ⚠️ Test init message missing"
    fi

    # Verify both cleanups ran
    if echo "$OUTPUT" | grep -q "Delete test company"; then
        log "  ✅ Test cleanup executed"
    else
        log "  ⚠️ Test cleanup message missing"
    fi

    if echo "$OUTPUT" | grep -q "Delete storage bucket"; then
        log "  ✅ Suite cleanup executed"
    else
        log "  ⚠️ Suite cleanup message missing"
    fi
else
    log "❌ Test 3 FAILED: Combined hooks test failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""
echo "=========================================="
echo "Lifecycle Hooks Test Summary"
echo "=========================================="
echo "✅ Tests Passed: $TESTS_PASSED"
echo "❌ Tests Failed: $TESTS_FAILED"
echo ""

if [ "$TESTS_FAILED" -eq 0 ]; then
    log "✅ All lifecycle hooks tests passed successfully"
    log ""
    log "Verified features:"
    log "  - Suite init creates shared resources"
    log "  - Suite globals available in all tests"
    log "  - Test init creates test-specific resources"
    log "  - Test variables isolated per test"
    log "  - Test cleanup removes test resources"
    log "  - Suite cleanup removes shared resources"
    log "  - Combined hooks work together correctly"
    exit 0
else
    log "❌ Some lifecycle hooks tests failed"
    exit 1
fi
