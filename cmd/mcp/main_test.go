package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/innomon/whatsadk/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_mcp.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cmds, err := s.PollPendingCommands(ctx)
				if err == nil {
					for _, cmd := range cmds {
						_ = s.UpdateCommandStatus(ctx, cmd.ID, "completed", map[string]string{"status": "sent"})
					}
				}
			}
		}
	}()

	t.Cleanup(func() {
		cancel()
		_ = s.Close()
	})
	return s
}

func TestMCP_BlacklistTools(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	t.Run("BlacklistAddAndRemove", func(t *testing.T) {
		phone := "919876543210"
		reason := "test blacklist"

		// Add
		res, _, err := BlacklistAdd(ctx, s, BlacklistAddArgs{
			Phone:  phone,
			Reason: reason,
		})
		if err != nil {
			t.Fatalf("BlacklistAdd failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected non-empty CallToolResult")
		}

		isBl, err := s.IsBlacklisted(ctx, phone)
		if err != nil || !isBl {
			t.Errorf("expected %s to be blacklisted", phone)
		}

		// Remove
		res, _, err = BlacklistRemove(ctx, s, BlacklistRemoveArgs{
			Phone: phone,
		})
		if err != nil {
			t.Fatalf("BlacklistRemove failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected non-empty CallToolResult")
		}

		isBl, err = s.IsBlacklisted(ctx, phone)
		if err != nil || isBl {
			t.Errorf("expected %s to no longer be blacklisted", phone)
		}
	})
}

func TestMCP_FilesysTools(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	t.Run("FilesysCRUD", func(t *testing.T) {
		path := "test/mcp_file.txt"
		content := "Hello MCP SQLite-P2P"
		rawMeta := json.RawMessage(`{"mime_type":"text/plain"}`)

		// Put
		_, _, err := FileSysPut(ctx, s, FileSysPutArgs{
			Path:     path,
			Content:  content,
			Metadata: rawMeta,
		})
		if err != nil {
			t.Fatalf("FileSysPut failed: %v", err)
		}

		// Get
		getRes, _, err := FileSysGet(ctx, s, FileSysGetArgs{Path: path})
		if err != nil {
			t.Fatalf("FileSysGet failed: %v", err)
		}
		if getRes == nil || len(getRes.Content) == 0 {
			t.Fatalf("expected FileSysGet content")
		}

		// List
		listRes, _, err := FileSysList(ctx, s, FileSysListArgs{Prefix: "test/", Limit: 10})
		if err != nil {
			t.Fatalf("FileSysList failed: %v", err)
		}
		if listRes == nil || len(listRes.Content) == 0 {
			t.Fatalf("expected FileSysList content")
		}

		// Delete
		_, _, err = FileSysDelete(ctx, s, FileSysDeleteArgs{Path: path})
		if err != nil {
			t.Fatalf("FileSysDelete failed: %v", err)
		}

		// Verify deleted
		deletedEntry, err := s.GetFile(ctx, path)
		if err != nil || deletedEntry != nil {
			t.Errorf("expected file to be deleted, got %v", deletedEntry)
		}
	})
}

func TestMCP_ContactsAndMessages(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	t.Run("QueryContacts", func(t *testing.T) {
		err := s.PutContact(ctx, store.Contact{
			OurJID:       "me@s.whatsapp.net",
			TheirJID:     "919876543210@s.whatsapp.net",
			FullName:     "Bob Developer",
			ShortName:    "Bob",
			PushName:     "Bob D",
			BusinessName: "Bob Corp",
		})
		if err != nil {
			t.Fatalf("failed to put contact: %v", err)
		}

		res, _, err := QueryContacts(ctx, s, QueryContactsArgs{Query: "Bob"})
		if err != nil {
			t.Fatalf("QueryContacts failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected query result")
		}
	})

	t.Run("GetRecentMessages", func(t *testing.T) {
		path := "whatsmeow/919876543210/msg123/request"
		_ = s.PutFile(ctx, path, map[string]string{"type": "chat"}, []byte("test message"), time.Now().UTC())

		res, _, err := GetRecentMessages(ctx, s, GetRecentMessagesArgs{Limit: 10})
		if err != nil {
			t.Fatalf("GetRecentMessages failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected recent messages")
		}
	})

	t.Run("SendMessage", func(t *testing.T) {
		res, _, err := SendMessage(ctx, s, SendMessageArgs{
			JID:  "919876543210@s.whatsapp.net",
			Text: "Hello from MCP test",
		})
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected SendMessage result")
		}
	})

	t.Run("GetDatabaseType", func(t *testing.T) {
		res, _, err := GetDatabaseType(ctx, s, GetDatabaseTypeArgs{})
		if err != nil {
			t.Fatalf("GetDatabaseType failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected GetDatabaseType result")
		}
	})

	t.Run("GetGroups", func(t *testing.T) {
		res, _, err := GetGroups(ctx, s, GetGroupsArgs{})
		if err != nil {
			t.Fatalf("GetGroups failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected GetGroups result")
		}
	})

	t.Run("GetGroupInfo", func(t *testing.T) {
		res, _, err := GetGroupInfo(ctx, s, GetGroupInfoArgs{JID: "1234567890@g.us"})
		if err != nil {
			t.Fatalf("GetGroupInfo failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected GetGroupInfo result")
		}
	})

	t.Run("JIDToPhone", func(t *testing.T) {
		res, _, err := JIDToPhone(ctx, s, JIDToPhoneArgs{JID: "15551234567:2@s.whatsapp.net"})
		if err != nil {
			t.Fatalf("JIDToPhone failed: %v", err)
		}
		if res == nil || len(res.Content) == 0 {
			t.Fatalf("expected JIDToPhone result")
		}

		// Test group JID
		groupRes, _, err := JIDToPhone(ctx, s, JIDToPhoneArgs{JID: "12036301234567890@g.us"})
		if err != nil {
			t.Fatalf("JIDToPhone for group failed: %v", err)
		}
		if groupRes == nil || len(groupRes.Content) == 0 {
			t.Fatalf("expected JIDToPhone group result")
		}
	})
}
