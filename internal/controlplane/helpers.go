package controlplane

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// Constants for verification and slug generation
const (
	orgRegistrationTTL         = time.Hour
	orgRegistrationResendDelay = time.Minute
	verificationCodeLength     = 6
	maxOrgNameLength           = 120
	maxInviteEmailLength       = 320
	defaultSlugSuffixLength    = 4
	maxRegistrationAttempts    = 5
)

// HTTP response writers

// writeJSON writes a JSON response with the given status code
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError writes an error response with the given status code
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeOAuthError writes an OAuth-formatted error response
func writeOAuthError(w http.ResponseWriter, code, description string) {
	payload := map[string]string{"error": code}
	if description != "" {
		payload["error_description"] = description
	}
	writeJSON(w, http.StatusBadRequest, payload)
}

// Verification code generation and validation

// newVerificationSecret generates a random numeric verification code with salt and hash
func newVerificationSecret(length int) (code string, salt, hash []byte, err error) {
	if length <= 0 {
		length = verificationCodeLength
	}
	const digits = "0123456789"
	buf := make([]byte, length)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, nil, err
	}
	b := make([]byte, length)
	for i := range buf {
		b[i] = digits[int(buf[i])%len(digits)]
	}
	code = string(b)

	salt = make([]byte, 16)
	if _, err = rand.Read(salt); err != nil {
		return "", nil, nil, err
	}
	sum := sha256.Sum256(append(salt, []byte(code)...))
	hash = sum[:]
	return code, salt, hash, nil
}

// verifyCode validates a verification code against its salt and hash using constant-time comparison
func verifyCode(code string, salt, hash []byte) bool {
	sum := sha256.Sum256(append(salt, []byte(strings.TrimSpace(code))...))
	return subtle.ConstantTimeCompare(hash, sum[:]) == 1
}

// Slug generation

// slugifyName converts a name into a URL-safe slug
func slugifyName(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var builder strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = "org"
	}
	return slug
}

// randomSuffix generates a random alphanumeric suffix of the given length
func randomSuffix(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	if length <= 0 {
		length = defaultSlugSuffixLength
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}

// Token generation

// generateRandomToken generates a secure random token using base64url encoding
func generateRandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Utility functions

// copyScopes creates a deep copy of a scopes slice
func copyScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	c := make([]string, len(scopes))
	copy(c, scopes)
	return c
}

// containsRole checks if a role exists in the roles slice (case-insensitive)
func containsRole(roles []string, role string) bool {
	role = strings.ToLower(role)
	for _, r := range roles {
		if strings.ToLower(r) == role {
			return true
		}
	}
	return false
}

// joinScopes joins a slice of scopes into a space-separated string
func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	return strings.Join(scopes, " ")
}

// selectPrimaryOrg chooses the primary organization from a role summary
func selectPrimaryOrg(summary persistence.RoleSummary) uuid.UUID {
	for _, org := range summary.Organizations {
		if org.IsAdmin {
			return org.OrganizationID
		}
	}
	for _, org := range summary.Organizations {
		return org.OrganizationID
	}
	for _, project := range summary.Projects {
		if project.OrganizationID != uuid.Nil {
			return project.OrganizationID
		}
	}
	return uuid.Nil
}

// JWT claim parsing helpers

// stringClaim extracts a string value from a JWT claim
func stringClaim(value interface{}) string {
	s, _ := value.(string)
	return strings.TrimSpace(s)
}

// stringSliceFromClaim converts a JWT claim value into a string slice
func stringSliceFromClaim(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
		return out, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, errors.New("roles must be strings")
			}
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, nil
		}
		return []string{trimmed}, nil
	default:
		return nil, fmt.Errorf("unexpected roles type %T", value)
	}
}

// matchAudience checks if the given audience value matches the expected audience
func matchAudience(aud interface{}, expected string) bool {
	switch v := aud.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), expected)
	case []string:
		for _, item := range v {
			if matchAudience(item, expected) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if matchAudience(item, expected) {
				return true
			}
		}
	}
	return false
}
