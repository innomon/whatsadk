package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signVerificationToken(t *testing.T, key *rsa.PrivateKey, claims VerificationClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestIsVerificationToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	now := time.Now()
	claims := VerificationClaims{
		Mobile:      "910987654321",
		AppName:     "test-app",
		CallbackURL: "https://example.com/callback",
		ChallengeID: "abc-123",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}

	tokenStr := signVerificationToken(t, key, claims)
	result := IsVerificationToken(tokenStr)
	if result == nil {
		t.Fatal("expected non-nil result for valid verification token")
	}
	if result.Mobile != "910987654321" {
		t.Errorf("expected mobile=910987654321, got %s", result.Mobile)
	}
	if result.AppName != "test-app" {
		t.Errorf("expected app_name=test-app, got %s", result.AppName)
	}
	if result.CallbackURL != "https://example.com/callback" {
		t.Errorf("expected callback_url, got %s", result.CallbackURL)
	}
}

func TestIsVerificationToken_NormalMessage(t *testing.T) {
	result := IsVerificationToken("Hello world")
	if result != nil {
		t.Fatal("expected nil for normal message")
	}
}

func TestIsVerificationToken_ADKToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	now := time.Now()
	claims := Claims{
		UserID:  "user123",
		Channel: "whatsapp",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	result := IsVerificationToken(tokenStr)
	if result != nil {
		t.Fatal("expected nil for ADK token without verification claims")
	}
}

func TestVerifyVerificationToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	now := time.Now()
	claims := VerificationClaims{
		Mobile:      "910987654321",
		AppName:     "test-app",
		CallbackURL: "https://example.com/callback",
		ChallengeID: "abc-123",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}

	tokenStr := signVerificationToken(t, key, claims)

	verified, err := VerifyVerificationToken(tokenStr, &key.PublicKey)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}
	if verified.Mobile != "910987654321" {
		t.Errorf("expected mobile=910987654321, got %s", verified.Mobile)
	}
	if verified.AppName != "test-app" {
		t.Errorf("expected app_name=test-app, got %s", verified.AppName)
	}
	if verified.ChallengeID != "abc-123" {
		t.Errorf("expected challenge_id=abc-123, got %s", verified.ChallengeID)
	}
}

func TestVerifyVerificationToken_BadSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate wrong key: %v", err)
	}

	now := time.Now()
	claims := VerificationClaims{
		Mobile:      "910987654321",
		AppName:     "test-app",
		CallbackURL: "https://example.com/callback",
		ChallengeID: "abc-123",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}

	tokenStr := signVerificationToken(t, key, claims)

	_, err = VerifyVerificationToken(tokenStr, &wrongKey.PublicKey)
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
}

func TestVerifyVerificationToken_Expired(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	past := time.Now().Add(-10 * time.Minute)
	claims := VerificationClaims{
		Mobile:      "910987654321",
		AppName:     "test-app",
		CallbackURL: "https://example.com/callback",
		ChallengeID: "abc-123",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(past),
			ExpiresAt: jwt.NewNumericDate(past.Add(5 * time.Minute)),
		},
	}

	tokenStr := signVerificationToken(t, key, claims)

	_, err = VerifyVerificationToken(tokenStr, &key.PublicKey)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
