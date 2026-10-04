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
}
