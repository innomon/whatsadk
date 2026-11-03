package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/verification"
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

func TestClient_HandleMessage_HMACVerification(t *testing.T) {
	ctx := context.Background()

	type webhookPayload struct {
		From string `json:"from"`
		Body string `json:"body"`
	}

	webhookReceived := make(chan webhookPayload, 1)
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p webhookPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		webhookReceived <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookSrv.Close()

	appSecret := "staging-secret-key-999"
	vCfg := config.VerificationConfig{
		Enabled: true,
		RateLimit: config.RateLimitConfig{
			MaxAttempts: 5,
			Window:      "1m",
		},
		Apps: map[string]config.AppVerifyConfig{
			"kb": {
				Name:           "kheti-badi",
				AuthType:       "hmac",
				Secret:         appSecret,
				WebhookURL:     webhookSrv.URL,
				TTL:            "5m",
				SuccessMessage: "✅ Verification successful! Please return to Kheti-Badi.",
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	vHandler := verification.NewHandler(nil, nil, nil, vCfg, webhookSrv.Client(), logger)

	st, err := store.Open("sqlite-p2p://:memory:")
	if err != nil {
		t.Fatalf("failed to open memory store: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{
		WhatsApp: config.WhatsAppConfig{
			StoreDSN: "sqlite-p2p://:memory:",
		},
		Verification: vCfg,
	}

	client, err := New(ctx, cfg, nil, vHandler, nil, st)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	senderPhone := "919876543210"
	senderJID := types.NewJID(senderPhone, types.DefaultUserServer)
	uniqueID := "VERIFY_MSG_001"
	token := "fedcba9876543210fedcba9876543210"
	now := time.Now()
	ts := now.Unix()
	sig := verification.ComputeHMAC("kb", token, ts, appSecret)
	verifyText := fmt.Sprintf("VERIFY kb:%s:%d:%s", token, ts, sig)

	msgEvt := &events.Message{
		Info: types.MessageInfo{
			ID:        uniqueID,
			Timestamp: now,
			MessageSource: types.MessageSource{
				Chat:   senderJID,
				Sender: senderJID,
			},
		},
		Message: &waE2E.Message{
			Conversation: proto.String(verifyText),
		},
	}

	client.handleMessage(msgEvt)

	// Verify webhook received the payload
	select {
	case payload := <-webhookReceived:
		if payload.From != senderPhone {
			t.Errorf("expected from=%s, got %s", senderPhone, payload.From)
		}
		if payload.Body != verifyText {
			t.Errorf("expected body=%s, got %s", verifyText, payload.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook callback")
	}

	// Verify audit record in filesys
	auditPath := fmt.Sprintf("whatsmeow/%s/%s/verification", senderPhone, uniqueID)
	rec, err := st.GetFile(ctx, auditPath)
	if err != nil {
		t.Fatalf("failed to get verification audit from filesys: %v", err)
	}
	if rec == nil {
		t.Fatal("expected verification audit record in filesys")
	}
	if !strings.Contains(string(rec.Content), "VERIFY_MSG_001") {
		t.Errorf("expected audit record to contain uniqueID, got %s", string(rec.Content))
	}
}

