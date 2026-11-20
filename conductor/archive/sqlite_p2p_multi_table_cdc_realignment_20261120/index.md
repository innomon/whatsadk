# Track: SQLite-P2P Multi-Table CDC & Protocol Table Isolation Realignment

- **Track ID**: `sqlite_p2p_multi_table_cdc_realignment_20261120`
- **Specification**: [spec.md](spec.md)
- **Plan**: [plan.md](plan.md)
- **Metadata**: [metadata.json](metadata.json)

## Scope
1. Realign `P2PConfig` and `ToNodeConfig()` in `/internal/config/config.go` with `sqlite-p2p` multi-table replication settings (`default_table`, `include_tables`, `exclude_tables`, and environment variable overrides).
2. Enforce automatic protocol-table exclusion (`whatsmeow_*` E2EE session/identity/pre-key tables) in `/internal/store/p2p_backend.go` so `sqlite-p2p` schema auto-discovery never attaches CDC triggers or replicates private Signal keys if a database file is ever shared.
3. Expose `DrainCDCOutbox(ctx)` on `/internal/store/store.go` and `/internal/store/p2p_backend.go`, and wire immediate CDC outbox draining into TUI/MCP custom SQL execution (`/internal/tui/handlers/handlers.go`).
4. Add multi-table CDC and co-located direct WAL fallback replication tests in `/internal/store/p2p_mesh_sync_test.go`.
5. Update `/README.md`, `/ARCHITECTURE.md`, and `/docs/sqlite-p2p-storage.md` to document multi-table CDC configuration, protocol table isolation, and co-located WAL trigger synchronization.
