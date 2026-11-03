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

	return hmac.Equal(actualSigBytes, expectedSigBytes)
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
