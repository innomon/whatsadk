package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadEdDSAKey loads an Ed25519 private key from a PEM file (PKCS#8)
// or a raw 64-byte seed file.
func LoadEdDSAKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Try PEM decode first
	block, _ := pem.Decode(data)
	if block != nil {
		return parseEdDSAPEM(block.Bytes)
	}

	// Fall back to raw 32-byte seed
	if len(data) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(data), nil
	}

	return nil, fmt.Errorf("key file is neither PEM-encoded nor a %d-byte raw seed", ed25519.SeedSize)
}

func parseEdDSAPEM(der []byte) (ed25519.PrivateKey, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS#8 key: %w", err)
	}

	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("PKCS#8 key is not Ed25519")
	}
	return key, nil
}

// EdDSAPublicKeyBase64 returns the base64url-encoded public key (no padding)
// for sharing with the ADK server.
func EdDSAPublicKeyBase64(key ed25519.PrivateKey) string {
	pub := key.Public().(ed25519.PublicKey)
	return base64.RawURLEncoding.EncodeToString(pub)
}
