# Implementation Plan: Interactive Command TUI & MCP UI (`tui-mcp-cmd`)

## Phase 1: Handcrafted Command Registry & Config Resolver
- [ ] Task: Create handcrafted command registry and configuration loader in `internal/tui/registry/` and `internal/tui/config/`.
- [ ] Task: Implement parameter parser supporting `string`, `int`, `bool` types and dynamic prompt resolution for missing values.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Handcrafted Command Registry & Config Resolver'

## Phase 2: MCP Tools & SQL Handlers Binding
- [ ] Task: Bind all MCP tools (`send_message`, `query_contacts`, `jid_to_phone`, `list_groups`, `list_group_members`, `get_recent_messages`, `filesys_sql_select`, `blacklist_add`, `blacklist_remove`, etc.) to command registry handlers.
- [ ] Task: Implement `sql` command handler for direct query execution against SQLite-P2P store.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: MCP Tools & SQL Handlers Binding'

## Phase 3: Charm Bubble Tea TUI & Multi-Format Render Engine
- [ ] Task: Build Bubble Tea TUI application shell in `cmd/tui/main.go` and `internal/tui/app.go`.
- [ ] Task: Implement direct terminal command prompt (first word = command, rest = params).
- [ ] Task: Implement Slash `/` modal popup with parameter input fields and help display.
- [ ] Task: Implement multi-format output viewport rendering: Plain Text, Markdown (Lip Gloss / Glamour), and A2UI structured components.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Charm Bubble Tea TUI & Multi-Format Render Engine'

## Phase 4: SQLite-P2P Swarm Connection & Integration
- [ ] Task: Wire `sqlite-p2p` swarm initialization into TUI startup sequence using `config.yaml`.
- [ ] Task: Add live P2P mesh status bar indicator and background replication handler.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: SQLite-P2P Swarm Connection & Integration'

## Phase 5: Build, Test Verification & Cross-Platform Packaging
- [ ] Task: Add unit tests for registry, parser, handlers, and TUI components (`go test ./...`).
- [ ] Task: Update `Makefile` and `scripts/build-cross.sh` to include `tui` binary.
- [ ] Task: Update `README.md` and `cmd/tui/README.md`.
- [ ] Task: Conductor - User Manual Verification 'Phase 5: Build, Test Verification & Cross-Platform Packaging'
