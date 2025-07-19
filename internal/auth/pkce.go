package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GeneratePKCEChallenge generates a PKCE code verifier and challenge
func GeneratePKCEChallenge() (*PKCEChallenge, error) {
	// Generate code verifier (43-128 characters)
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	
	// Use URL-safe base64 encoding without padding
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	
	// Generate code challenge using SHA256
	h := sha256.New()
	h.Write([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	
	return &PKCEChallenge{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		Method:        "S256",
	}, nil
}