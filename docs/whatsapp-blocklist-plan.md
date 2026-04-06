# Implementation Plan: Unified WhatsApp Blocklist Management

This plan outlines the steps to unify local "Shadow Ban" and remote WhatsApp-level blocking.

## Phase 1: Data Layer (Shared Store)

### 1.1 Store Migrations (`internal/store/store.go`)
- [ ] Add `whatsmeow_contacts` table to `migrate()`.
- [ ] Add `whatsmeow_commands` table for IPC.

### 1.2 Store Methods
- [ ] **Command Queueing**:
    - `EnqueueCommand(ctx, cmd string, payload interface{}) (int64, error)`
    - `UpdateCommandStatus(ctx, id int64, status string, result interface{}) error`
    - `WaitForCommand(ctx, id int64, timeout time.Duration) (string, error)`
- [ ] **Polling**:
    - `PollPendingCommands(ctx) ([]Command, error)`

## Phase 2: Gateway Execution Layer

### 2.1 WhatsApp Client Enhancements (`internal/whatsapp/client.go`)
- [ ] Implement `processCommands(ctx)` background loop.
- [ ] Handle `block` / `unblock` / `get_blocklist` commands.
- [ ] **Action Methods**:
    - `RemoteBlock(jid string) error`
    - `RemoteUnblock(jid string) error`
    - `RemoteGetBlocklist() ([]string, error)`

### 2.2 IPC Integration
- [ ] Start the command processor goroutine in `Run()`.

## Phase 3: MCP User Layer (`cmd/mcp/main.go`)

### 3.1 Handler Updates
- [ ] **`blacklist_add`**: Update to call `s.AddBlacklist` **and** `s.EnqueueCommand`. Wait for remote confirmation.
- [ ] **`blacklist_remove`**: Update to call `s.RemoveBlacklist` **and** `s.EnqueueCommand`. Wait for remote confirmation.
- [ ] **`blacklist_get_remote`**: New tool handler that enqueues `get_blocklist` and returns the result.

## Phase 4: Verification

### 4.1 Functional Tests
- [ ] Call `blacklist_add` from MCP; verify number is in local DB **and** blocked on WhatsApp.
- [ ] Call `blacklist_remove` from MCP; verify number is removed from local DB **and** unblocked on WhatsApp.
- [ ] Call `blacklist_get_remote` from MCP; verify it matches your real WhatsApp blocklist.

## Success Criteria
- **Consistency**: The local `blacklisted_numbers` table and the remote WhatsApp blocklist remain in sync.
- **Latency**: Remote block operations confirm success within 2 seconds.
- **Robustness**: If the Gateway is disconnected, the MCP tools report "WhatsApp Gateway is offline; command is pending."
