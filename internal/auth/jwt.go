package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTGenerator struct {
	key      *rsa.PrivateKey
	issuer   string
	audience string
	ttl      time.Duration
}

func NewJWTGenerator(keyPath, issuer, audience string, ttl time.Duration) (*JWTGenerator, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	key, err := parseRSAPrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}

	return &JWTGenerator{
		key:      key,
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
	}, nil
}

func (g *JWTGenerator) Token(userID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:  userID,
		Channel: "whatsapp",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(g.ttl)),
		},
	}

	if g.audience != "" {
		claims.Audience = jwt.ClaimStrings{g.audience}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.key)
}

func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key data")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("key is neither PKCS#1 nor PKCS#8: %w", err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("PKCS#8 key is not RSA")
	}
	return key, nil
}
