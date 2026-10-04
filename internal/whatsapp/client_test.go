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

	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "sqlite-p2p with wal parameter",
			dsn:  "sqlite-p2p://" + filepath.Join(tmpDir, "sub", "session1.db") + "?wal=true",
		},
		{
			name: "sqlite-p2p standard path",
			dsn:  "sqlite-p2p://" + filepath.Join(tmpDir, "session2.db"),
		},
		{
			name: "sqlite prefix with wal",
			dsn:  "sqlite://" + filepath.Join(tmpDir, "session3.db") + "?wal=true",
		},
		{
			name: "p2p prefix with wal",
			dsn:  "p2p://" + filepath.Join(tmpDir, "session4.db") + "?wal=true",
		},
		{
			name: "file URI with foreign keys already set",
			dsn:  "file:" + filepath.Join(tmpDir, "session5.db") + "?_foreign_keys=on",
		},
		{
			name: "memory database",
			dsn:  "sqlite-p2p://:memory:",
		},
	}

	st, err := store.Open("sqlite-p2p://:memory:")
	if err != nil {
		t.Fatalf("failed to open memory store: %v", err)
	}
	defer st.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				WhatsApp: config.WhatsAppConfig{
					StoreDSN: tc.dsn,
				},
			}

			client, err := New(ctx, cfg, nil, nil, nil, st)
			if err != nil {
				t.Fatalf("failed to initialize whatsapp client with DSN %q: %v", tc.dsn, err)
			}
			if client == nil {
				t.Fatal("expected non-nil client")
			}
		})
	}
}
