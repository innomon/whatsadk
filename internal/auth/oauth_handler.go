package auth

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var authCommandRe = regexp.MustCompile(`^AUTH\s+([A-Za-z0-9_-]{43}=?)\s+([A-Za-z0-9_-]{16,})$`)

// OAuthHandler processes AUTH commands received via WhatsApp messages.
type OAuthHandler struct {
	tokenGen  *OAuthTokenGenerator
	spaURL    string
	rateLimit int

	mu      sync.Mutex
	history map[string][]time.Time // phone → timestamps of AUTH requests
}

// NewOAuthHandler creates a handler that generates OAuth deep links.
func NewOAuthHandler(tokenGen *OAuthTokenGenerator, spaURL string, rateLimit int) *OAuthHandler {
	return &OAuthHandler{
		tokenGen:  tokenGen,
		spaURL:    strings.TrimRight(spaURL, "/"),
		rateLimit: rateLimit,
		history:   make(map[string][]time.Time),
	}
}

// IsAuthCommand returns true if the text starts with "AUTH " (case-insensitive).
func IsAuthCommand(text string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(text)), "AUTH ")
}

// Handle parses an AUTH command and returns a WhatsApp reply with a deep link.
func (h *OAuthHandler) Handle(senderPhone, messageBody string) (string, error) {
	messageBody = strings.TrimSpace(messageBody)
	matches := authCommandRe.FindStringSubmatch(messageBody)
	if matches == nil {
		return "❌ Invalid AUTH command format.\nExpected: AUTH <public_key> <nonce>", nil
	}

	userPubKey := matches[1]
	nonce := matches[2]

	// Validate the public key is a valid 32-byte base64url-encoded key
	decoded, err := base64.RawURLEncoding.DecodeString(userPubKey)
	if err != nil {
		return "❌ Invalid public key: not valid base64url encoding.", nil
	}
	if len(decoded) != 32 {
		return fmt.Sprintf("❌ Invalid public key: expected 32 bytes, got %d.", len(decoded)), nil
	}

	// Check rate limit
	if !h.checkRateLimit(senderPhone) {
		return "⏳ Too many AUTH requests. Please try again later.", nil
	}

	// Generate JWT
	tokenStr, err := h.tokenGen.Token(senderPhone, nonce, userPubKey)
	if err != nil {
		return "", fmt.Errorf("failed to generate OAuth token: %w", err)
	}

	deepLink := fmt.Sprintf("%s/auth#token=%s&nonce=%s", h.spaURL, tokenStr, nonce)
	reply := fmt.Sprintf("Click here to complete login:\n%s", deepLink)
	return reply, nil
}

func (h *OAuthHandler) checkRateLimit(phone string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	// Prune old entries
	timestamps := h.history[phone]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= h.rateLimit {
		h.history[phone] = valid
		return false
	}

	h.history[phone] = append(valid, now)
	return true
}
