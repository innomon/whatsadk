# Implementation Plan: TUI SQLite-P2P Swarm Sync & Co-located DB Discovery (`tui-p2p-sync_20261006`)

## Phase 1: Co-located Database Auto-Discovery & Fallback Enhancement
- [ ] Task: Update `cmd/tui/main.go` to scan local `data/` directory for active gateway databases (`p2p_num1.db`, `p2p_num2.db`, `whatsadk_p2p.db`) if primary DSN/path is missing.
- [ ] Task: Ensure co-located TUI opens direct SQLite WAL store when P2P engine lock is present on local host.

## Phase 2: P2P Swarm Connection & Peer Replication Tuning
- [ ] Task: Enhance `store.OpenP2PFromNodeConfig` handling in `cmd/tui/main.go` to ensure peer discovery and `peer_addrs` / `bootstrap` addresses are correctly configured from `config.yaml`.
- [ ] Task: Add automatic background synchronization tick so TUI viewport refreshes when replicated commands complete.

## Phase 3: Status Bar Visibility & Status Refresh
- [ ] Task: Update TUI status bar in `internal/tui/app.go` to display active database file path, P2P sync state, and peer count.

## Phase 4: Verification & Packaging
- [ ] Task: Add unit tests for DB auto-discovery and store initialization in `cmd/tui/main_test.go`.
- [ ] Task: Test with `go test ./...` and rebuild binaries using `make build`.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Verification & Packaging'
