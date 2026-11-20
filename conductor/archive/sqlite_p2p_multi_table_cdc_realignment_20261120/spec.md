# Specification: SQLite-P2P Multi-Table CDC & Protocol Table Isolation Realignment

## 1. Overview
`sqlite-p2p` has been upgraded from a single-table (`crm_store`), API-only replicator into a **universal multi-table SQLite replication engine** with automatic schema discovery (`sqlite_master` + `PRAGMA table_info`), SQLite trigger-based Change Data Capture (`_p2p_cdc_outbox` and `_p2p_sync_state`), `OpDDL` table creation events, and table-scoped Last-Write-Wins (LWW) conflict resolution.

While WhatsADK's existing `crm_store` + SQLite `VIEW` projection layer in `/internal/store/p2p_backend.go` remains backward-compatible, WhatsADK requires targeted realignments to:
1. Expose the new multi-table configuration options (`default_table`, `include_tables`, `exclude_tables`) in `config.yaml` and environment variables.
2. Prevent `sqlite-p2p`'s table auto-discovery from ever attaching CDC triggers to or replicating `whatsmeow` E2EE Signal protocol tables (`whatsmeow_device`, `whatsmeow_identity_keys`, `whatsmeow_pre_keys`, `whatsmeow_sessions`, etc.) if a user points `whatsapp.store_dsn` and `p2p.db_path` to the same SQLite database file.
3. Leverage trigger-based CDC outbox draining (`DrainCDCOutbox`) for immediate replication after custom SQL execution and verify live swarm replication when co-located processes (`whatsadk-mcp`, `tui`, `dbutil`) write via direct SQLite WAL fallback mode.

---

## 2. Functional Requirements

### 2.1 Multi-Table Configuration in `P2PConfig` (`/internal/config/config.go`)
- Extend `P2PConfig` with:
  - `DefaultTable string` (`yaml:"default_table,omitempty" json:"default_table,omitempty"`)
  - `IncludeTables []string` (`yaml:"include_tables,omitempty" json:"include_tables,omitempty"`)
  - `ExcludeTables []string` (`yaml:"exclude_tables,omitempty" json:"exclude_tables,omitempty"`)
- Support environment variable overrides:
  - `P2P_DEFAULT_TABLE`: Sets `P2P.DefaultTable`.
  - `P2P_INCLUDE_TABLES`: Comma-separated list of table names to include in P2P CDC replication.
  - `P2P_EXCLUDE_TABLES`: Comma-separated list of table names to exclude from P2P CDC replication.
- Forward `DefaultTable`, `IncludeTables`, and `ExcludeTables` in `P2PConfig.ToNodeConfig()`.

### 2.2 Mandatory `whatsmeow` Protocol Table Exclusion Guardrail (`/internal/store/p2p_backend.go`)
- Define the canonical list of `whatsmeow` local E2EE protocol and session tables that must **never** be replicated across a P2P swarm:
  - `whatsmeow_device`
  - `whatsmeow_identity_keys`
  - `whatsmeow_pre_keys`
  - `whatsmeow_sessions`
  - `whatsmeow_sender_keys`
  - `whatsmeow_app_state_sync_keys`
  - `whatsmeow_app_state_version`
  - `whatsmeow_app_state_mutation_macs`
  - `whatsmeow_contacts` (when created as a physical table by `whatsmeow/store/sqlstore` rather than a WhatsADK view)
  - `whatsmeow_chat_settings`
  - `whatsmeow_message_secrets`
  - `whatsmeow_privacy_tokens`
  - `whatsmeow_lid_map`
  - `whatsmeow_event_buffer`
  - `whatsmeow_version`
- In `OpenP2PFromNodeConfig(cfg *p2pconfig.NodeConfig)`, automatically merge these `whatsmeow` protocol tables into `cfg.ExcludeTables` (deduplicated) before calling `p2p.OpenEngineFromNodeConfig(cfg)`.
- Ensure that if `whatsmeow_contacts` already exists as a physical table in a shared SQLite file, `NewBackend` does not fail when executing `CREATE VIEW IF NOT EXISTS whatsmeow_contacts` (inspect whether a physical table with that name exists or guard view creation cleanly).

### 2.3 On-Demand CDC Outbox Draining & Custom SQL Support (`/internal/store/` & `/internal/tui/`)
- Add `DrainCDCOutbox(ctx context.Context) (int, error)` to `Backend` (`/internal/store/p2p_backend.go`) and `Store` (`/internal/store/store.go`), delegating to `b.p2pEngine.DrainCDCOutbox(ctx)` or `b.engine.DrainCDCOutbox(ctx)` when active (and returning `0, nil` on non-P2P backends).
- In `Backend.QueryFilesys(ctx, query, args...)` (`/internal/store/p2p_query.go`), if the P2P engine is active on the backend, trigger a non-blocking or immediate `DrainCDCOutbox(ctx)` after query execution so any custom DDL/DML executed via `sql` in TUI is immediately discovered and broadcasted to peers.

### 2.4 Co-Located Direct WAL Fallback & Multi-Table CDC Verification (`/internal/store/p2p_mesh_sync_test.go`)
- Add integration tests verifying:
  1. **Co-located WAL Fallback Replication**: When Node A runs an active P2P Swarm engine on `nodeA.db` and a co-located process (simulating `whatsadk-mcp` or `tui` in direct WAL fallback mode via `store.Open(dbPathA)`) enqueues commands or writes files directly to `nodeA.db`, the SQLite CDC triggers populate `_p2p_cdc_outbox` on `nodeA.db` and Node A's `ReplicationEngine` automatically drains and replicates those records to Node B over Hyperswarm.
  2. **Multi-Table Custom Relational Table Replication**: Creating and mutating a custom SQL table on Node A replicates `OpDDL` and row mutations to Node B.
  3. **`whatsmeow` Protocol Table Exclusion**: Verify that `whatsmeow_device` and `whatsmeow_identity_keys` tables created in Node A's database are excluded from CDC trigger installation and are never replicated to Node B.

---

## 3. Acceptance Criteria
- [x] `P2PConfig` parses `default_table`, `include_tables`, and `exclude_tables` from YAML and `P2P_DEFAULT_TABLE`, `P2P_INCLUDE_TABLES`, `P2P_EXCLUDE_TABLES` environment variables, and maps them in `ToNodeConfig()`.
- [x] `OpenP2PFromNodeConfig` automatically excludes all `whatsmeow_*` protocol tables from P2P replication.
- [x] Co-located direct SQLite WAL writes (`store.Open(dbPath)`) to a database managed by an active P2P Gateway node are captured by `_p2p_cdc_outbox` triggers and replicated to remote swarm peers.
- [x] All unit and integration tests (`go test ./...`) and builds (`make build`) pass with zero regressions.
- [x] `/README.md`, `/ARCHITECTURE.md`, and `/docs/sqlite-p2p-storage.md` are updated to reflect the realigned architecture.
