package authbroker

import (
	"regexp"
	"strings"
)

// RefKind indicates the type of git reference
type RefKind string

const (
	RefKindBranch  RefKind = "branch"
	RefKindPR      RefKind = "pr"
	RefKindSHA     RefKind = "sha"
	RefKindUnknown RefKind = "unknown"
)

// NormalizedRef contains the normalized reference and its kind
type NormalizedRef struct {
	Ref  string  // Normalized reference (e.g., "main", "pr/123", or SHA)
	Kind RefKind // Type of reference
	Raw  string  // Original raw value
}

var (
	// shaPattern matches a 40-character hex string (full SHA)
	shaPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

	// prRefPattern matches refs/pull/<number>/head or refs/pull/<number>/merge
	prRefPattern = regexp.MustCompile(`^refs/pull/(\d+)/(head|merge)$`)
)

// NormalizeSourceRef normalizes a GitHub ref to a canonical format.
//
// Rules:
//   - refs/heads/<branch> -> <branch> (kind=branch)
//   - refs/pull/<num>/head or refs/pull/<num>/merge -> pr/<num> (kind=pr)
//   - 40-character hex string -> unchanged (kind=sha)
//   - Otherwise -> trimmed input (kind=unknown)
func NormalizeSourceRef(eventRef string) NormalizedRef {
	ref := strings.TrimSpace(eventRef)
	if ref == "" {
		return NormalizedRef{Ref: "", Kind: RefKindUnknown, Raw: eventRef}
	}

	// Check for refs/heads/<branch>
	if strings.HasPrefix(ref, "refs/heads/") {
		branch := strings.TrimPrefix(ref, "refs/heads/")
		return NormalizedRef{Ref: branch, Kind: RefKindBranch, Raw: eventRef}
	}

	// Check for refs/pull/<num>/head or refs/pull/<num>/merge
	if matches := prRefPattern.FindStringSubmatch(ref); len(matches) == 3 {
		prNum := matches[1]
		return NormalizedRef{Ref: "pr/" + prNum, Kind: RefKindPR, Raw: eventRef}
	}

	// Check for refs/tags/<tag> - treat as unknown for now, caller can handle
	if strings.HasPrefix(ref, "refs/tags/") {
		tag := strings.TrimPrefix(ref, "refs/tags/")
		return NormalizedRef{Ref: tag, Kind: RefKindUnknown, Raw: eventRef}
	}

	// Check for full SHA (40 hex characters)
	if shaPattern.MatchString(ref) {
		return NormalizedRef{Ref: strings.ToLower(ref), Kind: RefKindSHA, Raw: eventRef}
	}

	// Return trimmed input as-is (could be a plain branch name without refs/heads/)
	return NormalizedRef{Ref: ref, Kind: RefKindUnknown, Raw: eventRef}
}

// IsBranch returns true if the reference is a branch
func (n NormalizedRef) IsBranch() bool {
	return n.Kind == RefKindBranch
}

// IsPR returns true if the reference is a pull request
func (n NormalizedRef) IsPR() bool {
	return n.Kind == RefKindPR
}

// IsSHA returns true if the reference is a commit SHA
func (n NormalizedRef) IsSHA() bool {
	return n.Kind == RefKindSHA
}
