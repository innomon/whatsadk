// Package main provides the Multi-Device (QR Code) WhatsApp-ADK Gateway executable.
//
// The gateway establishes a linked multi-device session with WhatsApp using whatsmeow,
// bridging incoming messages to Google ADK agent endpoints over REST or SSE streaming.
// It provides built-in media transcoding, JWT authentication (RS256), OAuth logins (EdDSA),
// reverse OTP verification, cron heartbeat scheduling, and multi-database persistence
// (PostgreSQL, SurrealDB, and SQLite-P2P).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/cron"
	"github.com/innomon/whatsadk/internal/logger"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/verification"
	"github.com/innomon/whatsadk/internal/whatsapp"
)

// main is the entry point for the Multi-Device WhatsApp-ADK Gateway.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	appLogger, err := logger.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	appLogger.Info("Structured logging initialized")

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

	var gwStore *store.Store
	if cfg.P2P.Enabled {
		gwStore, err = store.OpenP2PFromNodeConfig(cfg.P2P.ToNodeConfig())
		if err != nil {
			log.Fatalf("Failed to open P2P gateway store: %v", err)
		}
		fmt.Printf("🌐 SQLite P2P Mesh enabled (Node: %s, Topic: %s)\n", cfg.P2P.NodeID, cfg.P2P.SwarmTopic)
	} else {
		gwStore, err = store.Open(cfg.Verification.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to open gateway store: %v", err)
		}
	}
	defer gwStore.Close()

	var verifyHandler *verification.Handler
	if cfg.Verification.Enabled {
		keyRegistry, err := auth.NewKeyRegistry(cfg.Verification.Apps)
		if err != nil {
			log.Fatalf("Failed to load verification app keys: %v", err)
		}
		if jwtGen == nil {
			log.Fatalf("Verification requires JWT auth to be enabled (private_key_path must be set) ")
		}

		timeout, _ := time.ParseDuration(cfg.Verification.CallbackTimeout)
		if timeout == 0 {
			timeout = 10 * time.Second
		}

		verifyHandler = verification.NewHandler(
			keyRegistry,
			jwtGen,
			gwStore,
			cfg.Verification,
			&http.Client{Timeout: timeout},
			appLogger,
		)
		fmt.Printf("🔑 Verification enabled (%d app(s) registered)\n", len(cfg.Verification.Apps))
	}

	// Initialize Cron Heartbeats
	if cfg.Cron.Enabled {
		cronStore := cron.NewStore(gwStore)
		cronManager := cron.NewManager(ctx, cfg, cronStore, jwtGen)
		if err := cronManager.Start(); err != nil {
			log.Printf("⚠️ Failed to start cron manager: %v", err)
		} else {
			fmt.Printf("⏰ Cron heartbeats enabled (%d job(s) scheduled)\n", len(cfg.Cron.Jobs))
			defer cronManager.Stop()
		}
	}

	var oauthHandler *auth.OAuthHandler

	if cfg.Auth.OAuth.Enabled {
		ttl, err := time.ParseDuration(cfg.Auth.OAuth.TTL)
		if err != nil {
			log.Fatalf("Invalid OAuth TTL %q: %v", cfg.Auth.OAuth.TTL, err)
		}
		tokenGen, err := auth.NewOAuthTokenGenerator(
			cfg.Auth.OAuth.KeyPath,
			cfg.Auth.OAuth.Issuer,
			cfg.Auth.OAuth.Audience,
			ttl,
		)
		if err != nil {
			log.Fatalf("Failed to initialize OAuth token generator: %v", err)
		}
		oauthHandler = auth.NewOAuthHandler(tokenGen, cfg.Auth.OAuth.SPAURL, cfg.Auth.OAuth.RateLimit)
		fmt.Println("🔑 WhatsApp OAuth enabled (EdDSA)")
	}

	fmt.Println("🚀 Starting WhatsApp-ADK Gateway...")
	fmt.Printf("📡 Connecting to ADK service: %s\n", cfg.ADK.Endpoint)
	fmt.Printf("🤖 Agent: %s\n", cfg.ADK.AppName)

	adkClient := agent.NewClient(&cfg.ADK, jwtGen)

	client, err := whatsapp.New(ctx, cfg, adkClient, verifyHandler, oauthHandler, gwStore)

	if err != nil {
		log.Fatalf("Failed to create WhatsApp client: %v", err)
	}

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to WhatsApp: %v", err)
	}

	if err := client.Run(ctx); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}

	fmt.Println("👋 Gateway stopped.")
}
