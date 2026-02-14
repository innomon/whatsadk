package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKey(t *testing.T) (string, *rsa.PublicKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	path := filepath.Join(t.TempDir(), "test_key.pem")
	if err := os.WriteFile(path, keyPEM, 0600); err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}

	return path, &key.PublicKey
}

func TestJWTGenerator_Token(t *testing.T) {
	keyPath, pubKey := generateTestKey(t)

	gen, err := NewJWTGenerator(keyPath, "test-issuer", "test-audience", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	tokenStr, err := gen.Token("user123")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return pubKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		t.Fatal("token claims invalid")
	}

	t.Run("user_id claim", func(t *testing.T) {
		if claims.UserID != "user123" {
			t.Errorf("expected user_id=user123, got %s", claims.UserID)
		}
	})

	t.Run("channel claim", func(t *testing.T) {
		if claims.Channel != "whatsapp" {
			t.Errorf("expected channel=whatsapp, got %s", claims.Channel)
		}
	})

	t.Run("issuer", func(t *testing.T) {
		if claims.Issuer != "test-issuer" {
			t.Errorf("expected issuer=test-issuer, got %s", claims.Issuer)
		}
	})

	t.Run("audience", func(t *testing.T) {
		if len(claims.Audience) != 1 || claims.Audience[0] != "test-audience" {
			t.Errorf("expected audience=[test-audience], got %v", claims.Audience)
		}
	})

	t.Run("expiry", func(t *testing.T) {
		if claims.ExpiresAt == nil {
			t.Fatal("expected expiry to be set")
		}
		if time.Until(claims.ExpiresAt.Time) < 4*time.Minute {
			t.Errorf("expected expiry ~5m from now, got %v", claims.ExpiresAt.Time)
		}
	})
}

func TestJWTGenerator_InvalidKeyPath(t *testing.T) {
	_, err := NewJWTGenerator("/nonexistent/key.pem", "", "", time.Minute)
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestJWTGenerator_InvalidKeyData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad_key.pem")
	if err := os.WriteFile(path, []byte("not a key"), 0600); err != nil {
		t.Fatalf("failed to write bad key: %v", err)
	}

	_, err := NewJWTGenerator(path, "", "", time.Minute)
	if err == nil {
		t.Fatal("expected error for invalid key data")
	}
}

func TestTokenWithAudience(t *testing.T) {
	keyPath, pubKey := generateTestKey(t)

	gen, err := NewJWTGenerator(keyPath, "test-issuer", "default-audience", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	tokenStr, err := gen.TokenWithAudience("user123", "custom-app")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return pubKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		t.Fatal("token claims invalid")
	}

	if claims.UserID != "user123" {
		t.Errorf("expected user_id=user123, got %s", claims.UserID)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "custom-app" {
		t.Errorf("expected audience=[custom-app], got %v", claims.Audience)
	}
}
