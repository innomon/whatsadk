package handlers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/tui/registry"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_tui.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

func TestHandlers_RegisterAndExecute(t *testing.T) {
	s := setupTestStore(t)
	reg := registry.NewRegistry()
	RegisterAllHandlers(reg)

	t.Run("JIDToPhone", func(t *testing.T) {
		handler, ok := reg.GetHandler("jid_to_phone")
		if !ok {
			t.Fatalf("expected jid_to_phone handler")
		}

		res, err := handler(context.Background(), s, map[string]any{
			"jid": "15551234567:3@s.whatsapp.net",
		})
		if err != nil {
			t.Fatalf("JIDToPhone handler failed: %v", err)
		}
		if res.Type != registry.ResultTypeMarkdown {
			t.Errorf("expected markdown type, got %s", res.Type)
		}
	})

	t.Run("SQLExec", func(t *testing.T) {
		handler, ok := reg.GetHandler("sql")
		if !ok {
			t.Fatalf("expected sql handler")
		}

		res, err := handler(context.Background(), s, map[string]any{
			"query": "SELECT 1",
		})
		if err != nil {
			t.Fatalf("SQL handler failed: %v", err)
		}
		if res.Type != registry.ResultTypeMarkdown {
			t.Errorf("expected markdown result, got %s", res.Type)
		}
	})

	t.Run("GetDatabaseType", func(t *testing.T) {
		handler, ok := reg.GetHandler("get_database_type")
		if !ok {
			t.Fatalf("expected get_database_type handler")
		}

		res, err := handler(context.Background(), s, map[string]any{})
		if err != nil {
			t.Fatalf("GetDatabaseType handler failed: %v", err)
		}
		if res.Content == "" {
			t.Errorf("expected database type content")
		}
	})
}
