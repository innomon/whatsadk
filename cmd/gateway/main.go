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

	if cfg.ADK.Endpoint == "" {
		fmt.Println("Error: ADK endpoint is required")
		fmt.Println("Set it in config.yaml or via ADK_ENDPOINT environment variable")
		os.Exit(1)
	}

	fmt.Println("🚀 Starting WhatsApp-ADK Gateway...")
	fmt.Printf("📡 Connecting to ADK service: %s\n", cfg.ADK.Endpoint)
	fmt.Printf("🤖 Agent: %s\n", cfg.ADK.AppName)

	adkClient := agent.NewClient(&cfg.ADK)

	client, err := whatsapp.New(ctx, cfg, adkClient)
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
