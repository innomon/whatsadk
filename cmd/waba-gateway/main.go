package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/waba"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	// Webhook handler logic
	onMessage := func(sender, text string) {
		log.Printf("Received WABA message from %s: %s", sender, text)
		
		ctx := context.Background()
		
		// 1. Store Request
		uniqueID := fmt.Sprintf("waba_%d", time.Now().UnixNano())
		path := fmt.Sprintf("waba/%s/%s/request", sender, uniqueID)
		metadata := map[string]interface{}{
			"mime_type": "text/plain",
			"metadata":  map[string]interface{}{"is_from_me": false},
		}
		if err := s.PutFile(ctx, path, metadata, []byte(text), time.Now()); err != nil {
			log.Printf("Failed to store request: %v", err)
		}

		// 2. Forward to ADK
		parts, err := adkClient.Chat(ctx, sender, text)
		if err != nil {
			log.Printf("ADK Error: %v", err)
			wabaClient.SendText(ctx, sender, "Sorry, I encountered an error processing your message.")
			return
		}

		// 3. Send and Store Response
		for _, part := range parts {
			if part.Text != "" {
				if err := wabaClient.SendText(ctx, sender, part.Text); err != nil {
					log.Printf("WABA Send Error: %v", err)
					continue
				}
				
				respPath := fmt.Sprintf("waba/%s/%s/response", sender, uniqueID)
				respMetadata := map[string]interface{}{
					"mime_type": "text/plain",
					"metadata":  map[string]interface{}{},
				}
				s.PutFile(ctx, respPath, respMetadata, []byte(part.Text), time.Now())
			}
		}
	}

	handler := waba.NewWebhookHandler(&cfg.WABA, onMessage)

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
