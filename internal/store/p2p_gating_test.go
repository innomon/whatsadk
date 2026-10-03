package store

import (
	"context"
	"testing"
	"time"

	"go-pear/pkg/policy"
	p2pconfig "sqlite-p2p/pkg/config"
	"sqlite-p2p/pkg/p2p"
)

func TestP2P_GatingAndNodeAccessControl(t *testing.T) {
	ctx := context.Background()

	// 1. Create a whitelist policy for Node A
	allowedKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	blockedKeyHex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	allowedKey, err := policy.ParseHexKey(allowedKeyHex)
	if err != nil {
		t.Fatalf("failed to parse allowed key: %v", err)
	}
	blockedKey, err := policy.ParseHexKey(blockedKeyHex)
	if err != nil {
		t.Fatalf("failed to parse blocked key: %v", err)
	}

	pol := policy.New(policy.ModeWhitelist)
	pol.AddWhitelist(allowedKey)

	nodeA, err := NewBackend(Options{
		DBPath: ":memory:",
		Policy: pol,
	})
	if err != nil {
		t.Fatalf("failed to create nodeA: %v", err)
	}
	defer nodeA.Close()

	if nodeA.Policy() == nil {
		t.Fatal("expected non-nil policy from nodeA")
	}
	if nodeA.Policy().Mode() != policy.ModeWhitelist {
		t.Errorf("expected ModeWhitelist, got %v", nodeA.Policy().Mode())
	}
	if !nodeA.Policy().IsAllowed(allowedKey) {
		t.Error("expected allowedKey to be permitted by policy")
	}
	if nodeA.Policy().IsAllowed(blockedKey) {
		t.Error("expected blockedKey to be denied by policy")
	}

	// 2. Test Store-level Policy forwarding
	storeA := &Store{backend: nodeA}
	if storeA.Policy() == nil || storeA.Policy().Mode() != policy.ModeWhitelist {
		t.Errorf("expected Store to forward policy correctly")
	}

	// 3. Dynamic policy mutation
	newPol := policy.New(policy.ModeBlacklist)
	newPol.AddBlacklist(blockedKey)
	storeA.SetPolicy(newPol)

	if storeA.Policy().Mode() != policy.ModeBlacklist {
		t.Errorf("expected ModeBlacklist after dynamic update, got %v", storeA.Policy().Mode())
	}
	if storeA.Policy().IsAllowed(blockedKey) {
		t.Error("expected blockedKey to be blocked under blacklist policy")
	}
	if !storeA.Policy().IsAllowed(allowedKey) {
		t.Error("expected allowedKey to be allowed under blacklist policy")
	}

	// 4. Test OpenP2PFromNodeConfig with in-memory test configuration
	nodeCfg := &p2pconfig.NodeConfig{
		NodeID:    "test-gating-node",
		DBPath:    ":memory:",
		EnableWAL: false,
		Replication: &policy.Config{
			Mode:      "whitelist",
			Whitelist: []string{allowedKeyHex},
		},
	}

	p2pStore, err := OpenP2PFromNodeConfig(nodeCfg)
	if err != nil {
		t.Fatalf("failed to open p2p store from node config: %v", err)
	}
	defer p2pStore.Close()

	if p2pStore.Policy() == nil {
		t.Fatal("expected non-nil policy from p2pStore")
	}
	if p2pStore.Policy().Mode() != policy.ModeWhitelist {
		t.Errorf("expected whitelist mode, got %v", p2pStore.Policy().Mode())
	}

	// Verify standard CRM store operations work over p2pStore
	err = p2pStore.PutFile(ctx, "test/p2p_gated.txt", nil, []byte("gated content"), time.Now().UTC())
	if err != nil {
		t.Fatalf("PutFile failed on gated store: %v", err)
	}

	entry, err := p2pStore.GetFile(ctx, "test/p2p_gated.txt")
	if err != nil || entry == nil {
		t.Fatalf("GetFile failed on gated store: %v", err)
	}
	if string(entry.Content) != "gated content" {
		t.Errorf("expected 'gated content', got %q", string(entry.Content))
	}
}

func TestP2P_GatingReplicationFilterSimulation(t *testing.T) {
	ctx := context.Background()

	// Node A and Node B
	polA := policy.New(policy.ModeWhitelist)
	nodeBKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	nodeBKey, _ := policy.ParseHexKey(nodeBKeyHex)
	polA.AddWhitelist(nodeBKey)

	nodeA, err := NewBackend(Options{DBPath: ":memory:", Policy: polA})
	if err != nil {
		t.Fatalf("failed to create nodeA: %v", err)
	}
	defer nodeA.Close()

	nodeB, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create nodeB: %v", err)
	}
	defer nodeB.Close()

	// Connect with simulated peer key checking
	nodeB.Tracker().Subscribe(func(cs *p2p.Changeset, raw []byte) {
		// Verify policy on Node A allows Node B
		if !nodeA.Policy().IsAllowed(nodeBKey) {
			t.Errorf("nodeA policy unexpectedly rejected nodeB")
			return
		}
		decoded, err := p2p.DecodeChangeset(raw)
		if err != nil {
			t.Errorf("decode error: %v", err)
			return
		}
		_ = p2p.ApplyChangeset(ctx, nodeA.Repo(), decoded)
	})

	// Replicate file from B to A
	err = nodeB.PutFile(ctx, "test/from_b.txt", nil, []byte("from node B"), time.Now().UTC())
	if err != nil {
		t.Fatalf("PutFile failed: %v", err)
	}

	fileOnA, err := nodeA.GetFile(ctx, "test/from_b.txt")
	if err != nil || fileOnA == nil {
		t.Fatalf("expected file to replicate from B to A: %v", err)
	}
	if string(fileOnA.Content) != "from node B" {
		t.Errorf("unexpected content: %q", string(fileOnA.Content))
	}

	// Now deny Node B on Node A
	nodeA.Policy().AddBlacklist(nodeBKey)
	if nodeA.Policy().IsAllowed(nodeBKey) {
		t.Error("expected nodeB to be disallowed after blacklisting")
	}
}
