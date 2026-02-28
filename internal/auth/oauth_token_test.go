package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func writeTestEdDSAKey(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		t.Fatalf("pem.Encode: %v", err)
	}
	f.Close()
	return path, priv
}

func TestOAuthTokenGenerator_Token(t *testing.T) {
	keyPath, priv := writeTestEdDSAKey(t)

	gen, err := NewOAuthTokenGenerator(keyPath, "test-issuer", "test-audience", 24*time.Hour)
	if err != nil {
		t.Fatalf("NewOAuthTokenGenerator: %v", err)
	}

	phone := "919876543210"
	nonce := "a1b2c3d4e5f6g7h8"
	pubkey := "dGVzdHB1YmtleXRoYXRpczMyYnl0ZXNsb25nISE"

	tokenStr, err := gen.Token(phone, nonce, pubkey)
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	// Parse and verify the token
	pub := priv.Public().(ed25519.PublicKey)
	claims := &OAuthClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return pub, nil
	})
	if err != nil {
		t.Fatalf("ParseWithClaims: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("token is not valid")
	}

	if claims.Nonce != nonce {
		t.Errorf("nonce = %q, want %q", claims.Nonce, nonce)
	}
	if claims.PubKey != pubkey {
		t.Errorf("pubkey = %q, want %q", claims.PubKey, pubkey)
	}

	sub, _ := claims.GetSubject()
	if sub != phone {
		t.Errorf("sub = %q, want %q", sub, phone)
	}

	iss, _ := claims.GetIssuer()
	if iss != "test-issuer" {
		t.Errorf("iss = %q, want %q", iss, "test-issuer")
	}

	aud, _ := claims.GetAudience()
	if len(aud) != 1 || aud[0] != "test-audience" {
		t.Errorf("aud = %v, want [test-audience]", aud)
	}

	exp, _ := claims.GetExpirationTime()
	iat, _ := claims.GetIssuedAt()
	if exp.Sub(iat.Time) != 24*time.Hour {
		t.Errorf("TTL = %v, want 24h", exp.Sub(iat.Time))
	}
}

func TestOAuthTokenGenerator_TokenLength(t *testing.T) {
	keyPath, _ := writeTestEdDSAKey(t)

	gen, err := NewOAuthTokenGenerator(keyPath, "whatsadk-gateway", "adk-cloud-proxy", 24*time.Hour)
	if err != nil {
		t.Fatalf("NewOAuthTokenGenerator: %v", err)
	}

	tokenStr, err := gen.Token("919876543210", "a1b2c3d4e5f6g7h8", "dGVzdHB1YmtleXRoYXRpczMyYnl0ZXNsb25nISE")
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	if len(tokenStr) > 500 {
		t.Errorf("token length = %d, want <= 500 (compact enough for WhatsApp)", len(tokenStr))
	}
}
