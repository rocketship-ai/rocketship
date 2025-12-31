// gen-worker-token generates a long-lived JWT for the worker service account
// Usage: go run scripts/gen-worker-token/main.go -key <path-to-signing-key.pem> -issuer <issuer> -audience <audience> [-ttl <duration>]
package main

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func main() {
	keyPath := flag.String("key", "", "Path to PEM-encoded signing key")
	issuer := flag.String("issuer", "", "JWT issuer (e.g., http://auth.minikube.local)")
	audience := flag.String("audience", "", "JWT audience (e.g., rocketship-cli)")
	ttl := flag.Duration("ttl", 30*24*time.Hour, "Token TTL (default: 30 days)")
	flag.Parse()

	if *keyPath == "" || *issuer == "" || *audience == "" {
		fmt.Fprintln(os.Stderr, "Usage: gen-worker-token -key <path> -issuer <issuer> -audience <audience> [-ttl <duration>]")
		os.Exit(1)
	}

	token, err := generateWorkerToken(*keyPath, *issuer, *audience, *ttl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(token)
}

func generateWorkerToken(keyPath, issuer, audience string, ttl time.Duration) (string, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read signing key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return "", errors.New("failed to decode PEM block")
	}

	// Try PKCS8 first, then specific formats
	var signingMethod jwt.SigningMethod
	var signingKey interface{}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 (RSA)
		rsaKey, rsaErr := x509.ParsePKCS1PrivateKey(block.Bytes)
		if rsaErr == nil {
			key = rsaKey
		} else {
			// Try EC
			ecKey, ecErr := x509.ParseECPrivateKey(block.Bytes)
			if ecErr != nil {
				return "", errors.New("unsupported signing key format")
			}
			key = ecKey
		}
	}

	switch k := key.(type) {
	case *rsa.PrivateKey:
		signingMethod = jwt.SigningMethodRS256
		signingKey = k
	case *ecdsa.PrivateKey:
		signingMethod = jwt.SigningMethodES256
		signingKey = k
	default:
		return "", errors.New("unsupported key type")
	}

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":   issuer,
		"aud":   audience,
		"sub":   "service:worker",
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"roles": []string{"service_account"},
		"scope": "write",
	}

	token := jwt.NewWithClaims(signingMethod, claims)
	signed, err := token.SignedString(signingKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}
