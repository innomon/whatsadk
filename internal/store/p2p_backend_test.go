package store

import (
	"context"
	"testing"

	p2pconfig "sqlite-p2p/pkg/config"
	"sqlite-p2p/pkg/p2p"
)

func TestNewBackend_Memory(t *testing.T) {
	ctx := context.Background()

	opts := Options{
		DBPath: ":memory:",
	}
	backend, err := NewBackend(opts)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	if backend.DB() == nil {
		t.Fatal("expected non-nil DB")
	}
	if backend.Repo() == nil {
		t.Fatal("expected non-nil Repo")
	}
	if backend.Tracker() == nil {
		t.Fatal("expected non-nil Tracker")
	}

	// Verify crm_store table exists
	var tableName string
	err = backend.DB().QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='crm_store'").Scan(&tableName)
	if err != nil {
		t.Fatalf("crm_store table missing: %v", err)
	}

	// Verify views exist
	expectedViews := []string{"filesys", "whatsmeow_contacts", "whatsmeow_commands", "blacklisted_numbers", "whitelisted_numbers"}
	for _, view := range expectedViews {
		var viewName string
		err = backend.DB().QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='view' AND name=?", view).Scan(&viewName)
		if err != nil {
			t.Fatalf("view %s missing: %v", view, err)
		}
		if viewName != view {
			t.Fatalf("expected view %s, got %s", view, viewName)
		}
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("backend.Close failed: %v", err)
	}
}

func TestNewBackend_WithExistingDB(t *testing.T) {
	db, err := p2p.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open store DB: %v", err)
	}
	defer db.Close()

	opts := Options{
		DB: db,
	}
	backend, err := NewBackend(opts)
	if err != nil {
		t.Fatalf("failed to create backend with existing db: %v", err)
	}

	if backend.DB() != db {
		t.Fatalf("expected backend to use provided db")
	}

	// Close backend; since it doesn't own db, it shouldn't error
	if err := backend.Close(); err != nil {
		t.Fatalf("failed to close backend: %v", err)
	}
}

func TestNewBackend_InvalidPath(t *testing.T) {
	opts := Options{
		DBPath: "/nonexistent_dir/impossible/path/db.sqlite",
	}
	// SQLite OpenDB creates parent directories unless it has no permission
	// Using a path under a read-only root or invalid device
	opts.DBPath = "/sys/kernel/debug/invalid/db.sqlite"
	_, err := NewBackend(opts)
	if err == nil {
		// If running as root, this might succeed; test empty DBPath
		opts.DBPath = ""
		_, err = NewBackend(opts)
		if err == nil {
			t.Fatal("expected error for empty DB path without DB")
		}
	}
}

func TestStore_Open_SQLiteP2P(t *testing.T) {
	ctx := context.Background()

	// 1. Verify scheme detection
	if !IsSQLiteP2P("sqlite-p2p://:memory:") {
		t.Fatal("expected IsSQLiteP2P to match sqlite-p2p://")
	}
	if !IsSQLiteP2P("sqlite://mydb.sqlite") {
		t.Fatal("expected IsSQLiteP2P to match sqlite://")
	}
	if !IsSQLiteP2P("p2p://peer.db") {
		t.Fatal("expected IsSQLiteP2P to match p2p://")
	}
	if !IsSQLiteP2P("pear://peer.db") {
		t.Fatal("expected IsSQLiteP2P to match pear://")
	}
	if IsSQLiteP2P("postgres://user:pass@localhost/db") {
		t.Fatal("expected IsSQLiteP2P not to match postgres://")
	}

	// 2. Open Store via SQLite P2P DSN
	st, err := Open("sqlite-p2p://:memory:")
	if err != nil {
		t.Fatalf("Open sqlite-p2p failed: %v", err)
	}
	defer st.Close()

	// 3. Perform basic operations through Store wrapper
	err = st.AddBlacklist(ctx, "+15551234567", "spam")
	if err != nil {
		t.Fatalf("AddBlacklist failed: %v", err)
	}

	blacklisted, err := st.IsBlacklisted(ctx, "+15551234567")
	if err != nil {
		t.Fatalf("IsBlacklisted failed: %v", err)
	}
	if !blacklisted {
		t.Fatal("expected phone to be blacklisted")
	}

	id, err := st.EnqueueCommand(ctx, "send_message", map[string]string{"text": "hello"})
	if err != nil {
		t.Fatalf("EnqueueCommand failed: %v", err)
	}
	if id != 1 {
		t.Fatalf("expected id 1, got %d", id)
	}

	cmds, err := st.PollPendingCommands(ctx)
	if err != nil {
		t.Fatalf("PollPendingCommands failed: %v", err)
	}
	if len(cmds) != 1 || cmds[0].ID != 1 {
		t.Fatalf("expected 1 command with id 1, got %+v", cmds)
	}

	// 4. Test Whitelist operations
	isWhitelisted, err := st.IsWhitelisted(ctx, "+15559876543")
	if err != nil {
		t.Fatalf("IsWhitelisted failed: %v", err)
	}
	if isWhitelisted {
		t.Fatal("expected phone not to be whitelisted initially")
	}

	err = st.AddWhitelist(ctx, "+15559876543", "verified via kb")
	if err != nil {
		t.Fatalf("AddWhitelist failed: %v", err)
	}

	isWhitelisted, err = st.IsWhitelisted(ctx, "+15559876543")
	if err != nil {
		t.Fatalf("IsWhitelisted check failed: %v", err)
	}
	if !isWhitelisted {
		t.Fatal("expected phone to be whitelisted")
	}

	wList, err := st.ListWhitelist(ctx)
	if err != nil {
		t.Fatalf("ListWhitelist failed: %v", err)
	}
	if len(wList) != 1 || wList[0].Phone != "+15559876543" {
		t.Fatalf("unexpected whitelist contents: %+v", wList)
	}

	err = st.RemoveWhitelist(ctx, "+15559876543")
	if err != nil {
		t.Fatalf("RemoveWhitelist failed: %v", err)
	}

	isWhitelisted, err = st.IsWhitelisted(ctx, "+15559876543")
	if err != nil {
		t.Fatalf("IsWhitelisted after removal failed: %v", err)
	}
	if isWhitelisted {
		t.Fatal("expected phone to not be whitelisted after removal")
	}
}

func TestNewBackend_CoexistsWithPhysicalWhatsmeowTables(t *testing.T) {
	ctx := context.Background()

	db, err := p2p.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	// Simulate whatsmeow sqlstore physical tables in the same SQLite DB
	_, err = db.ExecContext(ctx, `
		CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY, registration_id INTEGER);
		CREATE TABLE whatsmeow_contacts (our_jid TEXT, their_jid TEXT, full_name TEXT, short_name TEXT, push_name TEXT, business_name TEXT, PRIMARY KEY (our_jid, their_jid));
	`)
	if err != nil {
		t.Fatalf("failed to create simulated whatsmeow tables: %v", err)
	}

	backend, err := NewBackend(Options{DB: db})
	if err != nil {
		t.Fatalf("NewBackend should succeed when physical whatsmeow_contacts table exists: %v", err)
	}
	defer backend.Close()

	// Verify OpenP2PFromNodeConfig merges DefaultWhatsmeowExcludeTables
	cfg := &p2pconfig.NodeConfig{
		NodeID:        "test-exclude-node",
		DBPath:        ":memory:",
		ExcludeTables: []string{"custom_temp"},
	}
	st, err := OpenP2PFromNodeConfig(cfg)
	if err != nil {
		t.Fatalf("OpenP2PFromNodeConfig failed: %v", err)
	}
	defer st.Close()

	foundDevice := false
	foundCustom := false
	for _, tbl := range cfg.ExcludeTables {
		if tbl == "whatsmeow_device" {
			foundDevice = true
		}
		if tbl == "custom_temp" {
			foundCustom = true
		}
	}
	if !foundDevice || !foundCustom {
		t.Fatalf("expected ExcludeTables to contain both custom_temp and whatsmeow_device, got %+v", cfg.ExcludeTables)
	}
}
