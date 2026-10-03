package config

import (
	"os"
	"strings"
	"testing"

	"go-pear/pkg/policy"
	"gopkg.in/yaml.v3"
)

func TestP2PConfig_Defaults(t *testing.T) {
	cfg := defaultConfig()
	if cfg.P2P.Replication.Mode != "all" {
		t.Errorf("expected default replication mode 'all', got %q", cfg.P2P.Replication.Mode)
	}

	pol, err := cfg.P2P.BuildPolicy()
	if err != nil {
		t.Fatalf("unexpected error building policy: %v", err)
	}
	if pol.Mode() != policy.ModeAllAllowed {
		t.Errorf("expected ModeAllAllowed, got %v", pol.Mode())
	}
}

func TestP2PConfig_YAMLUnmarshal_Whitelist(t *testing.T) {
	rawYAML := `
p2p:
  enabled: true
  node_id: "node-alpha"
  swarm_topic: "test-topic"
  swarm_port: 43219
  bootstrap:
    - "127.0.0.1:43219"
  peer_addrs:
    - "127.0.0.1:43220"
  db_path: "data/node_alpha.db"
  enable_wal: true
  enable_crypto: true
  auto_sync: true
  replication:
    mode: "whitelist"
    whitelist:
      - "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
`

	var cfg Config
	err := yaml.Unmarshal([]byte(rawYAML), &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}

	if !cfg.P2P.Enabled {
		t.Errorf("expected P2P.Enabled to be true")
	}
	if cfg.P2P.NodeID != "node-alpha" {
		t.Errorf("expected NodeID 'node-alpha', got %q", cfg.P2P.NodeID)
	}
	if cfg.P2P.SwarmPort != 43219 {
		t.Errorf("expected SwarmPort 43219, got %d", cfg.P2P.SwarmPort)
	}
	if cfg.P2P.Replication.Mode != "whitelist" {
		t.Errorf("expected replication mode 'whitelist', got %q", cfg.P2P.Replication.Mode)
	}
	if len(cfg.P2P.Replication.Whitelist) != 1 {
		t.Fatalf("expected 1 whitelisted key, got %d", len(cfg.P2P.Replication.Whitelist))
	}

	// Verify BuildPolicy
	pol, err := cfg.P2P.BuildPolicy()
	if err != nil {
		t.Fatalf("failed to build policy: %v", err)
	}
	if pol.Mode() != policy.ModeWhitelist {
		t.Errorf("expected ModeWhitelist, got %v", pol.Mode())
	}

	allowedKey, _ := policy.ParseHexKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if !pol.IsAllowed(allowedKey) {
		t.Errorf("expected key to be allowed by whitelist policy")
	}

	randomKey, _ := policy.ParseHexKey("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
	if pol.IsAllowed(randomKey) {
		t.Errorf("expected non-whitelisted key to be rejected")
	}

	// Verify ToNodeConfig
	nc := cfg.P2P.ToNodeConfig()
	if nc.NodeID != "node-alpha" || nc.Replication.Mode != "whitelist" {
		t.Errorf("unexpected NodeConfig: %+v", nc)
	}
}

func TestP2PConfig_YAMLUnmarshal_Blacklist(t *testing.T) {
	rawYAML := `
p2p:
  enabled: true
  replication:
    mode: "blacklist"
    blacklist:
      - "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
`

	var cfg Config
	err := yaml.Unmarshal([]byte(rawYAML), &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}

	pol, err := cfg.P2P.BuildPolicy()
	if err != nil {
		t.Fatalf("failed to build policy: %v", err)
	}
	if pol.Mode() != policy.ModeBlacklist {
		t.Errorf("expected ModeBlacklist, got %v", pol.Mode())
	}

	blockedKey, _ := policy.ParseHexKey("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
	if pol.IsAllowed(blockedKey) {
		t.Errorf("expected blacklisted key to be rejected")
	}

	otherKey, _ := policy.ParseHexKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if !pol.IsAllowed(otherKey) {
		t.Errorf("expected non-blacklisted key to be allowed")
	}
}

func TestP2PConfig_InvalidKeyError(t *testing.T) {
	p2pCfg := P2PConfig{
		Replication: ReplicationConfig{
			Mode:      "whitelist",
			Whitelist: []string{"invalid-key"},
		},
	}

	_, err := p2pCfg.BuildPolicy()
	if err == nil {
		t.Fatal("expected error for invalid hex key, got nil")
	}
	if !strings.Contains(err.Error(), "whitelist error") {
		t.Errorf("expected whitelist error message, got %v", err)
	}
}

func TestP2PConfig_EnvOverrides(t *testing.T) {
	os.Setenv("P2P_ENABLED", "true")
	os.Setenv("P2P_NODE_ID", "env-node-1")
	os.Setenv("P2P_SWARM_TOPIC", "env-mesh")
	os.Setenv("P2P_SWARM_PORT", "9999")
	os.Setenv("P2P_DB_PATH", "/tmp/env_p2p.db")
	os.Setenv("P2P_ENABLE_WAL", "false")
	os.Setenv("P2P_ENABLE_CRYPTO", "true")
	os.Setenv("P2P_AUTO_SYNC", "false")
	os.Setenv("P2P_REPLICATION_MODE", "whitelist")
	defer func() {
		os.Unsetenv("P2P_ENABLED")
		os.Unsetenv("P2P_NODE_ID")
		os.Unsetenv("P2P_SWARM_TOPIC")
		os.Unsetenv("P2P_SWARM_PORT")
		os.Unsetenv("P2P_DB_PATH")
		os.Unsetenv("P2P_ENABLE_WAL")
		os.Unsetenv("P2P_ENABLE_CRYPTO")
		os.Unsetenv("P2P_AUTO_SYNC")
		os.Unsetenv("P2P_REPLICATION_MODE")
	}()

	cfg := defaultConfig()
	if !cfg.P2P.Enabled {
		t.Error("expected P2P.Enabled = true from env")
	}
	if cfg.P2P.NodeID != "env-node-1" {
		t.Errorf("expected NodeID 'env-node-1', got %q", cfg.P2P.NodeID)
	}
	if cfg.P2P.SwarmTopic != "env-mesh" {
		t.Errorf("expected SwarmTopic 'env-mesh', got %q", cfg.P2P.SwarmTopic)
	}
	if cfg.P2P.SwarmPort != 9999 {
		t.Errorf("expected SwarmPort 9999, got %d", cfg.P2P.SwarmPort)
	}
	if cfg.P2P.DBPath != "/tmp/env_p2p.db" {
		t.Errorf("expected DBPath '/tmp/env_p2p.db', got %q", cfg.P2P.DBPath)
	}
	if cfg.P2P.EnableWAL {
		t.Error("expected EnableWAL = false from env")
	}
	if !cfg.P2P.EnableCrypto {
		t.Error("expected EnableCrypto = true from env")
	}
	if cfg.P2P.AutoSync {
		t.Error("expected AutoSync = false from env")
	}
	if cfg.P2P.Replication.Mode != "whitelist" {
		t.Errorf("expected Replication.Mode 'whitelist', got %q", cfg.P2P.Replication.Mode)
	}
}
