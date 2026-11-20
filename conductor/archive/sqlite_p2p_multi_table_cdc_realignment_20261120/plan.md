# Implementation Plan: SQLite-P2P Multi-Table CDC & Protocol Table Isolation Realignment

## Phase 1: Multi-Table P2P Configuration & Environment Overrides (`/internal/config`)
- [x] Task 1.1: Write unit tests in `/internal/config/config_test.go` for `DefaultTable`, `IncludeTables`, and `ExcludeTables` YAML unmarshaling, `P2P_DEFAULT_TABLE` / `P2P_INCLUDE_TABLES` / `P2P_EXCLUDE_TABLES` env overrides, and `ToNodeConfig()` mapping.
- [x] Task 1.2: Update `P2PConfig`, `ToNodeConfig()`, and `applyEnvOverrides()` in `/internal/config/config.go` to support `DefaultTable`, `IncludeTables`, and `ExcludeTables`.

## Phase 2: `whatsmeow` Protocol Table Isolation & CDC Outbox Integration (`/internal/store`)
- [x] Task 2.1: Write unit and mesh integration tests in `/internal/store/p2p_backend_test.go` and `/internal/store/p2p_mesh_sync_test.go` verifying:
  - Automatic exclusion of `whatsmeow_*` session/signal tables from P2P replication.
  - Safe coexistence when `whatsmeow_contacts` is already a physical table in SQLite.
  - Live P2P replication of mutations written by a co-located direct WAL fallback store (`store.Open(dbPath)`) via `_p2p_cdc_outbox` triggers.
  - Live P2P replication of custom relational SQL tables and `Store.DrainCDCOutbox(ctx)`.
- [x] Task 2.2: Implement `DefaultWhatsmeowExcludeTables` merging in `OpenP2PFromNodeConfig`, view creation conflict guard in `NewBackend`, and `DrainCDCOutbox(ctx)` in `/internal/store/p2p_backend.go`, `/internal/store/p2p_query.go`, and `/internal/store/store.go`.

## Phase 3: Documentation & Full Verification
- [x] Task 3.1: Update `/docs/sqlite-p2p-storage.md`, `/README.md`, `/ARCHITECTURE.md`, and `/config/config.yaml` with multi-table CDC options, `whatsmeow` table isolation guardrails, and co-located WAL trigger sync behavior.
- [x] Task 3.2: Run `go test -v ./...` and `go build ./...` to verify all tests and binaries pass cleanly.
- [x] Task 3.3: Conductor - User Manual Verification & Checkpoint.
