package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/verification"
	"github.com/innomon/whatsadk/internal/whatsapp"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.ADK.Endpoint == "" {
		fmt.Println("Error: ADK endpoint is required")
		fmt.Println("Set it in config.yaml or via ADK_ENDPOINT environment variable")
		os.Exit(1)
	}

	var jwtGen *auth.JWTGenerator
	if cfg.Auth.JWT.PrivateKeyPath != "" {
		ttl := 2 * time.Minute
		if cfg.Auth.JWT.TTL != "" {
			parsed, err := time.ParseDuration(cfg.Auth.JWT.TTL)
			if err != nil {
				log.Fatalf("Invalid JWT TTL %q: %v", cfg.Auth.JWT.TTL, err)
			}
			ttl = parsed
		}

		jwtGen, err = auth.NewJWTGenerator(
			cfg.Auth.JWT.PrivateKeyPath,
			cfg.Auth.JWT.Issuer,
			cfg.Auth.JWT.Audience,
			ttl,
		)
		if err != nil {
			log.Fatalf("Failed to initialize JWT auth: %v", err)
		}
		fmt.Println("🔐 JWT authentication enabled (RS256)")
	}

	var verifyHandler *verification.Handler
	if cfg.Verification.Enabled {
		keyRegistry, err := auth.NewKeyRegistry(cfg.Verification.Apps)
		if err != nil {
			log.Fatalf("Failed to load verification app keys: %v", err)
		}
		if jwtGen == nil {
			log.Fatalf("Verification requires JWT auth to be enabled (private_key_path must be set)")
		}

		gwStore, err := store.Open(cfg.Verification.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to open gateway store: %v", err)
		}
		defer gwStore.Close()

		timeout, _ := time.ParseDuration(cfg.Verification.CallbackTimeout)
		if timeout == 0 {
			timeout = 10 * time.Second
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		verifyHandler = verification.NewHandler(
			keyRegistry,
			jwtGen,
			gwStore,
			cfg.Verification,
			&http.Client{Timeout: timeout},
			logger,
		)
		fmt.Printf("🔑 Verification enabled (%d app(s) registered)\n", len(cfg.Verification.Apps))
	}

	fmt.Println("🚀 Starting WhatsApp-ADK Gateway...")
	fmt.Printf("📡 Connecting to ADK service: %s\n", cfg.ADK.Endpoint)
	fmt.Printf("🤖 Agent: %s\n", cfg.ADK.AppName)

	adkClient := agent.NewClient(&cfg.ADK, jwtGen)

	client, err := whatsapp.New(ctx, cfg, adkClient, verifyHandler)
	if err != nil {
		log.Fatalf("Failed to create WhatsApp client: %v", err)
	}

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to WhatsApp: %v", err)
	}

	if err := client.Run(ctx); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}

	fmt.Println("👋 Gateway stopped")
}
