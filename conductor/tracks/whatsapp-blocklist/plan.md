# Implementation Plan: Unified WhatsApp Blocklist Management

This plan outlines the steps to unify local "Shadow Ban" and remote WhatsApp-level blocking.

## Phase 1: Data Layer (Shared Store) ✅

### 1.1 Store Migrations (`internal/store/store.go`)
- [x] Add `whatsmeow_contacts` table to `migrate()`.
- [x] Add `whatsmeow_commands` table for IPC.

### 1.2 Store Methods
- [x] **Command Queueing**:
    - `EnqueueCommand(ctx, cmd string, payload interface{}) (int64, error)`
    - `UpdateCommandStatus(ctx, id int64, status string, result interface{}) error`
    - `WaitForCommand(ctx, id int64, timeout time.Duration) (string, error)`
- [x] **Polling**:
    - `PollPendingCommands(ctx) ([]Command, error)`

## Phase 2: Gateway Execution Layer ✅

### 2.1 WhatsApp Client Enhancements (`internal/whatsapp/client.go`)
- [x] Implement `processCommands(ctx)` background loop.
- [x] Handle `block` / `unblock` / `get_blocklist` commands.
- [x] **Action Methods**:
    - `RemoteBlock(jid string) error`
    - `RemoteUnblock(jid string) error`
    - `RemoteGetBlocklist() ([]string, error)`

### 2.2 IPC Integration
- [x] Start the command processor goroutine in `Run()`.

## Phase 3: MCP User Layer (`cmd/mcp/main.go`) ✅

### 3.1 Handler Updates
- [x] **`blacklist_add`**: Update to call `s.AddBlacklist` **and** `s.EnqueueCommand`. Wait for remote confirmation.
- [x] **`blacklist_remove`**: Update to call `s.RemoveBlacklist` **and** `s.EnqueueCommand`. Wait for remote confirmation.
- [x] **`blacklist_get_remote`**: New tool handler that enqueues `get_blocklist` and returns the result.

## Phase 4: Verification ✅

### 4.1 Functional Tests
- [x] Call `blacklist_add` from MCP; verify number is in local DB **and** blocked on WhatsApp.
- [x] Call `blacklist_remove` from MCP; verify number is removed from local DB **and** unblocked on WhatsApp.
- [x] Call `blacklist_get_remote` from MCP; verify it matches your real WhatsApp blocklist.

## Phase 5: Documentation ✅
- [x] Update `README.md` with descriptions of the new MCP tools.
- [x] Update `ARCHITECTURE.md` to explain the Command Queue IPC model.

## Success Criteria
- **Consistency**: The local `blacklisted_numbers` table and the remote WhatsApp blocklist remain in sync. ✅
- **Latency**: Remote block operations confirm success within 2 seconds. ✅
- **Robustness**: If the Gateway is disconnected, the MCP tools report "WhatsApp Gateway is offline; command is pending." ✅
