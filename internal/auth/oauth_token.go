package auth

import (
	"crypto/ed25519"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OAuthClaims represents the JWT claims for the WhatsApp OAuth flow.
type OAuthClaims struct {
	Nonce  string `json:"nonce"`
	PubKey string `json:"pubkey"`
	jwt.RegisteredClaims
}

// OAuthTokenGenerator creates EdDSA-signed JWTs for the WhatsApp OAuth flow.
type OAuthTokenGenerator struct {
	key      ed25519.PrivateKey
	issuer   string
	audience string
	ttl      time.Duration
}

// NewOAuthTokenGenerator creates a new generator by loading the Ed25519 key from keyPath.
func NewOAuthTokenGenerator(keyPath, issuer, audience string, ttl time.Duration) (*OAuthTokenGenerator, error) {
	key, err := LoadEdDSAKey(keyPath)
	if err != nil {
		return nil, err
	}

	return &OAuthTokenGenerator{
		key:      key,
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
	}, nil
}

// Token creates and signs a JWT with the given phone number, nonce, and user public key.
func (g *OAuthTokenGenerator) Token(phone, nonce, userPubKey string) (string, error) {
	now := time.Now()
	claims := OAuthClaims{
		Nonce:  nonce,
		PubKey: userPubKey,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   phone,
			Issuer:    g.issuer,
			Audience:  jwt.ClaimStrings{g.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(g.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(g.key)
}
