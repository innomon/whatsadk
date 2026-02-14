package verification

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
)

type Handler struct {
	keys       *auth.KeyRegistry
	jwtGen     *auth.JWTGenerator
	httpClient *http.Client
	messages   config.VerificationMessages
	logger     *slog.Logger
}

func NewHandler(
	keys *auth.KeyRegistry,
	jwtGen *auth.JWTGenerator,
	cfg config.VerificationConfig,
	httpClient *http.Client,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		keys:       keys,
		jwtGen:     jwtGen,
		httpClient: httpClient,
		messages:   cfg.Messages,
		logger:     logger,
	}
}

func (h *Handler) Handle(ctx context.Context, senderPhone, messageBody string) string {
	claims := auth.IsVerificationToken(messageBody)
	if claims == nil {
		return ""
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

	senderNormalized := normalizePhone(senderPhone)
	mobileNormalized := normalizePhone(verified.Mobile)
	if senderNormalized != mobileNormalized {
		h.logger.Warn("phone mismatch",
			"sender", senderNormalized,
			"claim_mobile", mobileNormalized,
		)
		return h.messages.PhoneMismatch
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
