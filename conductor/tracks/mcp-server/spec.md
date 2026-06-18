# MCP Server Specification: WhatsADK Gateway

The **WhatsADK MCP Server** provides a standardized interface for AI models and agents (like Claude Code, OpenCode, and Cursor) to interact with the internal state of the WhatsApp gateway. It exposes contact lists, blacklisted numbers, and message logs (`filesys`) through the Model Context Protocol (MCP).

## Architecture

- **SDK**: `github.com/modelcontextprotocol/go-sdk/mcp`
- **Transport**: `stdio` (Standard input/output), enabling direct use with `claudcode` and `open-code`.
- **Database Context**: Shared PostgreSQL instance (DSN-based).

---

## 🛠 Tools (Actions)

The following tools allow agents to perform side-effects or search for specific records.

### `blacklist_add`
Adds a phone number or JID to the global blacklist.
- **Parameters**:
  - `phone` (string, required): The phone number or JID (e.g., `1234567890@s.whatsapp.net`).
  - `reason` (string, optional): Reason for blacklisting.

### `blacklist_remove`
Removes a phone number or JID from the global blacklist.
- **Parameters**:
  - `phone` (string, required): The phone number or JID to remove.

### `query_contacts`
Searches the `whatsmeow_contacts` table by name or JID.
- **Parameters**:
  - `query` (string, required): A search term (matches `full_name`, `push_name`, or `their_jid`).

### `get_message_logs`
Retrieves recent messages from the `filesys` table for a specific user.
- **Parameters**:
  - `phone` (string, required): The phone number (user ID portion).
  - `limit` (int, optional): Number of messages to return (default: 10).

---

## 📖 Resources (Data)

Resources provide read-only views of the database state.

### `mcp://whatsadk/blacklist`
- **Description**: Returns the full list of currently blacklisted numbers.
- **Format**: JSON array of `{phone, reason, created_at}`.

### `mcp://whatsadk/contacts/summary`
- **Description**: Returns a count of unique contacts and sessions.
- **Format**: JSON object `{total_contacts, sessions}`.

---

## 🛡 Security & Permissions

1.  **Transport**: Limited to local `stdio`. The MCP server is launched by the local agent environment (e.g., Claude Code), inheriting its permissions.
2.  **Read/Write Segregation**: Sensitive tables (sessions/keys) are **NOT** exposed.
3.  **Validation**: All inputs (JIDs/Phone numbers) are validated before being used in SQL queries.

---

## 📦 Data Schemas (JSON)

### Contact Object
```json
{
  "jid": "1234567890@s.whatsapp.net",
  "full_name": "John Doe",
  "push_name": "Johnny",
  "business_name": null
}
```

### FileSys Message Object
```json
{
  "path": "whatsmeow/12345/abc/request",
  "content": "Hello world",
  "metadata": { "mime_type": "text/plain" },
  "timestamp": "2026-04-04T12:00:00Z"
}
```
