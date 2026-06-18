package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/logger"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/waba"
	"github.com/innomon/whatsadk/internal/whatsapp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	appLogger, err := logger.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	appLogger.Info("Structured logging initialized")

	if !cfg.WABA.Enabled {
		fmt.Println("WABA is disabled in config. To enable, set WABA_ENABLED=true or update config.yaml")
		return
	}

	// Initialize Store
	s, err := store.Open(cfg.WhatsApp.StoreDSN)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	// Initialize JWT Generator if configured
	var jwtGen *auth.JWTGenerator
	if cfg.Auth.JWT.PrivateKeyPath != "" {
		ttl := 24 * time.Hour
		if cfg.Auth.JWT.TTL != "" {
			if d, err := time.ParseDuration(cfg.Auth.JWT.TTL); err == nil {
				ttl = d
			}
		}
		jwtGen, err = auth.NewJWTGenerator(cfg.Auth.JWT.PrivateKeyPath, cfg.Auth.JWT.Issuer, cfg.Auth.JWT.Audience, ttl)
		if err != nil {
			log.Fatalf("Failed to initialize JWT: %v", err)
		}
	}

	// Initialize Clients
	adkClient := agent.NewClient(&cfg.ADK, jwtGen)
	wabaClient := waba.NewClient(&cfg.WABA)
	mediaProc := whatsapp.NewProcessor()

	// Webhook handler logic
	onMessageParts := func(sender string, parts []agent.Part) {
		appLogger.Info("Received WABA message parts", "sender", sender, "parts_count", len(parts))

		ctx := context.Background()
		uniqueID := fmt.Sprintf("waba_%d", time.Now().UnixNano())

		var processedParts []agent.Part
		for _, part := range parts {
			// Handle inbound media (Media ID protocol)
			if part.InlineData != nil && strings.HasPrefix(part.InlineData.Data, "media_id:") {
				mediaID := strings.TrimPrefix(part.InlineData.Data, "media_id:")
				appLogger.Info("Downloading WABA media", "media_id", mediaID)

				mediaInfo, err := wabaClient.GetMediaURL(ctx, mediaID)
				if err != nil {
					appLogger.Error("Failed to get media URL for WABA media", "media_id", mediaID, "error", err)
					continue
				}

				data, err := wabaClient.DownloadMedia(ctx, mediaInfo.URL)
				if err != nil {
					appLogger.Error("Failed to download WABA media", "media_id", mediaID, "error", err)
					continue
				}

				// Store raw media
				path := fmt.Sprintf("waba/%s/%s/request_media", sender, uniqueID)
				metadata := map[string]interface{}{
					"mime_type": mediaInfo.MimeType,
					"metadata":  map[string]interface{}{"is_from_me": false, "media_id": mediaID},
				}
				s.PutFile(ctx, path, metadata, data, time.Now())

				// Process for ADK
				if strings.HasPrefix(mediaInfo.MimeType, "image/") {
					pPart, pErr := mediaProc.ProcessImage(ctx, data)
					if pErr == nil {
						processedParts = append(processedParts, *pPart)
					} else {
						appLogger.Error("Failed to process image", "error", pErr)
					}
				}
			} else {
				// Regular text part
				processedParts = append(processedParts, part)

				// Store text request
				if part.Text != "" {
					path := fmt.Sprintf("waba/%s/%s/request", sender, uniqueID)
					metadata := map[string]interface{}{
						"mime_type": "text/plain",
						"metadata":  map[string]interface{}{"is_from_me": false},
					}
					s.PutFile(ctx, path, metadata, []byte(part.Text), time.Now())
				}
			}
		}

		if len(processedParts) == 0 {
			return
		}

		// 2. Forward to ADK
		respParts, err := adkClient.ChatParts(ctx, sender, processedParts)
		if err != nil {
			appLogger.Error("ADK Error", "error", err)
			wabaClient.SendText(ctx, sender, "Sorry, I encountered an error processing your message.")
			return
		}

		// 3. Send and Store Response
		var caption string
		for _, part := range respParts {
			if part.Text != "" {
				// If we haven't sent media yet, accumulate text as caption
				// Otherwise send as separate message
				caption = part.Text
				if err := wabaClient.SendText(ctx, sender, part.Text); err != nil {
					appLogger.Error("WABA Send Error", "error", err)
				}

				respPath := fmt.Sprintf("waba/%s/%s/response", sender, uniqueID)
				respMetadata := map[string]interface{}{
					"mime_type": "text/plain",
					"metadata": map[string]interface{}{
						"context_type": "response",
						"msg_ref":      uniqueID,
					},
				}
				s.PutFile(ctx, respPath, respMetadata, []byte(part.Text), time.Now())
				continue
			}

			if part.InlineData != nil {
				data, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
				if err != nil {
					appLogger.Error("Failed to decode outbound media", "error", err)
					continue
				}

				// Only handling images for now as requested
				if strings.HasPrefix(part.InlineData.MimeType, "image/") {
					fileName := fmt.Sprintf("image-%d.jpg", time.Now().Unix())
					mediaID, err := wabaClient.UploadMedia(ctx, data, fileName, part.InlineData.MimeType)
					if err != nil {
						appLogger.Error("WABA Upload Error", "error", err)
						continue
					}

					if err := wabaClient.SendImage(ctx, sender, mediaID, caption); err != nil {
						appLogger.Error("WABA Send Image Error", "error", err)
					}

					// Reset caption after use
					caption = ""

					respPath := fmt.Sprintf("waba/%s/%s/response_media", sender, uniqueID)
					respMetadata := map[string]interface{}{
						"mime_type": part.InlineData.MimeType,
						"metadata": map[string]interface{}{
							"media_id":     mediaID,
							"context_type": "response",
							"msg_ref":      uniqueID,
						},
					}
					s.PutFile(ctx, respPath, respMetadata, []byte("[IMAGE]"), time.Now())
				} else {
					appLogger.Warn("Outbound non-image media detected but not yet supported for WABA in this loop")
				}
			}
		}
	}

	// Legacy text-only handler
	onMessage := func(sender, text string) {
		onMessageParts(sender, []agent.Part{{Text: text}})
	}

	handler := waba.NewWebhookHandler(&cfg.WABA, onMessage)
	handler.SetOnMessageParts(onMessageParts)

	addr := fmt.Sprintf(":%d", cfg.WABA.Port)
	fmt.Printf("🚀 WABA Gateway listening on %s/webhook\n", addr)

	http.Handle("/webhook", handler)

	server := &http.Server{
		Addr: addr,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
