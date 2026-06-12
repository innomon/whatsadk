package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	s, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	ctx := context.Background()
	if isSurrealDB(dsn) {
		_, _ = s.QueryFilesys(ctx, "DELETE FROM blacklisted_numbers")
		_, _ = s.QueryFilesys(ctx, "DELETE FROM whatsmeow_contacts")
		_, _ = s.QueryFilesys(ctx, "DELETE FROM whatsmeow_commands")
		_, _ = s.QueryFilesys(ctx, "DELETE FROM filesys")
		_, _ = s.QueryFilesys(ctx, "DELETE FROM counter")
	} else {
		_, _ = s.QueryFilesys(ctx, "TRUNCATE TABLE blacklisted_numbers, whatsmeow_contacts, whatsmeow_commands, filesys CASCADE")
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIsBlacklisted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	ok, err := s.IsBlacklisted(ctx, "910987654321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not blacklisted")
	}

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	ok, err = s.IsBlacklisted(ctx, "910987654321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected blacklisted")
	}
}

func TestRemoveBlacklist(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	if err := s.RemoveBlacklist(ctx, "910987654321"); err != nil {
		t.Fatalf("failed to remove: %v", err)
	}

	ok, err := s.IsBlacklisted(ctx, "910987654321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not blacklisted after removal")
	}
}

func TestListBlacklist(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := s.AddBlacklist(ctx, "910111111111", "abuse"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	list, err := s.ListBlacklist(ctx)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
}

func TestAddBlacklist_Duplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := s.AddBlacklist(ctx, "910987654321", "spam again"); err != nil {
		t.Fatalf("duplicate add should not error: %v", err)
	}

	list, err := s.ListBlacklist(ctx)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 entry after duplicate add, got %d", len(list))
	}
}

func TestCommands(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := map[string]string{"foo": "bar"}
	id, err := s.EnqueueCommand(ctx, "test_cmd", payload)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	cmds, err := s.PollPendingCommands(ctx)
	if err != nil {
		t.Fatalf("failed to poll: %v", err)
	}

	found := false
	for _, c := range cmds {
		if c.ID == id {
			found = true
			if c.Command != "test_cmd" {
				t.Errorf("expected command name test_cmd, got %q", c.Command)
			}
			break
		}
	}
	if !found {
		t.Errorf("enqueued command %d not found in pending list", id)
	}

	err = s.UpdateCommandStatus(ctx, id, "completed", map[string]string{"result": "success"})
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	cmds, err = s.PollPendingCommands(ctx)
	if err != nil {
		t.Fatalf("failed to poll after update: %v", err)
	}
	for _, c := range cmds {
		if c.ID == id {
			t.Errorf("command %d should no longer be pending", id)
		}
	}
}

func TestFilesys(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	path := "test/file.txt"
	metadata := map[string]string{"mime_type": "text/plain"}
	content := []byte("hello world")
	ts := time.Now().UTC()

	err := s.PutFile(ctx, path, metadata, content, ts)
	if err != nil {
		t.Fatalf("failed to put file: %v", err)
	}

	file, err := s.GetFile(ctx, path)
	if err != nil {
		t.Fatalf("failed to get file: %v", err)
	}
	if file == nil {
		t.Fatalf("expected file to be found")
	}
	if file.Path != path {
		t.Errorf("expected path %q, got %q", path, file.Path)
	}
	if string(file.Content) != "hello world" {
		t.Errorf("expected content 'hello world', got %q", string(file.Content))
	}

	// Clean up
	err = s.DeleteFile(ctx, path)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	file, err = s.GetFile(ctx, path)
	if err != nil {
		t.Fatalf("failed to get file after delete: %v", err)
	}
	if file != nil {
		t.Errorf("expected file to be deleted")
	}
}
