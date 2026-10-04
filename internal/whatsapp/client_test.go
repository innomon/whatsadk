package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
)

func TestNewClient_SQLiteP2P(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "whatsadk_whatsapp_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "session.db")
	cfg := &config.Config{
		WhatsApp: config.WhatsAppConfig{
			StoreDSN: "sqlite-p2p://" + dbPath,
		},
	}

	st, err := store.Open("sqlite-p2p://:memory:")
	if err != nil {
		t.Fatalf("failed to open memory store: %v", err)
	}
	defer st.Close()

	client, err := New(ctx, cfg, nil, nil, nil, st)
	if err != nil {
		t.Fatalf("failed to initialize whatsapp client with sqlite-p2p DSN: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
