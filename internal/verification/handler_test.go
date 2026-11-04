package verification

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

func TestHMAC_TokenParsingAndFreshness(t *testing.T) {
	appID := "kb"
	token := "0123456789abcdef0123456789abcdef"
	now := time.Now()
	ts := now.Unix()
	secret := "secret-key-42"
	sig := ComputeHMAC(appID, token, ts, secret)

	validMsg := "VERIFY " + appID + ":" + token + ":" + strconv.FormatInt(ts, 10) + ":" + sig

	// 1. IsHMACVerificationMessage
	if !IsHMACVerificationMessage(validMsg) {
		t.Errorf("expected IsHMACVerificationMessage to be true for uppercase")
	}
	if !IsHMACVerificationMessage("verify " + appID + ":" + token) {
		t.Errorf("expected IsHMACVerificationMessage to be true for lowercase")
	}
	if IsHMACVerificationMessage("HELLO WORLD") {
		t.Errorf("expected IsHMACVerificationMessage to be false for normal text")
	}

	// 2. ParseHMACVerificationToken
	parsed, err := ParseHMACVerificationToken(validMsg)
	if err != nil {
		t.Fatalf("failed to parse valid token: %v", err)
	}
	if parsed.AppID != appID || parsed.Token != token || parsed.Timestamp != ts || parsed.Signature != sig {
		t.Fatalf("parsed token mismatch: %+v", parsed)
	}

	// 3. VerifyHMAC
	if !VerifyHMAC(parsed, secret) {
		t.Errorf("expected VerifyHMAC to return true for correct secret")
	}
	if VerifyHMAC(parsed, "wrong-secret") {
		t.Errorf("expected VerifyHMAC to return false for wrong secret")
	}

	// 3b. VerifyHMAC with truncated 32-char (16-byte) HMAC
	truncated := *parsed
	truncated.Signature = sig[:32]
	if !VerifyHMAC(&truncated, secret) {
		t.Errorf("expected VerifyHMAC to accept truncated 32-character hex HMAC signature")
	}

	// 4. Tampered signature
	tampered := *parsed
	tampered.Signature = "0000000000000000000000000000000000000000000000000000000000000000"
	if VerifyHMAC(&tampered, secret) {
		t.Errorf("expected VerifyHMAC to reject tampered signature")
	}

	// 5. Invalid parsing cases
	invalidCases := []string{
		"NOT_VERIFY kb:token:123:abc",
		"VERIFY kb:token:123", // missing signature
		"VERIFY kb:token:123:abc:extra", // too many fields
		"VERIFY kb::123:abc", // empty token
		"VERIFY kb:token:not-a-number:abc", // invalid timestamp
		"VERIFY kb:token:123:not-hex-sig-xyz!", // invalid hex
	}
	for _, ic := range invalidCases {
		if _, err := ParseHMACVerificationToken(ic); err == nil {
			t.Errorf("expected error for invalid token format: %q", ic)
		}
	}

	// 6. Timestamp freshness
	if !CheckTimestampFreshness(ts, 5*time.Minute, now) {
		t.Errorf("expected fresh timestamp to be valid")
	}
	expiredTs := now.Add(-10 * time.Minute).Unix()
	if CheckTimestampFreshness(expiredTs, 5*time.Minute, now) {
		t.Errorf("expected 10-minute old timestamp to be rejected with 5m TTL")
	}

	// 7. Outbound Webhook Signing & Verification
	payloadBody := []byte(`{"from":"919876543210","body":"VERIFY test"}`)
	outboundSig := SignWebhookPayload(secret, now.Unix(), payloadBody)
	if !strings.HasPrefix(outboundSig, "sha256=") {
		t.Errorf("expected signature to have sha256= prefix, got %s", outboundSig)
	}

	tsStr := strconv.FormatInt(now.Unix(), 10)
	if err := VerifyWebhookSignature(secret, payloadBody, tsStr, outboundSig, 5*time.Minute, now); err != nil {
		t.Errorf("expected signature verification to succeed, got %v", err)
	}

	// Tampered body
	tamperedBody := []byte(`{"from":"919876543210","body":"VERIFY tampered"}`)
	if err := VerifyWebhookSignature(secret, tamperedBody, tsStr, outboundSig, 5*time.Minute, now); err == nil {
		t.Errorf("expected signature verification to fail for tampered body")
	}

	// Expired webhook timestamp
	oldTsStr := strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10)
	if err := VerifyWebhookSignature(secret, payloadBody, oldTsStr, outboundSig, 5*time.Minute, now); err == nil {
		t.Errorf("expected verification to fail for expired timestamp")
	}

	// Wrong secret
	if err := VerifyWebhookSignature("wrong-secret", payloadBody, tsStr, outboundSig, 5*time.Minute, now); err == nil {
		t.Errorf("expected verification to fail for wrong secret")
	}
}

func TestHandler_HMAC_FullFlow(t *testing.T) {
	type webhookReqBody struct {
		From         string
		Body         string
		RawBody      []byte
		SigHeader    string
		TsHeader     string
	}

	webhookReceived := make(chan webhookReqBody, 1)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed struct {
			From string `json:"from"`
			Body string `json:"body"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		webhookReceived <- webhookReqBody{
			From:      parsed.From,
			Body:      parsed.Body,
			RawBody:   raw,
			SigHeader: r.Header.Get("X-Signature-256"),
			TsHeader:  r.Header.Get("X-Signature-Timestamp"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	appSecret := "kheti-badi-shared-secret"
	cfg := config.VerificationConfig{
		Enabled: true,
		RateLimit: config.RateLimitConfig{
			MaxAttempts: 3,
			Window:      "1m",
		},
		Apps: map[string]config.AppVerifyConfig{
			"kb": {
				Name:           "kheti-badi",
				AuthType:       "hmac",
				Secret:         appSecret,
				WebhookURL:     webhookServer.URL,
				TTL:            "5m",
				SuccessMessage: "✅ Verified successfully! Please return to Kheti-Badi.",
			},
		},
		Messages: config.VerificationMessages{
			Success:       "Standard success",
			Expired:       "Expired token",
			Blacklisted:   "Blacklisted",
			Error:         "Server error",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	bl := &mockBlacklist{blocked: make(map[string]bool)}
	handler := NewHandler(nil, nil, bl, cfg, webhookServer.Client(), logger)

	tokenHex := "abcdef0123456789abcdef0123456789"
	ts := time.Now().Unix()
	sig := ComputeHMAC("kb", tokenHex, ts, appSecret)
	validMessage := fmt.Sprintf("VERIFY kb:%s:%d:%s", tokenHex, ts, sig)
	sender := "+91 (987) 654-3210"

	// 1. Success Flow
	resp := handler.Handle(context.Background(), sender, validMessage)
	if resp != "✅ Verified successfully! Please return to Kheti-Badi." {
		t.Fatalf("unexpected response: %q", resp)
	}

	select {
	case received := <-webhookReceived:
		if received.From != "919876543210" {
			t.Errorf("expected from=919876543210, got %q", received.From)
		}
		if received.Body != validMessage {
			t.Errorf("expected body=%q, got %q", validMessage, received.Body)
		}
		if received.SigHeader == "" {
			t.Errorf("expected X-Signature-256 header to be present")
		}
		if received.TsHeader == "" {
			t.Errorf("expected X-Signature-Timestamp header to be present")
		}
		// Verify signature matches using VerifyWebhookSignature helper
		if err := VerifyWebhookSignature(appSecret, received.RawBody, received.TsHeader, received.SigHeader, 1*time.Minute, time.Now()); err != nil {
			t.Errorf("failed to verify received webhook signature: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook callback")
	}

	// 2. Unknown App ID -> dropped (empty response, no webhook)
	unknownAppSig := ComputeHMAC("unknown", tokenHex, ts, appSecret)
	unknownMsg := fmt.Sprintf("VERIFY unknown:%s:%d:%s", tokenHex, ts, unknownAppSig)
	resp = handler.Handle(context.Background(), sender, unknownMsg)
	if resp != "" {
		t.Errorf("expected empty string (dropped) for unknown app, got %q", resp)
	}

	// 3. Expired Timestamp -> returns Expired message, no webhook
	expiredTs := time.Now().Add(-10 * time.Minute).Unix()
	expiredSig := ComputeHMAC("kb", tokenHex, expiredTs, appSecret)
	expiredMsg := fmt.Sprintf("VERIFY kb:%s:%d:%s", tokenHex, expiredTs, expiredSig)
	resp = handler.Handle(context.Background(), sender, expiredMsg)
	if resp != "Expired token" {
		t.Errorf("expected Expired token message, got %q", resp)
	}

	// 4. Rate Limiter Exceeded (max_attempts = 3)
	// We've already performed 3 attempts for 'sender'
	resp = handler.Handle(context.Background(), sender, validMessage)
	if resp != "" {
		t.Errorf("expected 4th attempt to be dropped by rate limiter, got %q", resp)
	}

	// 5. Signature mismatch / tamper -> dropped
	otherSender := "911234567890"
	tamperedMsg := fmt.Sprintf("VERIFY kb:%s:%d:%s", tokenHex, ts, "deadbeef1234")
	resp = handler.Handle(context.Background(), otherSender, tamperedMsg)
	if resp != "" {
		t.Errorf("expected empty string (dropped) for tampered signature, got %q", resp)
	}

	// 6. Blacklisted Sender
	bl.blocked["911234567890"] = true
	resp = handler.Handle(context.Background(), otherSender, validMessage)
	if resp != "Blacklisted" {
		t.Errorf("expected Blacklisted response, got %q", resp)
	}
}
