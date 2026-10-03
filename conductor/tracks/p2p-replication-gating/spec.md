# Track Specification: P2P Replication Gating & Node Access Control Configuration (`p2p-replication-gating`)

## 1. Overview
WhatsADK integrates a pure Go, decentralized peer-to-peer storage engine (`sqlite-p2p` powered by `go-pear`). To support granular replication security, this track introduces comprehensive P2P configuration and Replication Gating & Node Access Control directly into WhatsADK's configuration schema (`config.yaml`, environment variables, and `internal/config`).

Nodes configured in WhatsADK can enforce replication access control using `go-pear/pkg/policy` in three distinct modes:
1. `all`: Open discovery and unrestricted peer replication (default).
2. `whitelist`: Permitting only explicitly whitelisted peer public keys (`[32]byte` hex strings).
3. `blacklist`: Permitting all peers except explicitly blacklisted peer public keys (`[32]byte` hex strings).

---

## 2. Configuration Specification

### 2.1 YAML Schema (`config/config.yaml`)
A new top-level `p2p:` block is added to configure the embedded SQLite P2P engine and replication policy:

```yaml
# Optional: SQLite P2P Decentralized Storage & Replication Configuration
# Supports peer-to-peer Autobase replication and cryptographic node access control (go-pear policy).
p2p:
  enabled: false
  node_id: "whatsadk-node-1"
  swarm_topic: "whatsadk-mesh-topic"
  swarm_port: 0
  bootstrap:
    - "192.168.1.100:43219"
  peer_addrs: []
  db_path: "data/whatsadk_p2p.db"
  enable_wal: true
  enable_crypto: false
  auto_sync: true
  replication:
    mode: "all"  # "all" (open), "whitelist" (explicit allow), or "blacklist" (explicit deny)
    whitelist:
      # 64-character hexadecimal Ed25519 public keys
      # - "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    blacklist:
      # 64-character hexadecimal Ed25519 public keys
      # - "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
```

### 2.2 Environment Variable Overrides
The following environment variables override settings in `config.yaml`:
- `P2P_ENABLED`: Boolean (`true` / `false`).
- `P2P_NODE_ID`: Identifier for the local node.
- `P2P_SWARM_TOPIC`: Discovery topic name for the swarm mesh.
- `P2P_SWARM_PORT`: Port for P2P swarm listener (0 for ephemeral).
- `P2P_DB_PATH`: Filesystem path for SQLite P2P database.
- `P2P_ENABLE_WAL`: Enable SQLite Write-Ahead Logging.
- `P2P_ENABLE_CRYPTO`: Enable cryptographic key registry encryption.
- `P2P_AUTO_SYNC`: Enable automated P2P replication synchronization.
- `P2P_REPLICATION_MODE`: Policy mode (`all`, `whitelist`, `blacklist`).

---

## 3. Go Configuration Structures (`internal/config/config.go`)

### 3.1 Struct Definitions
```go
type P2PConfig struct {
    Enabled      bool              `yaml:"enabled" json:"enabled"`
    NodeID       string            `yaml:"node_id" json:"node_id"`
    SwarmTopic   string            `yaml:"swarm_topic" json:"swarm_topic"`
    SwarmPort    int               `yaml:"swarm_port" json:"swarm_port"`
    Bootstrap    []string          `yaml:"bootstrap" json:"bootstrap"`
    PeerAddrs    []string          `yaml:"peer_addrs" json:"peer_addrs"`
    DBPath       string            `yaml:"db_path" json:"db_path"`
    EnableWAL    bool              `yaml:"enable_wal" json:"enable_wal"`
    EnableCrypto bool              `yaml:"enable_crypto" json:"enable_crypto"`
    AutoSync     bool              `yaml:"auto_sync" json:"auto_sync"`
    Replication  ReplicationConfig `yaml:"replication" json:"replication"`
}

type ReplicationConfig struct {
    Mode      string   `yaml:"mode" json:"mode"`
    Whitelist []string `yaml:"whitelist,omitempty" json:"whitelist,omitempty"`
    Blacklist []string `yaml:"blacklist,omitempty" json:"blacklist,omitempty"`
}
```

### 3.2 Conversion & Policy Builders
- `(p *P2PConfig) ToNodeConfig() *config.NodeConfig`: Maps `P2PConfig` to `sqlite-p2p/pkg/config.NodeConfig`.
- `(p *P2PConfig) BuildPolicy() (*policy.ReplicationPolicy, error)`: Constructs an active `*policy.ReplicationPolicy` validated against provided hex keys.
- `(r *ReplicationConfig) ToPolicyConfig() *policy.Config`: Maps `ReplicationConfig` to `go-pear/pkg/policy.Config`.

---

## 4. Storage Backend & Gating Integration (`internal/store/p2p_backend.go`)
- Support passing `*policy.ReplicationPolicy` and `*p2p.Engine` directly through `store.Options`.
- Expose `Policy() *policy.ReplicationPolicy` and `SetPolicy(p *policy.ReplicationPolicy)` on `Backend`.
- Expose `OpenP2P(cfg config.P2PConfig)` and `OpenWithNodeConfig(nc *p2pconfig.NodeConfig)` to open and initialize store backends with full replication gating policies.

---

## 5. Verification & Testing
- Table-driven unit tests in `internal/config/config_test.go` covering:
  - Configuration deserialization from YAML and JSON.
  - Default values and environment variable overrides.
  - `BuildPolicy` validation for `all`, `whitelist`, and `blacklist` modes.
  - Invalid public key error handling.
- Table-driven tests in `internal/store/p2p_gating_test.go` verifying policy enforcement across simulated P2P nodes.
