# Implementation Plan: P2P Replication Gating & Node Access Control Configuration (`p2p-replication-gating`)

## Phase 1: Configuration Schema & Struct Definitions (`internal/config`)
- [x] Define `P2PConfig` and `ReplicationConfig` structs in `internal/config/config.go`.
- [x] Add `P2P P2PConfig` field to root `Config` struct.
- [x] Implement conversion helper methods:
  - `(p *P2PConfig) ToNodeConfig() *p2pconfig.NodeConfig`
  - `(p *P2PConfig) BuildPolicy() (*policy.ReplicationPolicy, error)`
  - `(r *ReplicationConfig) ToPolicyConfig() *policy.Config`

## Phase 2: Defaults, Environment Overrides & Unit Tests
- [x] Implement default values in `(c *Config) applyDefaults()`:
  - `P2P.Replication.Mode = "all"` if not set.
  - `P2P.DBPath = "data/whatsadk_p2p.db"` if not set.
  - Default `EnableWAL = true` and `AutoSync = true`.
- [x] Implement environment variable overrides in `(c *Config) applyEnvOverrides()`:
  - `P2P_ENABLED`, `P2P_NODE_ID`, `P2P_SWARM_TOPIC`, `P2P_SWARM_PORT`, `P2P_DB_PATH`, `P2P_ENABLE_WAL`, `P2P_ENABLE_CRYPTO`, `P2P_AUTO_SYNC`, `P2P_REPLICATION_MODE`.
- [x] Add comprehensive table-driven tests in `internal/config/config_test.go`.

## Phase 3: Store & Backend Gating Integration (`internal/store`)
- [x] Extend `store.Options` to include `Policy *policy.ReplicationPolicy` and `P2PEngine *p2p.Engine`.
- [x] Add `Policy()` and `SetPolicy(p *policy.ReplicationPolicy)` methods to `store.Backend` and `store.Store`.
- [x] Provide `OpenP2P(cfg config.P2PConfig)` and `OpenWithNodeConfig(nc *p2pconfig.NodeConfig)` helper constructors in `internal/store/p2p_backend.go`.
- [x] Add integration/unit test in `internal/store/p2p_gating_test.go`.

## Phase 4: Configuration Files & Documentation Updates
- [x] Update `config/config.yaml` with the complete documented `p2p:` section.
- [x] Update `docs/sqlite-p2p-storage.md` with gating documentation, schema, and usage.
- [x] Update `README.md` and `ARCHITECTURE.md` to document the P2P replication gating configuration.
- [x] Register track in `conductor/tracks.md`.

## Phase 5: Verification & Quality Assurance
- [x] Run `go test ./...` across all packages.
- [x] Run `go build ./...` to ensure no build regressions.
