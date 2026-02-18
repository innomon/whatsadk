package verification

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
)

type mockBlacklist struct {
	blocked map[string]bool
}

func (m *mockBlacklist) IsBlacklisted(_ context.Context, phone string) (bool, error) {
	return m.blocked[phone], nil
}

type testSetup struct {
	appKey     *rsa.PrivateKey
	gwKeyPath  string
	gwPubKey   *rsa.PublicKey
	handler    *Handler
	blacklist  *mockBlacklist
	server     *httptest.Server
	serverURL  string
	callbackCh chan *http.Request
}

func setupTest(t *testing.T) *testSetup {
	t.Helper()

	// Generate app keypair (for signing verification tokens)
	appKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate app key: %v", err)
	}

	// Write app public key to file
	appPubBytes, err := x509.MarshalPKIXPublicKey(&appKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal app public key: %v", err)
	}
	appPubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: appPubBytes,
	})
	appPubPath := filepath.Join(t.TempDir(), "app_public.pem")
	if err := os.WriteFile(appPubPath, appPubPEM, 0644); err != nil {
		t.Fatalf("failed to write app public key: %v", err)
	}

	// Generate gateway keypair (for signing callback JWTs)
	gwKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate gw key: %v", err)
	}
	gwKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(gwKey),
	})
	gwKeyPath := filepath.Join(t.TempDir(), "gw_private.pem")
	if err := os.WriteFile(gwKeyPath, gwKeyPEM, 0600); err != nil {
		t.Fatalf("failed to write gw private key: %v", err)
	}

	// Create key registry with app public key
	apps := map[string]config.AppVerifyConfig{
		"test-app": {PublicKeyPath: appPubPath},
	}
	keyRegistry, err := auth.NewKeyRegistry(apps)
	if err != nil {
		t.Fatalf("failed to create key registry: %v", err)
	}

	// Create JWT generator (gateway's)
	jwtGen, err := auth.NewJWTGenerator(gwKeyPath, "whatsadk-gateway", "", 2*time.Minute)
	if err != nil {
		t.Fatalf("failed to create jwt generator: %v", err)
	}

	// Create mock callback server
	callbackCh := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackCh <- r
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := config.VerificationConfig{
		Messages: config.VerificationMessages{
			Success:       "✅ Verification successful! You can now return to the app.",
			Expired:       "❌ Verification failed. The link may have expired. Please request a new one from the app.",
			PhoneMismatch: "❌ Verification failed. Please make sure you're sending from the same number you registered with.",
			Blacklisted:   "🚫 This number has been blocked from verification.",
			Error:         "⚠️ Something went wrong. Please try again in a moment.",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	bl := &mockBlacklist{blocked: make(map[string]bool)}
	handler := NewHandler(keyRegistry, jwtGen, bl, cfg, server.Client(), logger)

	return &testSetup{
		appKey:     appKey,
		gwKeyPath:  gwKeyPath,
		gwPubKey:   &gwKey.PublicKey,
		handler:    handler,
		blacklist:  bl,
		server:     server,
		serverURL:  server.URL,
		callbackCh: callbackCh,
	}
}

func signTestVerificationToken(t *testing.T, key *rsa.PrivateKey, mobile, appName, callbackURL, challengeID string, expiry time.Time) string {
	t.Helper()
	claims := auth.VerificationClaims{
		Mobile:      mobile,
		AppName:     appName,
		CallbackURL: callbackURL,
		ChallengeID: challengeID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestHandler_SuccessFlow(t *testing.T) {
	ts := setupTest(t)

	tokenStr := signTestVerificationToken(t, ts.appKey,
		"910987654321", "test-app",
		ts.serverURL+"/callback?challenge_id=abc-123", "abc-123",
		time.Now().Add(5*time.Minute),
	)

	result := ts.handler.Handle(context.Background(), "910987654321", tokenStr)

	if !strings.Contains(result, "Verification successful") {
		t.Errorf("expected success message, got: %s", result)
	}

	// Verify callback was received
	select {
	case req := <-ts.callbackCh:
		authHeader := req.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Errorf("expected Bearer auth header, got: %s", authHeader)
		}
		// Verify the callback JWT
		callbackToken := strings.TrimPrefix(authHeader, "Bearer ")
		parsed, err := jwt.ParseWithClaims(callbackToken, &auth.Claims{}, func(t *jwt.Token) (interface{}, error) {
			return ts.gwPubKey, nil
		})
		if err != nil {
			t.Fatalf("failed to parse callback JWT: %v", err)
		}
		claims := parsed.Claims.(*auth.Claims)
		if claims.UserID != "910987654321" {
			t.Errorf("expected user_id=910987654321, got %s", claims.UserID)
		}
		if claims.Channel != "whatsapp" {
			t.Errorf("expected channel=whatsapp, got %s", claims.Channel)
		}
		if len(claims.Audience) != 1 || claims.Audience[0] != "test-app" {
			t.Errorf("expected audience=[test-app], got %v", claims.Audience)
		}
		if claims.Issuer != "whatsadk-gateway" {
			t.Errorf("expected issuer=whatsadk-gateway, got %s", claims.Issuer)
		}
	default:
		t.Fatal("expected callback request but none received")
	}
}

func TestHandler_PhoneMismatch(t *testing.T) {
	ts := setupTest(t)

	tokenStr := signTestVerificationToken(t, ts.appKey,
		"910987654321", "test-app",
		ts.serverURL+"/callback", "abc-123",
		time.Now().Add(5*time.Minute),
	)

	result := ts.handler.Handle(context.Background(), "911111111111", tokenStr)

	if !strings.Contains(result, "same number") {
		t.Errorf("expected phone mismatch message, got: %s", result)
	}
}

func TestHandler_CallbackFails(t *testing.T) {
	ts := setupTest(t)

	// Override server to return 500
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failServer.Close)

	tokenStr := signTestVerificationToken(t, ts.appKey,
		"910987654321", "test-app",
		failServer.URL+"/callback", "abc-123",
		time.Now().Add(5*time.Minute),
	)

	// Use the fail server's client so the handler hits the fail server
	cfg := config.VerificationConfig{
		Messages: ts.handler.messages,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Need to re-create handler with failServer's client
	apps := map[string]config.AppVerifyConfig{
		"test-app": {PublicKeyPath: writeAppPubKey(t, ts.appKey)},
	}
	keyRegistry, _ := auth.NewKeyRegistry(apps)
	jwtGen, _ := auth.NewJWTGenerator(ts.gwKeyPath, "whatsadk-gateway", "", 2*time.Minute)
	handler := NewHandler(keyRegistry, jwtGen, ts.blacklist, cfg, failServer.Client(), logger)

	result := handler.Handle(context.Background(), "910987654321", tokenStr)

	if !strings.Contains(result, "Something went wrong") {
		t.Errorf("expected error message, got: %s", result)
	}
}

func TestHandler_BlacklistedNumber(t *testing.T) {
	ts := setupTest(t)

	ts.blacklist.blocked["910987654321"] = true

	tokenStr := signTestVerificationToken(t, ts.appKey,
		"910987654321", "test-app",
		ts.serverURL+"/callback?challenge_id=abc-123", "abc-123",
		time.Now().Add(5*time.Minute),
	)

	result := ts.handler.Handle(context.Background(), "910987654321", tokenStr)

	if !strings.Contains(result, "blocked") {
		t.Errorf("expected blacklisted message, got: %s", result)
	}

	// Verify no callback was made
	select {
	case <-ts.callbackCh:
		t.Fatal("expected no callback for blacklisted number")
	default:
	}
}

func TestHandler_PhoneMismatch_DevOps(t *testing.T) {
	ts := setupTest(t)

	// Re-create handler with devops number configured
	cfg := config.VerificationConfig{
		DevOpsNumbers: []string{"919999999999"},
		Messages:      ts.handler.messages,
	}
	apps := map[string]config.AppVerifyConfig{
		"test-app": {PublicKeyPath: writeAppPubKey(t, ts.appKey)},
	}
	keyRegistry, _ := auth.NewKeyRegistry(apps)
	jwtGen, _ := auth.NewJWTGenerator(ts.gwKeyPath, "whatsadk-gateway", "", 2*time.Minute)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := NewHandler(keyRegistry, jwtGen, ts.blacklist, cfg, ts.server.Client(), logger)

	// Token claims mobile=910987654321, but sender is the devops number
	tokenStr := signTestVerificationToken(t, ts.appKey,
		"910987654321", "test-app",
		ts.serverURL+"/callback?challenge_id=abc-123", "abc-123",
		time.Now().Add(5*time.Minute),
	)

	result := handler.Handle(context.Background(), "919999999999", tokenStr)

	if !strings.Contains(result, "Verification successful") {
		t.Errorf("expected success message for devops override, got: %s", result)
	}

	// Verify callback was received
	select {
	case req := <-ts.callbackCh:
		authHeader := req.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Errorf("expected Bearer auth header, got: %s", authHeader)
		}
	default:
		t.Fatal("expected callback request for devops override but none received")
	}
}

func TestHandler_ExpiredToken(t *testing.T) {
	ts := setupTest(t)

	tokenStr := signTestVerificationToken(t, ts.appKey,
		"910987654321", "test-app",
		ts.serverURL+"/callback", "abc-123",
		time.Now().Add(-5*time.Minute), // expired
	)

	result := ts.handler.Handle(context.Background(), "910987654321", tokenStr)

	if !strings.Contains(result, "expired") {
		t.Errorf("expected expired message, got: %s", result)
	}
}

func writeAppPubKey(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	path := filepath.Join(t.TempDir(), "app_public.pem")
	if err := os.WriteFile(path, pubPEM, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}
	return path
}
