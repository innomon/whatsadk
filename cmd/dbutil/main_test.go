package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/innomon/whatsadk/internal/store"
)

func TestExportImport(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	s, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Clean up any test records we might create
	testPhone := "919999999999"
	testPath := "test/dbutil_file.txt"
	testOurJID := "our_jid_test@s.whatsapp.net"
	testTheirJID := "their_jid_test@s.whatsapp.net"

	t.Cleanup(func() {
		_ = s.RemoveBlacklist(ctx, testPhone)
		_ = s.DeleteFile(ctx, testPath)
	})

	// 1. Insert test data
	err = s.AddBlacklist(ctx, testPhone, "test export reason")
	if err != nil {
		t.Fatalf("failed to add blacklist: %v", err)
	}

	err = s.PutContact(ctx, store.Contact{
		OurJID:    testOurJID,
		TheirJID:  testTheirJID,
		FullName:  "Test Full Name",
		ShortName: "Test Short",
	})
	if err != nil {
		t.Fatalf("failed to put contact: %v", err)
	}

	payload := map[string]string{"arg": "val"}
	cmdID, err := s.EnqueueCommand(ctx, "test_dbutil_cmd", payload)
	if err != nil {
		t.Fatalf("failed to enqueue command: %v", err)
	}

	err = s.PutFile(ctx, testPath, map[string]string{"mime": "text"}, []byte("test content data"), time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to put file: %v", err)
	}

	// Create temp file for export
	tmpDir, err := os.MkdirTemp("", "dbutil_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	exportFile := filepath.Join(tmpDir, "export.jsonl")

	// 2. Run Export
	exporter := &exportCmd{}
	err = exporter.Run(ctx, s, []string{"-out", exportFile})
	if err != nil {
		t.Fatalf("export run failed: %v", err)
	}

	// Verify file exists and has content
	if _, err := os.Stat(exportFile); os.IsNotExist(err) {
		t.Fatalf("export file was not created")
	}

	// 3. Clear data from database so we can test import restoring it
	err = s.RemoveBlacklist(ctx, testPhone)
	if err != nil {
		t.Fatalf("failed to clear blacklist: %v", err)
	}
	err = s.DeleteFile(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to clear file: %v", err)
	}

	// Verify they are deleted
	isBl, err := s.IsBlacklisted(ctx, testPhone)
	if err != nil || isBl {
		t.Fatalf("blacklist not cleared")
	}
	fl, err := s.GetFile(ctx, testPath)
	if err != nil || fl != nil {
		t.Fatalf("file not cleared")
	}

	// 4. Run Import
	importer := &importCmd{}
	err = importer.Run(ctx, s, []string{"-in", exportFile})
	if err != nil {
		t.Fatalf("import run failed: %v", err)
	}

	// 5. Verify restored data
	isBl, err = s.IsBlacklisted(ctx, testPhone)
	if err != nil || !isBl {
		t.Fatalf("blacklist not restored")
	}

	fl, err = s.GetFile(ctx, testPath)
	if err != nil || fl == nil {
		t.Fatalf("file not restored")
	}
	if string(fl.Content) != "test content data" {
		t.Errorf("expected content 'test content data', got %q", string(fl.Content))
	}

	// Verify contact
	contacts, err := s.GetAllContacts(ctx)
	if err != nil {
		t.Fatalf("failed to get contacts: %v", err)
	}
	foundContact := false
	for _, ct := range contacts {
		if ct.OurJID == testOurJID && ct.TheirJID == testTheirJID {
			foundContact = true
			if ct.FullName != "Test Full Name" {
				t.Errorf("expected fullname 'Test Full Name', got %q", ct.FullName)
			}
		}
	}
	if !foundContact {
		t.Errorf("contact not found after import")
	}

	// Verify command
	commands, err := s.GetAllCommands(ctx)
	if err != nil {
		t.Fatalf("failed to get commands: %v", err)
	}
	foundCommand := false
	for _, c := range commands {
		if c.ID == cmdID {
			foundCommand = true
			if c.Command != "test_dbutil_cmd" {
				t.Errorf("expected command 'test_dbutil_cmd', got %q", c.Command)
			}
		}
	}
	if !foundCommand {
		t.Errorf("command not found after import")
	}
}
