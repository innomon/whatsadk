package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
)

type BlacklistChecker interface {
	IsBlacklisted(ctx context.Context, phone string) (bool, error)
}

type Handler struct {
	keys          *auth.KeyRegistry
	jwtGen        *auth.JWTGenerator
	blacklist     BlacklistChecker
	devOpsNumbers map[string]struct{}
	httpClient    *http.Client
	messages      config.VerificationMessages
	apps          map[string]config.AppVerifyConfig
	rateLimiter   *RateLimiter
	logger        *slog.Logger
}

func NewHandler(
	keys *auth.KeyRegistry,
	jwtGen *auth.JWTGenerator,
	blacklist BlacklistChecker,
	cfg config.VerificationConfig,
	httpClient *http.Client,
	logger *slog.Logger,
) *Handler {
	devOps := make(map[string]struct{}, len(cfg.DevOpsNumbers))
	for _, n := range cfg.DevOpsNumbers {
		devOps[normalizePhone(n)] = struct{}{}
	}
	return &Handler{
		keys:          keys,
		jwtGen:        jwtGen,
		blacklist:     blacklist,
		devOpsNumbers: devOps,
		httpClient:    httpClient,
		messages:      cfg.Messages,
		apps:          cfg.Apps,
		rateLimiter:   NewRateLimiter(cfg.RateLimit.GetMaxAttempts(), cfg.RateLimit.GetWindow()),
		logger:        logger,
	}
}

func (h *Handler) Handle(ctx context.Context, senderPhone, messageBody string) string {
	senderNormalized := normalizePhone(senderPhone)

	if IsHMACVerificationMessage(messageBody) {
		return h.handleHMAC(ctx, senderNormalized, messageBody)
	}

	return h.handleLegacyRSA(ctx, senderNormalized, messageBody)
}

func (h *Handler) handleHMAC(ctx context.Context, senderNormalized, messageBody string) string {
	// 1. Rate limiter check (sliding window per sender)
	if h.rateLimiter != nil && !h.rateLimiter.Allow(senderNormalized) {
		h.logger.Warn("verification rate limit exceeded", "phone", senderNormalized)
		return ""
	}

	// 2. Blacklist check
	if h.blacklist != nil {
		blocked, err := h.blacklist.IsBlacklisted(ctx, senderNormalized)
		if err != nil {
			h.logger.Error("blacklist check failed", "error", err, "phone", senderNormalized)
			return h.messages.Error
		}
		if blocked {
			h.logger.Warn("blacklisted number attempted verification", "phone", senderNormalized)
			return h.messages.Blacklisted
		}
	}

	// 3. Parse HMAC verification token
	token, err := ParseHMACVerificationToken(messageBody)
	if err != nil {
		h.logger.Warn("invalid hmac verification token format", "error", err, "phone", senderNormalized)
		return ""
	}

	// 4. Extract app_id and lookup in configured apps
	appCfg, ok := h.apps[token.AppID]
	if !ok {
		h.logger.Warn("unknown verification app_id", "app_id", token.AppID, "phone", senderNormalized)
		return ""
	}

	// 5. Validate timestamp freshness (|now - timestamp| <= App TTL)
	now := time.Now()
	if !CheckTimestampFreshness(token.Timestamp, appCfg.GetTTL(), now) {
		h.logger.Warn("verification token expired",
			"app_id", token.AppID,
			"timestamp", token.Timestamp,
			"phone", senderNormalized,
		)
		return h.messages.Expired
	}

	// 6. Verify HMAC-SHA256 signature (constant-time)
	secret := appCfg.GetSecret()
	if !VerifyHMAC(token, secret) {
		h.logger.Warn("verification signature mismatch / tamper detected",
			"app_id", token.AppID,
			"phone", senderNormalized,
		)
		return ""
	}

	// 7. Dispatch HTTP POST to App Webhook with HMAC signature
	if err := h.postHMACWebhook(ctx, appCfg.WebhookURL, appCfg.GetSecret(), senderNormalized, token.RawBody); err != nil {
		h.logger.Error("hmac webhook callback failed",
			"url", appCfg.WebhookURL,
			"app_id", token.AppID,
			"error", err,
		)
		return h.messages.Error
	}

	h.logger.Info("hmac verification successful",
		"phone", senderNormalized,
		"app_id", token.AppID,
		"token", token.Token,
	)

	if appCfg.SuccessMessage != "" {
		return appCfg.SuccessMessage
	}
	return h.messages.Success
}

type hmacWebhookPayload struct {
	From string `json:"from"`
	Body string `json:"body"`
}

func (h *Handler) postHMACWebhook(ctx context.Context, webhookURL, secret, from, body string) error {
	payload := hmacWebhookPayload{
		From: from,
		Body: body,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	nowTs := time.Now().Unix()
	req.Header.Set("X-Signature-Timestamp", strconv.FormatInt(nowTs, 10))
	if secret != "" {
		sig := SignWebhookPayload(secret, nowTs, payloadBytes)
		req.Header.Set("X-Signature-256", sig)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (h *Handler) handleLegacyRSA(ctx context.Context, senderNormalized, messageBody string) string {
	claims := auth.IsVerificationToken(messageBody)
	if claims == nil {
		return ""
	}

	if h.rateLimiter != nil && !h.rateLimiter.Allow(senderNormalized) {
		h.logger.Warn("verification rate limit exceeded", "phone", senderNormalized)
		return ""
	}

	if h.blacklist != nil {
		blocked, err := h.blacklist.IsBlacklisted(ctx, senderNormalized)
		if err != nil {
			h.logger.Error("blacklist check failed", "error", err, "phone", senderNormalized)
			return h.messages.Error
		}
		if blocked {
			h.logger.Warn("blacklisted number attempted verification", "phone", senderNormalized)
			return h.messages.Blacklisted
		}
	}

	if h.keys == nil {
		h.logger.Warn("key registry not initialized for legacy rsa token")
		return h.messages.Error
	}

	appKey, err := h.keys.GetAppPublicKey(claims.AppName)
	if err != nil {
		h.logger.Warn("unknown app", "app_name", claims.AppName)
		return h.messages.Error
	}

	verified, err := auth.VerifyVerificationToken(messageBody, appKey)
	if err != nil {
		h.logger.Warn("verification token invalid", "error", err, "app", claims.AppName)
		return h.messages.Expired
	}

	mobileNormalized := normalizePhone(verified.Mobile)
	if senderNormalized != mobileNormalized {
		if _, isDevOps := h.devOpsNumbers[senderNormalized]; !isDevOps {
			h.logger.Warn("phone mismatch",
				"sender", senderNormalized,
				"claim_mobile", mobileNormalized,
			)
			return h.messages.PhoneMismatch
		}
		h.logger.Info("devops override: phone mismatch allowed",
			"sender", senderNormalized,
			"claim_mobile", mobileNormalized,
		)
	}

	if h.jwtGen == nil {
		h.logger.Error("jwt generator not configured")
		return h.messages.Error
	}

	callbackJWT, err := h.jwtGen.TokenWithAudience(senderNormalized, verified.AppName)
	if err != nil {
		h.logger.Error("failed to sign callback JWT", "error", err)
		return h.messages.Error
	}

	if err := h.postCallback(ctx, verified.CallbackURL, callbackJWT); err != nil {
		h.logger.Error("callback failed",
			"url", verified.CallbackURL,
			"error", err,
		)
		return h.messages.Error
	}

	h.logger.Info("verification successful",
		"phone", senderNormalized,
		"app", verified.AppName,
		"challenge_id", verified.ChallengeID,
	)
	return h.messages.Success
}

func (h *Handler) postCallback(ctx context.Context, callbackURL, jwtToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute callback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("callback returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func normalizePhone(phone string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
}
