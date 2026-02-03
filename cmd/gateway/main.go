package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/whatsapp"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Agent.APIKey == "" {
		fmt.Println("Error: GOOGLE_API_KEY environment variable is required")
		fmt.Println("Set it with: export GOOGLE_API_KEY=your-api-key")
		os.Exit(1)
	}

	fmt.Println("🚀 Starting WhatsApp-ADK Gateway...")

	gateway, err := agent.New(ctx, &cfg.Agent)
	if err != nil {
		log.Fatalf("Failed to create agent gateway: %v", err)
	}

	client, err := whatsapp.New(ctx, cfg, gateway)
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
