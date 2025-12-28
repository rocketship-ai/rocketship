package authbroker

import (
	"testing"
)

func TestNormalizeSourceRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRef  string
		wantKind RefKind
	}{
		{
			name:     "branch ref with refs/heads prefix",
			input:    "refs/heads/main",
			wantRef:  "main",
			wantKind: RefKindBranch,
		},
		{
			name:     "branch ref with nested path",
			input:    "refs/heads/feature/foo",
			wantRef:  "feature/foo",
			wantKind: RefKindBranch,
		},
		{
			name:     "PR head ref",
			input:    "refs/pull/123/head",
			wantRef:  "pr/123",
			wantKind: RefKindPR,
		},
		{
			name:     "PR merge ref",
			input:    "refs/pull/456/merge",
			wantRef:  "pr/456",
			wantKind: RefKindPR,
		},
		{
			name:     "full SHA",
			input:    "abc123def456789012345678901234567890abcd",
			wantRef:  "abc123def456789012345678901234567890abcd",
			wantKind: RefKindSHA,
		},
		{
			name:     "full SHA uppercase normalized to lowercase",
			input:    "ABC123DEF456789012345678901234567890ABCD",
			wantRef:  "abc123def456789012345678901234567890abcd",
			wantKind: RefKindSHA,
		},
		{
			name:     "tag ref returns tag name",
			input:    "refs/tags/v1.0.0",
			wantRef:  "v1.0.0",
			wantKind: RefKindUnknown,
		},
		{
			name:     "plain branch name without prefix",
			input:    "main",
			wantRef:  "main",
			wantKind: RefKindUnknown,
		},
		{
			name:     "empty string",
			input:    "",
			wantRef:  "",
			wantKind: RefKindUnknown,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			wantRef:  "",
			wantKind: RefKindUnknown,
		},
		{
			name:     "whitespace trimmed",
			input:    "  refs/heads/main  ",
			wantRef:  "main",
			wantKind: RefKindBranch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSourceRef(tt.input)
			if result.Ref != tt.wantRef {
				t.Errorf("NormalizeSourceRef(%q).Ref = %q, want %q", tt.input, result.Ref, tt.wantRef)
			}
			if result.Kind != tt.wantKind {
				t.Errorf("NormalizeSourceRef(%q).Kind = %q, want %q", tt.input, result.Kind, tt.wantKind)
			}
			if result.Raw != tt.input {
				t.Errorf("NormalizeSourceRef(%q).Raw = %q, want %q", tt.input, result.Raw, tt.input)
			}
		})
	}
}

func TestNormalizedRef_Helpers(t *testing.T) {
	branchRef := NormalizeSourceRef("refs/heads/main")
	if !branchRef.IsBranch() {
		t.Error("expected IsBranch() to return true for branch ref")
	}
	if branchRef.IsPR() {
		t.Error("expected IsPR() to return false for branch ref")
	}
	if branchRef.IsSHA() {
		t.Error("expected IsSHA() to return false for branch ref")
	}

	prRef := NormalizeSourceRef("refs/pull/42/head")
	if prRef.IsBranch() {
		t.Error("expected IsBranch() to return false for PR ref")
	}
	if !prRef.IsPR() {
		t.Error("expected IsPR() to return true for PR ref")
	}
	if prRef.IsSHA() {
		t.Error("expected IsSHA() to return false for PR ref")
	}

	shaRef := NormalizeSourceRef("abc123def456789012345678901234567890abcd")
	if shaRef.IsBranch() {
		t.Error("expected IsBranch() to return false for SHA ref")
	}
	if shaRef.IsPR() {
		t.Error("expected IsPR() to return false for SHA ref")
	}
	if !shaRef.IsSHA() {
		t.Error("expected IsSHA() to return true for SHA ref")
	}
}
