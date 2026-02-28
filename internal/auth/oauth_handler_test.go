package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newTestOAuthHandler(t *testing.T) *OAuthHandler {
	t.Helper()
	keyPath, _ := writeTestEdDSAKey(t)
	gen, err := NewOAuthTokenGenerator(keyPath, "test-issuer", "test-aud", 24*time.Hour)
	if err != nil {
		t.Fatalf("NewOAuthTokenGenerator: %v", err)
	}
	return NewOAuthHandler(gen, "https://chat.example.com", 5)
}

func validPubKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(pub)
}

func TestIsAuthCommand(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"AUTH abc123 nonce", true},
		{"auth abc123 nonce", true},
		{"Auth abc123 nonce", true},
		{"  AUTH abc123 nonce", true},
		{"AUTHENTICATE", false},
		{"hello", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := IsAuthCommand(tt.text); got != tt.want {
				t.Errorf("IsAuthCommand(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestOAuthHandler_Handle_Valid(t *testing.T) {
	h := newTestOAuthHandler(t)
	pubkey := validPubKey(t)
	nonce := "abcdefghijklmnop"

	reply, err := h.Handle("919876543210", "AUTH "+pubkey+" "+nonce)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !strings.HasPrefix(reply, "Click here to complete login:") {
		t.Errorf("unexpected reply prefix: %s", reply)
	}
	if !strings.Contains(reply, "https://chat.example.com/auth#token=") {
		t.Errorf("reply missing deep link: %s", reply)
	}
	if !strings.Contains(reply, "&nonce="+nonce) {
		t.Errorf("reply missing nonce: %s", reply)
	}
}

func TestOAuthHandler_Handle_InvalidPubKey(t *testing.T) {
	h := newTestOAuthHandler(t)
	// 16-byte key (too short, but base64url will be 22 chars — won't match regex requiring 43)
	shortKey := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	nonce := "abcdefghijklmnop"

	reply, err := h.Handle("919876543210", "AUTH "+shortKey+" "+nonce)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !strings.Contains(reply, "Invalid AUTH command") {
		t.Errorf("expected invalid format error, got: %s", reply)
	}
}

func TestOAuthHandler_Handle_MalformedCommand(t *testing.T) {
	h := newTestOAuthHandler(t)

	tests := []string{
		"AUTH",
		"AUTH onlyonearg",
		"AUTH key",
		"HELLO world foo",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			reply, err := h.Handle("919876543210", text)
			if err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if !strings.Contains(reply, "Invalid AUTH command") {
				t.Errorf("expected error message, got: %s", reply)
			}
		})
	}
}

func TestOAuthHandler_Handle_RateLimit(t *testing.T) {
	h := newTestOAuthHandler(t)
	pubkey := validPubKey(t)
	nonce := "abcdefghijklmnop"
	phone := "919876543210"

	for i := 0; i < 5; i++ {
		reply, err := h.Handle(phone, "AUTH "+pubkey+" "+nonce)
		if err != nil {
			t.Fatalf("Handle #%d: %v", i+1, err)
		}
		if strings.Contains(reply, "Too many") {
			t.Fatalf("rate limited too early at request #%d", i+1)
		}
	}

	// 6th request should be rate-limited
	reply, err := h.Handle(phone, "AUTH "+pubkey+" "+nonce)
	if err != nil {
		t.Fatalf("Handle #6: %v", err)
	}
	if !strings.Contains(reply, "Too many") {
		t.Errorf("expected rate limit message, got: %s", reply)
	}
}

func TestOAuthHandler_Handle_Integration(t *testing.T) {
	keyPath, priv := writeTestEdDSAKey(t)

	gen, err := NewOAuthTokenGenerator(keyPath, "whatsadk-gateway", "adk-cloud-proxy", 24*time.Hour)
	if err != nil {
		t.Fatalf("NewOAuthTokenGenerator: %v", err)
	}

	h := NewOAuthHandler(gen, "https://chat.myadk.app", 5)

	// Generate a user key pair
	userPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubkey := base64.RawURLEncoding.EncodeToString(userPub)
	nonce := "test_nonce_1234567"
	phone := "919876543210"

	reply, err := h.Handle(phone, "AUTH "+pubkey+" "+nonce)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Extract the JWT from the reply
	idx := strings.Index(reply, "#token=")
	if idx == -1 {
		t.Fatalf("no token in reply: %s", reply)
	}
	fragment := reply[idx+7:]
	tokenStr := strings.SplitN(fragment, "&", 2)[0]

	// Verify JWT with gateway public key
	pub := priv.Public().(ed25519.PublicKey)
	claims := &OAuthClaims{}
	parsed, parseErr := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return pub, nil
	})
	if parseErr != nil {
		t.Fatalf("ParseWithClaims: %v", parseErr)
	}
	if !parsed.Valid {
		t.Fatal("JWT is not valid")
	}

	sub, _ := claims.GetSubject()
	if sub != phone {
		t.Errorf("sub = %q, want %q", sub, phone)
	}
	if claims.Nonce != nonce {
		t.Errorf("nonce = %q, want %q", claims.Nonce, nonce)
	}
	if claims.PubKey != pubkey {
		t.Errorf("pubkey = %q, want %q", claims.PubKey, pubkey)
	}
}
