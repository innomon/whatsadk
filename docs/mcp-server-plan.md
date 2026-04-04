# Implementation Plan: WhatsADK MCP Server

The goal is to create a standalone binary `cmd/mcp` that acts as a Model Context Protocol (MCP) server.

## Phase 1: Database Logic Expansion

Expand `internal/store/store.go` to include methods for querying `whatsmeow_contacts`.
- [ ] Add `ListContacts(ctx, query string) ([]Contact, error)`
- [ ] Add `GetFilesysLogs(ctx, phone string, limit int) ([]FileEntry, error)`
- [ ] Ensure all models have proper `json` tags for MCP output.

## Phase 2: MCP Server Development (`cmd/mcp`)

- [ ] **Dependency Setup**:
  - `go get github.com/modelcontextprotocol/go-sdk/mcp`
- [ ] **Core Initialization**:
  - Initialize the MCP server using the official SDK.
  - Standard `stdio` transport.
  - Connect to PostgreSQL via `internal/config` (shared DSN).
- [ ] **Implement Tool Handlers**:
  - `blacklist_add` / `blacklist_remove` (maps to existing store methods).
  - `query_contacts` (maps to new store methods).
  - `get_message_logs` (maps to new store methods).
- [ ] **Implement Resource Handlers**:
  - `mcp://whatsadk/blacklist`
  - `mcp://whatsadk/contacts/summary`

## Phase 3: Integration & Testing

- [ ] **Manual CLI Test**:
  - Use `mcp-inspector` or `claude-code --mcp-server ./bin/whatsadk-mcp` to verify connectivity.
- [ ] **Documentation Update**:
  - Add "MCP Usage" section to `README.md`.
  - Provide configuration examples for `claude-code` (`claude_desktop_config.json`).
- [ ] **Verification**:
  - Verify that `claudcode` can successfully list blacklisted users and query WhatsApp contacts.

## Phase 4: Build & Makefile

- [ ] Update `Makefile` to include the `mcp` target:
  ```makefile
  build-mcp:
      go build -o bin/whatsadk-mcp cmd/mcp/main.go
  ```
