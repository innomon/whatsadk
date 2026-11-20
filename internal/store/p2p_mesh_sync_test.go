package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	p2pconfig "sqlite-p2p/pkg/config"
)

func TestP2P_MeshLiveSyncAndSeeding(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "p2p_mesh_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPathA := filepath.Join(tmpDir, "nodeA.db")
	dbPathB := filepath.Join(tmpDir, "nodeB.db")

	ctx := context.Background()

	// 1. Pre-seed DB A with existing records directly into SQLite
	preStore, err := Open("sqlite-p2p://" + dbPathA)
	if err != nil {
		t.Fatalf("failed to open preStore: %v", err)
	}
	for i := 1; i <= 10; i++ {
		_ = preStore.PutFile(ctx, "history/msg"+string(rune('0'+i)), map[string]interface{}{"mime_type": "text/plain"}, []byte("historical message content"), time.Now().UTC())
	}
	preStore.Close()

	// 2. Start Node A with Swarm
	topic := "whatsadk-test-topic-" + time.Now().Format("150405")
	cfgA := &p2pconfig.NodeConfig{
		NodeID:     "node-a",
		SwarmTopic: topic,
		SwarmPort:  0,
		DBPath:     dbPathA,
		EnableWAL:  true,
		AutoSync:   true,
	}

	storeA, err := OpenP2PFromNodeConfig(cfgA)
	if err != nil {
		t.Fatalf("failed to open storeA: %v", err)
	}
	defer storeA.Close()

	// 3. Start Node B with Swarm and Node A's DHT as bootstrap
	dhtAddrA := ""
	if backendA, ok := storeA.backend.(*Backend); ok && backendA.Engine() != nil && backendA.Engine().Swarm() != nil {
		dhtAddrA = backendA.Engine().Swarm().DHTAddr()
	}

	var bootstrap []string
	if dhtAddrA != "" {
		bootstrap = append(bootstrap, dhtAddrA)
	}

	cfgB := &p2pconfig.NodeConfig{
		NodeID:     "node-b",
		SwarmTopic: topic,
		SwarmPort:  0,
		Bootstrap:  bootstrap,
		DBPath:     dbPathB,
		EnableWAL:  true,
		AutoSync:   true,
	}

	storeB, err := OpenP2PFromNodeConfig(cfgB)
	if err != nil {
		t.Fatalf("failed to open storeB: %v", err)
	}
	defer storeB.Close()

	// 4. Wait for P2P connection and sync
	synced := false
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		filesB, err := storeB.GetAllFiles(ctx)
		if err == nil && len(filesB) == 10 {
			synced = true
			break
		}
	}

	if !synced {
		filesB, _ := storeB.GetAllFiles(ctx)
		t.Fatalf("expected 10 records synced to Node B, got %d", len(filesB))
	}

	// 5. Test Live Mutation Node B -> Node A
	err = storeB.PutFile(ctx, "live/from_b.txt", nil, []byte("live sync from B"), time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to PutFile on storeB: %v", err)
	}

	liveSynced := false
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		f, err := storeA.GetFile(ctx, "live/from_b.txt")
		if err == nil && f != nil && string(f.Content) == "live sync from B" {
			liveSynced = true
			break
		}
	}

	if !liveSynced {
		t.Fatal("expected live mutation on Node B to sync to Node A")
	}

	// 6. Test Co-Located Direct WAL Fallback (simulating MCP / TUI writing directly to Node A's SQLite DB)
	colocatedStore, err := Open("sqlite-p2p://" + dbPathA)
	if err != nil {
		t.Fatalf("failed to open colocated direct WAL store on dbPathA: %v", err)
	}
	defer colocatedStore.Close()

	if err := colocatedStore.PutFile(ctx, "colocated/mcp_note.txt", nil, []byte("written via direct WAL fallback"), time.Now().UTC()); err != nil {
		t.Fatalf("colocatedStore.PutFile failed: %v", err)
	}
	cmdID, err := colocatedStore.EnqueueCommand(ctx, "send_message", map[string]string{"jid": "123@s.whatsapp.net", "text": "from colocated MCP"})
	if err != nil {
		t.Fatalf("colocatedStore.EnqueueCommand failed: %v", err)
	}

	// Trigger or wait for CDC poller on storeA to drain _p2p_cdc_outbox and broadcast to storeB
	_, _ = storeA.DrainCDCOutbox(ctx)

	colocatedSynced := false
	for i := 0; i < 30; i++ {
		time.Sleep(150 * time.Millisecond)
		fB, err1 := storeB.GetFile(ctx, "colocated/mcp_note.txt")
		cmdsB, err2 := storeB.GetAllCommands(ctx)
		if err1 == nil && fB != nil && string(fB.Content) == "written via direct WAL fallback" && err2 == nil && len(cmdsB) == 1 && cmdsB[0].ID == cmdID {
			colocatedSynced = true
			break
		}
	}
	if !colocatedSynced {
		t.Fatal("expected co-located direct WAL fallback mutations on Node A to replicate to Node B via SQLite CDC outbox triggers")
	}

	// 7. Test Multi-Table Custom Relational SQL Sync & whatsmeow Protocol Table Exclusion
	backendA := storeA.backend.(*Backend)
	_, err = backendA.DB().ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS whatsmeow_device (jid TEXT PRIMARY KEY, registration_id INTEGER);
		INSERT INTO whatsmeow_device (jid, registration_id) VALUES ('secret-device@s.whatsapp.net', 99999);
		CREATE TABLE IF NOT EXISTS custom_orders (id TEXT PRIMARY KEY, status TEXT);
		INSERT INTO custom_orders (id, status) VALUES ('ORD-777', 'SHIPPED');
	`)
	if err != nil {
		t.Fatalf("failed to create custom_orders and whatsmeow_device on Node A: %v", err)
	}
	_, _ = storeA.DrainCDCOutbox(ctx)

	backendB := storeB.backend.(*Backend)
	customOrderSynced := false
	for i := 0; i < 30; i++ {
		time.Sleep(150 * time.Millisecond)
		var status string
		err := backendB.DB().QueryRowContext(ctx, `SELECT status FROM custom_orders WHERE id = 'ORD-777'`).Scan(&status)
		if err == nil && status == "SHIPPED" {
			customOrderSynced = true
			break
		}
	}
	if !customOrderSynced {
		t.Fatal("expected custom_orders table and row to replicate from Node A to Node B via multi-table CDC")
	}

	// Verify whatsmeow_device was excluded and NOT replicated to Node B
	var wmTableCount int
	err = backendB.DB().QueryRowContext(ctx, `SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = 'whatsmeow_device'`).Scan(&wmTableCount)
	if err != nil {
		t.Fatalf("failed to query sqlite_master on Node B: %v", err)
	}
	if wmTableCount != 0 {
		t.Fatal("SECURITY VIOLATION: whatsmeow_device protocol table must never be replicated to Node B")
	}
}
