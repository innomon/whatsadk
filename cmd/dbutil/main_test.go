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

func TestExportImport_SQLiteP2P(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_p2p.db")
	exportFile := filepath.Join(tmpDir, "export.jsonl")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open SQLite P2P store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	testPhone := "919876543210"
	testPath := "logs/incoming/msg123.txt"
	testOurJID := "me@s.whatsapp.net"
	testTheirJID := "friend@s.whatsapp.net"

	if err := s.AddBlacklist(ctx, testPhone, "spam number"); err != nil {
		t.Fatalf("failed to add blacklist: %v", err)
	}
	if err := s.PutContact(ctx, store.Contact{
		OurJID:       testOurJID,
		TheirJID:     testTheirJID,
		FullName:     "Alice Smith",
		ShortName:    "Alice",
		PushName:     "Alice S",
		BusinessName: "Alice Corp",
	}); err != nil {
		t.Fatalf("failed to put contact: %v", err)
	}
	cmdID, err := s.EnqueueCommand(ctx, "send_message", map[string]string{"to": testTheirJID, "body": "hello"})
	if err != nil {
		t.Fatalf("failed to enqueue command: %v", err)
	}
	if err := s.PutFile(ctx, testPath, map[string]string{"type": "chat"}, []byte("Hello world!"), time.Now().UTC()); err != nil {
		t.Fatalf("failed to put file: %v", err)
	}

	// 1. Export
	exporter := &exportCmd{}
	if err := exporter.Run(ctx, s, []string{"-out", exportFile}); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Open a clean new SQLite P2P database to test importing into
	targetDBPath := filepath.Join(tmpDir, "target_p2p.db")
	targetStore, err := store.Open(targetDBPath)
	if err != nil {
		t.Fatalf("failed to open target SQLite P2P store: %v", err)
	}
	defer targetStore.Close()

	// 2. Import into target database
	importer := &importCmd{}
	if err := importer.Run(ctx, targetStore, []string{"-in", exportFile}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// 3. Verify blacklist
	isBl, err := targetStore.IsBlacklisted(ctx, testPhone)
	if err != nil || !isBl {
		t.Fatalf("blacklist not found in imported database")
	}

	// 4. Verify contact
	contacts, err := targetStore.GetAllContacts(ctx)
	if err != nil || len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d (err: %v)", len(contacts), err)
	}
	if contacts[0].FullName != "Alice Smith" {
		t.Errorf("expected FullName Alice Smith, got %s", contacts[0].FullName)
	}

	// 5. Verify command
	commands, err := targetStore.GetAllCommands(ctx)
	if err != nil || len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d (err: %v)", len(commands), err)
	}
	if commands[0].ID != cmdID || commands[0].Command != "send_message" {
		t.Errorf("command mismatch: %+v", commands[0])
	}

	// 6. Verify file
	fileEntry, err := targetStore.GetFile(ctx, testPath)
	if err != nil || fileEntry == nil {
		t.Fatalf("expected file entry, got nil (err: %v)", err)
	}
	if string(fileEntry.Content) != "Hello world!" {
		t.Errorf("expected content 'Hello world!', got %s", string(fileEntry.Content))
	}
}

func TestCommandRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	t.Run("GetExistingCommands", func(t *testing.T) {
		exportCmd, ok := registry.Get("export")
		if !ok || exportCmd == nil {
			t.Fatalf("expected export command in registry")
		}
		importCmd, ok := registry.Get("import")
		if !ok || importCmd == nil {
			t.Fatalf("expected import command in registry")
		}
	})

	t.Run("GetUnknownCommand", func(t *testing.T) {
		cmd, ok := registry.Get("nonexistent")
		if ok || cmd != nil {
			t.Fatalf("expected unknown command to return false")
		}
	})

	t.Run("CommandsList", func(t *testing.T) {
		cmds := registry.Commands()
		if len(cmds) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(cmds))
		}
	})
}

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantCmd     string
		wantConfig  string
		wantDB      string
		wantDSN     string
		wantSubArgs []string
		wantHelp    bool
	}{
		{
			name:        "export with out",
			args:        []string{"export", "-out", "backup.jsonl"},
			wantCmd:     "export",
			wantSubArgs: []string{"-out", "backup.jsonl"},
		},
		{
			name:        "config before subcommand",
			args:        []string{"-config", "config.yaml", "export", "-out", "backup.jsonl"},
			wantCmd:     "export",
			wantConfig:  "config.yaml",
			wantSubArgs: []string{"-out", "backup.jsonl"},
		},
		{
			name:        "config after subcommand",
			args:        []string{"export", "-config", "config.yaml", "-out", "backup.jsonl"},
			wantCmd:     "export",
			wantConfig:  "config.yaml",
			wantSubArgs: []string{"-out", "backup.jsonl"},
		},
		{
			name:        "db flag before subcommand",
			args:        []string{"-db", "data/p2p.db", "import", "-in", "dump.jsonl"},
			wantCmd:     "import",
			wantDB:      "data/p2p.db",
			wantSubArgs: []string{"-in", "dump.jsonl"},
		},
		{
			name:        "dsn flag after subcommand",
			args:        []string{"export", "-dsn", "sqlite-p2p://data/p2p.db", "-out", "dump.jsonl"},
			wantCmd:     "export",
			wantDSN:     "sqlite-p2p://data/p2p.db",
			wantSubArgs: []string{"-out", "dump.jsonl"},
		},
		{
			name:     "global help",
			args:     []string{"-help"},
			wantHelp: true,
		},
		{
			name:        "subcommand help",
			args:        []string{"export", "-help"},
			wantCmd:     "export",
			wantSubArgs: []string{"-help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := parseCLIArgs(tt.args)
			if parsed.commandName != tt.wantCmd {
				t.Errorf("commandName: got %q, want %q", parsed.commandName, tt.wantCmd)
			}
			if parsed.configFile != tt.wantConfig {
				t.Errorf("configFile: got %q, want %q", parsed.configFile, tt.wantConfig)
			}
			if parsed.dbPath != tt.wantDB {
				t.Errorf("dbPath: got %q, want %q", parsed.dbPath, tt.wantDB)
			}
			if parsed.dsn != tt.wantDSN {
				t.Errorf("dsn: got %q, want %q", parsed.dsn, tt.wantDSN)
			}
			if parsed.showHelp != tt.wantHelp {
				t.Errorf("showHelp: got %v, want %v", parsed.showHelp, tt.wantHelp)
			}
			if len(parsed.subArgs) != len(tt.wantSubArgs) {
				t.Fatalf("subArgs len: got %d (%v), want %d (%v)", len(parsed.subArgs), parsed.subArgs, len(tt.wantSubArgs), tt.wantSubArgs)
			}
			for i := range parsed.subArgs {
				if parsed.subArgs[i] != tt.wantSubArgs[i] {
					t.Errorf("subArgs[%d]: got %q, want %q", i, parsed.subArgs[i], tt.wantSubArgs[i])
				}
			}
		})
	}
}
