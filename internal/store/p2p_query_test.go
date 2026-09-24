package store

import (
	"context"
	"testing"
	"time"
)

func TestFilesys_GetFilesysLogs(t *testing.T) {
	ctx := context.Background()
	backend, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	now := time.Now().UTC()

	// Put files for phone +123456
	_ = backend.PutFile(ctx, "whatsmeow/123456/msg1", map[string]interface{}{"mime_type": "text/plain"}, []byte("text message 1"), now.Add(-2*time.Minute))
	_ = backend.PutFile(ctx, "whatsmeow/123456/img1", map[string]interface{}{"mime_type": "image/jpeg"}, []byte("binary jpeg data"), now.Add(-1*time.Minute))
	_ = backend.PutFile(ctx, "whatsmeow/123456/msg2", map[string]interface{}{"mime_type": "text/plain"}, []byte("text message 2"), now)

	// Put file for another phone
	_ = backend.PutFile(ctx, "whatsmeow/999999/msg3", map[string]interface{}{"mime_type": "text/plain"}, []byte("other phone"), now)

	logs, err := backend.GetFilesysLogs(ctx, "123456", 10)
	if err != nil {
		t.Fatalf("GetFilesysLogs failed: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(logs))
	}

	// Should be ordered tmstamp DESC (msg2 first, then img1, then msg1)
	if logs[0].Path != "whatsmeow/123456/msg2" {
		t.Fatalf("expected msg2 first, got %s", logs[0].Path)
	}
	// Case check: for text/plain, Content is returned; for image/jpeg, Content should be NULL/empty
	if string(logs[0].Content) != "text message 2" {
		t.Fatalf("expected text content for msg2, got %s", string(logs[0].Content))
	}
	if len(logs[1].Content) != 0 {
		t.Fatalf("expected empty content for non-text image, got %s", string(logs[1].Content))
	}
}

func TestFilesys_GetLatestGlobalMessages(t *testing.T) {
	ctx := context.Background()
	backend, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	now := time.Now().UTC()

	_ = backend.PutFile(ctx, "whatsmeow/111/request", map[string]interface{}{"mime_type": "text/plain"}, []byte("req 111"), now.Add(-3*time.Minute))
	_ = backend.PutFile(ctx, "whatsmeow/111/chatter", map[string]interface{}{"mime_type": "text/plain"}, []byte("chatter"), now.Add(-2*time.Minute))
	_ = backend.PutFile(ctx, "whatsmeow/222/response", map[string]interface{}{"mime_type": "text/plain"}, []byte("resp 222"), now.Add(-1*time.Minute))

	msgs, err := backend.GetLatestGlobalMessages(ctx, 10)
	if err != nil {
		t.Fatalf("GetLatestGlobalMessages failed: %v", err)
	}
	// Should only include request and response paths, not chatter
	if len(msgs) != 2 {
		t.Fatalf("expected 2 global messages, got %d", len(msgs))
	}
	if msgs[0].Path != "whatsmeow/222/response" || msgs[1].Path != "whatsmeow/111/request" {
		t.Fatalf("unexpected message paths: %s, %s", msgs[0].Path, msgs[1].Path)
	}
}

func TestFilesys_QueryFilesys(t *testing.T) {
	ctx := context.Background()
	backend, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	now := time.Now().UTC()
	_ = backend.PutFile(ctx, "alpha.txt", map[string]interface{}{"mime_type": "text/plain"}, []byte("alpha content"), now)
	_ = backend.PutFile(ctx, "beta.json", map[string]interface{}{"mime_type": "application/json"}, []byte(`{"a":1}`), now)

	// Test with SQLite '?' syntax
	res1, err := backend.QueryFilesys(ctx, "SELECT path, content FROM filesys WHERE path = ?", "alpha.txt")
	if err != nil {
		t.Fatalf("QueryFilesys with ? failed: %v", err)
	}
	if len(res1) != 1 || res1[0]["path"] != "alpha.txt" || res1[0]["content"] != "alpha content" {
		t.Fatalf("unexpected result with ?: %v", res1)
	}

	// Test with PostgreSQL '$1' parameter syntax (should be handled smoothly)
	res2, err := backend.QueryFilesys(ctx, "SELECT path FROM filesys WHERE path = $1", "beta.json")
	if err != nil {
		t.Fatalf("QueryFilesys with $1 failed: %v", err)
	}
	if len(res2) != 1 || res2[0]["path"] != "beta.json" {
		t.Fatalf("unexpected result with $1: %v", res2)
	}
}
