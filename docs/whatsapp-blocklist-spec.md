# Specification: Unified WhatsApp Blocklist Management

## 1. Objective
Unify the existing local "Shadow Ban" (Gateway-level filtering) with remote WhatsApp-level blocking. This ensures that when a number is blacklisted, it is both ignored by the WhatsADK Gateway and officially blocked on WhatsApp servers.

## 2. Architecture

### 2.1 Unified Flow
1.  **MCP Tool Call**: An agent calls `blacklist_add(phone, reason)`.
2.  **Local Persistence**: The `blacklisted_numbers` table is updated immediately.
3.  **Command Enqueue**: A "remote_block" command is written to the `whatsmeow_commands` table.
4.  **Gateway Execution**: The active Gateway process polls the command, executes `UpdateBlocklist` via `whatsmeow`, and updates the command status to `completed`.
5.  **MCP Response**: The MCP server waits (with timeout) for the Gateway to confirm the remote block before returning success to the agent.

### 2.2 IPC Strategy
- **Command Queue**: PostgreSQL table `whatsmeow_commands` acts as the bridge between the MCP process and the Gateway process.

## 3. Tool Definitions (Unified)

### `blacklist_add` (Updated)
- **Action**: Adds to local DB **and** triggers a remote WhatsApp block.
- **Parameters**: `phone`, `reason`.

### `blacklist_remove` (Updated)
- **Action**: Removes from local DB **and** triggers a remote WhatsApp unblock.
- **Parameters**: `phone`.

### `blacklist_list_remote` (New)
- **Action**: Fetches the official blocklist from WhatsApp servers (via Gateway).
- **Description**: Returns the list of JIDs blocked on the WhatsApp account.

## 4. Database Schema

### `whatsmeow_commands`
| Field | Type | Description |
|---|---|---|
| `id` | SERIAL | Primary Key |
| `command` | TEXT | `block`, `unblock`, `get_blocklist` |
| `payload` | JSONB | e.g., `{"jid": "12345@s.whatsapp.net"}` |
| `status` | TEXT | `pending`, `processing`, `completed`, `failed` |
| `result` | JSONB | Success data or error message |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

## 5. Security & Validation
- **JID Parsing**: Mandatory `@s.whatsapp.net` suffixing for raw numbers.
- **Error Handling**: If the remote block fails but the local add succeeds, the user is notified of the partial failure.
