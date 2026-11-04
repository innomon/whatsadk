package verification

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HMACToken represents the parsed components of a namespaced HMAC verification message.
type HMACToken struct {
	AppID     string
	Token     string
	Timestamp int64
	Signature string
	RawBody   string
}

// IsHMACVerificationMessage checks if the given message begins with the case-insensitive "VERIFY " trigger.
func IsHMACVerificationMessage(message string) bool {
	trimmed := strings.TrimSpace(message)
	if len(trimmed) < 7 {
		return false
	}
	return strings.EqualFold(trimmed[:7], "VERIFY ")
}

// ParseHMACVerificationToken parses a string formatted as "VERIFY <app_id>:<token>:<timestamp>:<signature>".
func ParseHMACVerificationToken(raw string) (*HMACToken, error) {
	trimmed := strings.TrimSpace(raw)
	if !IsHMACVerificationMessage(trimmed) {
		return nil, errors.New("message does not start with VERIFY command prefix")
	}

	payload := strings.TrimSpace(trimmed[7:])
	parts := strings.Split(payload, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid token format: expected 4 colon-separated fields, got %d", len(parts))
	}

	appID := strings.TrimSpace(parts[0])
	token := strings.TrimSpace(parts[1])
	tsStr := strings.TrimSpace(parts[2])
	sig := strings.TrimSpace(parts[3])

	if appID == "" || token == "" || tsStr == "" || sig == "" {
		return nil, errors.New("empty component in verification token")
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp %q: %w", tsStr, err)
	}

	// Validate that the signature is valid hex
	if _, err := hex.DecodeString(sig); err != nil {
		return nil, fmt.Errorf("invalid signature hex encoding: %w", err)
	}

	return &HMACToken{
		AppID:     appID,
		Token:     token,
		Timestamp: ts,
		Signature: sig,
		RawBody:   trimmed,
	}, nil
}

// ComputeHMAC generates the hex-encoded HMAC-SHA256 signature for the given appID, token, and timestamp using secret.
func ComputeHMAC(appID, token string, timestamp int64, secret string) string {
	message := fmt.Sprintf("%s:%s:%d", appID, token, timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC verifies that token's signature matches HMAC-SHA256(appID:token:timestamp) with secret using constant-time comparison.
// It supports both compact truncated HMAC (16 bytes / 32 hex chars) and full HMAC (32 bytes / 64 hex chars).
func VerifyHMAC(token *HMACToken, secret string) bool {
	if token == nil || secret == "" {
		return false
	}

	expectedSigHex := ComputeHMAC(token.AppID, token.Token, token.Timestamp, secret)

	expectedSigBytes, err1 := hex.DecodeString(expectedSigHex)
	actualSigBytes, err2 := hex.DecodeString(token.Signature)
	if err1 != nil || err2 != nil {
		return false
	}

	if len(actualSigBytes) == 16 && len(expectedSigBytes) >= 16 {
		return hmac.Equal(actualSigBytes, expectedSigBytes[:16])
	}
	if len(actualSigBytes) == 32 && len(expectedSigBytes) == 32 {
		return hmac.Equal(actualSigBytes, expectedSigBytes)
	}

	return false
}

// CheckTimestampFreshness verifies that |now - tokenTimestamp| <= ttl.
func CheckTimestampFreshness(tokenTimestamp int64, ttl time.Duration, now time.Time) bool {
	tokenTime := time.Unix(tokenTimestamp, 0)
	diff := now.Sub(tokenTime)
	if diff < 0 {
		diff = -diff
	}
	return diff <= ttl
}

// SignWebhookPayload generates an HMAC-SHA256 signature for outgoing webhook calls over "<timestamp>.<bodyBytes>".
// It returns formatted as "sha256=<hex>".
func SignWebhookPayload(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies that the given signatureHeader matches HMAC-SHA256(secret, timestamp + "." + body)
// and that the timestamp is within tolerance of now.
func VerifyWebhookSignature(secret string, body []byte, timestampHeader, signatureHeader string, tolerance time.Duration, now time.Time) error {
	if secret == "" {
		return errors.New("empty webhook secret")
	}
	if timestampHeader == "" {
		return errors.New("missing timestamp header")
	}
	if signatureHeader == "" {
		return errors.New("missing signature header")
	}

	ts, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp header: %w", err)
	}

	if tolerance > 0 && !CheckTimestampFreshness(ts, tolerance, now) {
		return errors.New("webhook timestamp outside tolerance window")
	}

	expectedSig := SignWebhookPayload(secret, ts, body)

	// Strip "sha256=" prefix if present in signatureHeader for comparison
	actualSig := signatureHeader
	if !strings.HasPrefix(actualSig, "sha256=") {
		actualSig = "sha256=" + actualSig
	}

	if !hmac.Equal([]byte(expectedSig), []byte(actualSig)) {
		return errors.New("invalid webhook signature")
	}

	return nil
}
