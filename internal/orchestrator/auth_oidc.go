package orchestrator

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OIDCSettings defines the configuration required to enable OIDC validation.
type OIDCSettings struct {
	Issuer            string
	Audience          string
	ClientID          string
	JWKSURL           string
	DeviceEndpoint    string
	TokenEndpoint     string
	Scopes            []string
	HTTPClient        *http.Client
	AllowedAlgorithms []string
}

type oidcProvider struct {
	verifier       *oidcVerifier
	Issuer         string
	Audience       string
	ClientID       string
	Scopes         []string
	DeviceEndpoint string
	TokenEndpoint  string
}

func newOIDCProvider(ctx context.Context, settings OIDCSettings) (*oidcProvider, error) {
	if settings.Issuer == "" {
		return nil, errors.New("oidc issuer is required")
	}
	if settings.ClientID == "" {
		return nil, errors.New("oidc client id is required")
	}
	if settings.JWKSURL == "" {
		return nil, errors.New("jwks url is required")
	}

	verifier, err := newOIDCVerifier(ctx, settings)
	if err != nil {
		return nil, err
	}

	provider := &oidcProvider{
		verifier:       verifier,
		Issuer:         settings.Issuer,
		Audience:       settings.Audience,
		ClientID:       settings.ClientID,
		Scopes:         append([]string(nil), settings.Scopes...),
		DeviceEndpoint: settings.DeviceEndpoint,
		TokenEndpoint:  settings.TokenEndpoint,
	}
	return provider, nil
}

func (o *oidcProvider) Validate(ctx context.Context, bearer string) error {
	if o.verifier == nil {
		return status.Error(codes.Internal, "oidc verifier not initialised")
	}
	return o.verifier.Verify(ctx, bearer)
}

type oidcVerifier struct {
	httpClient *http.Client
	jwksURL    string
	issuer     string
	audience   string
	algorithms map[string]struct{}
	mu         sync.RWMutex
	keys       map[string]interface{}
	lastFetch  time.Time
}

func newOIDCVerifier(ctx context.Context, settings OIDCSettings) (*oidcVerifier, error) {
	httpClient := settings.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	keys, err := fetchJWKS(ctx, httpClient, settings.JWKSURL)
	if err != nil {
		return nil, err
	}

	algs := map[string]struct{}{}
	if len(settings.AllowedAlgorithms) == 0 {
		settings.AllowedAlgorithms = []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}
	}
	for _, alg := range settings.AllowedAlgorithms {
		algs[strings.ToUpper(strings.TrimSpace(alg))] = struct{}{}
	}

	return &oidcVerifier{
		httpClient: httpClient,
		jwksURL:    settings.JWKSURL,
		issuer:     settings.Issuer,
		audience:   settings.Audience,
		algorithms: algs,
		keys:       keys,
		lastFetch:  time.Now(),
	}, nil
}

func (o *oidcVerifier) Verify(ctx context.Context, token string) error {
	parser := jwt.NewParser(jwt.WithValidMethods(o.allowedAlgorithms()))
	claims := &jwt.RegisteredClaims{}
	parsed, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		return o.keyFunc(ctx, t)
	})
	if err != nil {
		return status.Error(codes.PermissionDenied, fmt.Sprintf("invalid token: %v", err))
	}
	if !parsed.Valid {
		return status.Error(codes.PermissionDenied, "invalid token")
	}

	if err := claims.Valid(); err != nil {
		return status.Error(codes.PermissionDenied, fmt.Sprintf("token validation failed: %v", err))
	}

	if o.issuer != "" && claims.Issuer != o.issuer {
		return status.Error(codes.PermissionDenied, "issuer mismatch")
	}
	if o.audience != "" {
		if !audienceContains(claims.Audience, o.audience) {
			return status.Error(codes.PermissionDenied, "audience mismatch")
		}
	}
	return nil
}

func (o *oidcVerifier) allowedAlgorithms() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	algs := make([]string, 0, len(o.algorithms))
	for alg := range o.algorithms {
		algs = append(algs, alg)
	}
	return algs
}

func (o *oidcVerifier) keyFunc(ctx context.Context, token *jwt.Token) (interface{}, error) {
	alg := ""
	if token.Method != nil {
		alg = strings.ToUpper(token.Method.Alg())
	}
	if _, ok := o.algorithms[alg]; !ok {
		return nil, fmt.Errorf("algorithm %s not allowed", alg)
	}

	kid, _ := token.Header["kid"].(string)
	key := o.lookupKey(kid)
	if key != nil {
		return key, nil
	}

	if err := o.refreshKeys(ctx); err != nil {
		return nil, err
	}
	key = o.lookupKey(kid)
	if key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("unable to resolve signing key")
}

func (o *oidcVerifier) lookupKey(kid string) interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if kid != "" {
		if key, ok := o.keys[kid]; ok {
			return key
		}
	}
	if kid == "" && len(o.keys) == 1 {
		for _, key := range o.keys {
			return key
		}
	}
	return nil
}

func (o *oidcVerifier) refreshKeys(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	keys, err := fetchJWKS(ctx, o.httpClient, o.jwksURL)
	if err != nil {
		return err
	}
	o.keys = keys
	o.lastFetch = time.Now()
	return nil
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func fetchJWKS(ctx context.Context, client *http.Client, url string) (map[string]interface{}, error) {
	if url == "" {
		return nil, errors.New("jwks url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build JWKS request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JWKS request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS request failed: %s", resp.Status)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS document: %w", err)
	}
	if len(doc.Keys) == 0 {
		return nil, errors.New("jwks document contained no keys")
	}

	keys := make(map[string]interface{}, len(doc.Keys))
	for _, k := range doc.Keys {
		if strings.ToUpper(k.Kty) != "RSA" {
			continue
		}
		pub, err := parseRSAJWK(k)
		if err != nil {
			continue
		}
		kid := k.Kid
		if kid == "" {
			kid = fmt.Sprintf("rsa_%d", len(keys)+1)
		}
		keys[kid] = pub
	}
	if len(keys) == 0 {
		return nil, errors.New("no usable RSA keys in JWKS document")
	}
	return keys, nil
}

func parseRSAJWK(j jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	var eBytes []byte
	if j.E != "" {
		eBytes, err = base64.RawURLEncoding.DecodeString(j.E)
		if err != nil {
			return nil, fmt.Errorf("failed to decode exponent: %w", err)
		}
	}
	if len(eBytes) == 0 {
		eBytes = []byte{0x01, 0x00, 0x01} // 65537
	}

	modulus := new(big.Int).SetBytes(nBytes)
	exponent := 0
	for _, b := range eBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		exponent = 65537
	}

	return &rsa.PublicKey{N: modulus, E: exponent}, nil
}

func audienceContains(list jwt.ClaimStrings, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
