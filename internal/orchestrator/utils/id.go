package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID generates a random hex ID
func GenerateID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}