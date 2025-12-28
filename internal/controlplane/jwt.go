package controlplane

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Signer struct {
	method jwt.SigningMethod
	rsaKey *rsa.PrivateKey
	ecKey  *ecdsa.PrivateKey
	keyID  string
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid,omitempty"`
	Alg string `json:"alg,omitempty"`
	Use string `json:"use,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
	Crv string `json:"crv,omitempty"`
}

func NewSignerFromPEM(path, keyID string) (*Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read signing key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Try PKCS8 first, then specific formats.
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		return buildSigner(key, keyID)
	}

	if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return buildSigner(rsaKey, keyID)
	}
	if ecKey, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return buildSigner(ecKey, keyID)
	}

	return nil, errors.New("unsupported signing key format")
}

func buildSigner(key interface{}, keyID string) (*Signer, error) {
	signer := &Signer{keyID: keyID}
	switch k := key.(type) {
	case *rsa.PrivateKey:
		signer.method = jwt.SigningMethodRS256
		signer.rsaKey = k
		if signer.keyID == "" {
			signer.keyID = fmt.Sprintf("rsa-%d", k.N.BitLen())
		}
	case *ecdsa.PrivateKey:
		curveName := curveName(k.Curve)
		if curveName == "" {
			return nil, errors.New("unsupported elliptic curve")
		}
		switch curveName {
		case "P-256":
			signer.method = jwt.SigningMethodES256
		case "P-384":
			signer.method = jwt.SigningMethodES384
		case "P-521":
			signer.method = jwt.SigningMethodES512
		default:
			return nil, errors.New("unsupported ECDSA curve")
		}
		signer.ecKey = k
		if signer.keyID == "" {
			signer.keyID = fmt.Sprintf("ec-%s", strings.ToLower(curveName))
		}
	default:
		return nil, errors.New("unsupported signing key type")
	}
	return signer, nil
}

func (s *Signer) Sign(claims jwt.MapClaims) (string, error) {
	if claims == nil {
		claims = jwt.MapClaims{}
	}
	now := time.Now().Unix()
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = now
	}
	token := jwt.NewWithClaims(s.method, claims)
	if s.keyID != "" {
		token.Header["kid"] = s.keyID
	}

	var signed string
	var err error
	if s.rsaKey != nil {
		signed, err = token.SignedString(s.rsaKey)
	} else if s.ecKey != nil {
		signed, err = token.SignedString(s.ecKey)
	} else {
		return "", errors.New("no signing key configured")
	}
	if err != nil {
		return "", err
	}
	return signed, nil
}

func (s *Signer) JWKS() (JWKS, error) {
	switch {
	case s.rsaKey != nil:
		return JWKS{Keys: []JWK{rsaPublicJWK(&s.rsaKey.PublicKey, s.keyID)}}, nil
	case s.ecKey != nil:
		return JWKS{Keys: []JWK{ecPublicJWK(&s.ecKey.PublicKey, s.keyID)}}, nil
	default:
		return JWKS{}, errors.New("no signing key configured")
	}
}

func (j JWKS) JSON() ([]byte, error) {
	return json.Marshal(j)
}

func rsaPublicJWK(pub *rsa.PublicKey, kid string) JWK {
	return JWK{
		Kty: "RSA",
		Kid: kid,
		Use: "sig",
		Alg: "RS256",
		N:   encodeBig(pub.N),
		E:   encodeBig(big.NewInt(int64(pub.E))),
	}
}

func ecPublicJWK(pub *ecdsa.PublicKey, kid string) JWK {
	curve := curveName(pub.Curve)
	return JWK{
		Kty: "EC",
		Kid: kid,
		Use: "sig",
		Alg: ecAlg(curve),
		Crv: curve,
		X:   encodeBig(pub.X),
		Y:   encodeBig(pub.Y),
	}
}

func encodeBig(i *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(i.Bytes())
}

func curveName(curve elliptic.Curve) string {
	switch curve {
	case elliptic.P256():
		return "P-256"
	case elliptic.P384():
		return "P-384"
	case elliptic.P521():
		return "P-521"
	default:
		return ""
	}
}

func ecAlg(curve string) string {
	switch curve {
	case "P-256":
		return "ES256"
	case "P-384":
		return "ES384"
	case "P-521":
		return "ES512"
	default:
		return "ES256"
	}
}
