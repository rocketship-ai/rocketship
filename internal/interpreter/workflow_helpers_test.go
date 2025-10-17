package interpreter

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestExtractCleanError_WithTemporalWrapping(t *testing.T) {
	// Multi-line error that gets duplicated by Temporal wrapping
	multiLineErr := "python execution failed: AssertionError: expected value\nCall log:\n  - Expect \"to_have_url\""

	// Simulate Temporal's error wrapping pattern with duplication
	fullError := fmt.Sprintf("workflow execution error: step 1: %s (type: wrapError, retryable: true): step 1: %s",
		multiLineErr, multiLineErr)

	clean := ExtractCleanError(errors.New(fullError))

	// Should truncate at first (type: wrapError, retryable: true):
	if strings.Contains(clean, "(type: wrapError, retryable: true):") {
		t.Fatalf("should have removed Temporal wrapping duplication, got %q", clean)
	}

	// Should preserve multi-line content before the marker
	if !strings.Contains(clean, "Call log:") {
		t.Fatalf("should preserve multi-line content, got %q", clean)
	}

	// Should only appear once
	if strings.Count(clean, multiLineErr) != 1 {
		t.Fatalf("error should appear only once, got %q", clean)
	}
}

func TestExtractCleanError_NoWrapping(t *testing.T) {
	// Error without Temporal wrapping should be returned as-is
	simpleErr := errors.New("workflow error: step 0: sql error: connection refused")

	clean := ExtractCleanError(simpleErr)

	if clean != simpleErr.Error() {
		t.Fatalf("expected full error when no wrapping marker, got %q", clean)
	}
}

func TestExtractCleanErrorNil(t *testing.T) {
	if ExtractCleanError(nil) != "" {
		t.Fatalf("expected empty string for nil error")
	}
}
