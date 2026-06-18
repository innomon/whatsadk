# WhatsADK MCP Server Agent Guide

The WhatsADK MCP (Model Context Protocol) server provides a standardized interface for AI models and agents to interact with the WhatsApp Gateway. It allows agents to read messages, send replies, manage the blacklist, and interact with the virtual file system.

## 🛠 Build and Installation

### 1. Build the Binary
From the project root, run:
```bash
make build-mcp
```
This will create the `whatsadk-mcp` binary in the `bin/` directory.

### 2. Configuration
The MCP server requires access to the same database as the main Gateway. Ensure your `config.yaml` is correctly configured with the PostgreSQL DSN or the SurrealDB configuration block.

## 🚀 How to Run (Manual)
The MCP server uses `stdio` transport. You can test it manually (though it's designed for machine interaction):
```bash
./bin/whatsadk-mcp -config ./config/config.yaml
```

## 🤖 Integration with AI Agents

### Gemini CLI
Add the server to your Gemini CLI configuration:
```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/absolute/path/to/whatsadk/bin/whatsadk-mcp",
      "args": ["-config", "/absolute/path/to/whatsadk/config/config.yaml"]
    }
  }
}
```

### Claude Desktop / Claude Code
Update your `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/absolute/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "CONFIG_FILE": "/absolute/path/to/whatsadk/config/config.yaml"
      }
    }
  }
}
```

### pi.dev (Pi Coding Agent)
Create or update `.pi/mcp.json` in your project root:
```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/absolute/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "CONFIG_FILE": "/absolute/path/to/whatsadk/config/config.yaml"
      },
      "lifecycle": "lazy"
    }
  }
}
```

### OpenCode / Block Goose
Add the following to your MCP configuration file (usually `mcp.json` or `config.json` in the tool's config directory):
```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/absolute/path/to/whatsadk/bin/whatsadk-mcp",
      "args": ["-config", "/absolute/path/to/whatsadk/config/config.yaml"]
    }
  }
}
```

## 🛠 Available Tools

### Blacklist Management
- `blacklist_add`: Block a phone number/JID (Local Shadow Ban + Remote WhatsApp Block).
- `blacklist_remove`: Unblock a phone number/JID.
- `blacklist_get_remote`: Fetch the official blocklist from WhatsApp servers.

### Contacts & Messaging
- `query_contacts`: Search for WhatsApp contacts by name or JID.
- `get_recent_messages`: Retrieve recent message logs globally or for a specific user.
- `send_message`: Send multi-modal messages (text and/or media). Supports `context_type` (enum: `"recommendation"`, `"notification"`, `"advertisement"`, `"system"`, `"response"`) and `msg_ref` (original request message ID being replied to) to link the reply.
- `get_database_type`: Discover the active database backend type (`postgres` or `surrealdb`).

### Virtual File System (filesys)
- `filesys_sql_select`: Execute custom SELECT queries for advanced filtering.
- `filesys_put`: Create or update entries in the virtual file system.
- `filesys_get`: Retrieve specific entries by path.
- `filesys_delete`: Remove entries from the file system.
- `filesys_list`: List entries with prefix filtering.

## 💾 Querying filesys with SQL/SurrealQL Dialects

Since the `filesys_sql_select` tool forwards the query directly to the database backend without translation, you must construct the query depending on the database engine.

Use the `get_database_type` tool first to discover the active database type (`postgres` or `surrealdb`).

### 🗄 filesys Table/Collection Schema

#### PostgreSQL
```sql
CREATE TABLE filesys (
    path     TEXT PRIMARY KEY,            -- Format: whatsmeow/<phone>/<uniqueID>/<request|response>
    metadata JSONB,                       -- JSON Object containing mime_type, errors, etc.
    content  BYTEA,                       -- Message content bytes (encoded text or binary data)
    tmstamp  TIMESTAMPTZ DEFAULT NOW()   -- Log creation timestamp
);

CREATE INDEX idx_filesys_metadata ON filesys USING GIN (metadata);
```

#### SurrealDB
```surrealql
-- The table is dynamically/schemalessly defined.
-- Record ID is derived from the MD5 hash of the path: filesys:<md5(path)>
DEFINE TABLE filesys SCHEMALESS;
-- Document Fields:
--   path: string                         -- Format: whatsmeow/<phone>/<uniqueID>/<request|response>
--   metadata: string                     -- JSON stringified metadata object
--   content: bytes                       -- Message content bytes
--   tmstamp: datetime                    -- Log creation timestamp
```

Here are dialect-specific SQL examples for common operations on the `filesys` schema:

### 1. Retrieve the Latest 5 Logs
* **Postgres**:
  ```sql
  SELECT path, metadata->>'mime_type' AS mime_type, tmstamp 
  FROM filesys 
  ORDER BY tmstamp DESC 
  LIMIT 5
  ```
* **SurrealDB**:
  ```surrealql
  SELECT path, metadata, tmstamp 
  FROM filesys 
  ORDER BY tmstamp DESC 
  LIMIT 5
  ```

### 2. Search Messages by Path Prefix (e.g. a particular phone number)
* **Postgres**:
  ```sql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE path LIKE 'whatsmeow/1234567890/%' 
  ORDER BY tmstamp DESC
  ```
* **SurrealDB**:
  ```surrealql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE path CONTAINS 'whatsmeow/1234567890/' 
  ORDER BY tmstamp DESC
  ```

### 3. Filter by Metadata Fields (JSON / Document search)
In PostgreSQL, `metadata` is stored as a native `JSONB` column. In SurrealDB, it is stored as a JSON-encoded string.
* **Postgres**:
  ```sql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE metadata->>'mime_type' = 'text/plain' 
  ORDER BY tmstamp DESC
  ```
* **SurrealDB**:
  ```surrealql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE metadata CONTAINS '"mime_type":"text/plain"' 
  ORDER BY tmstamp DESC
  ```

### 4. Search Content by Substring (Text Message)
* **Postgres** (Note: `content` is a `BYTEA` column in Postgres, so it must be cast/encoded to match text):
  ```sql
  SELECT path, encode(content, 'escape') AS message 
  FROM filesys 
  WHERE encode(content, 'escape') LIKE '%hello%'
  ```
* **SurrealDB**:
  ```surrealql
  SELECT path, content 
  FROM filesys 
  WHERE content CONTAINS 'hello'
  ```


## 📂 Virtual File System (filesys) Put/Get Examples

Below are JSON examples showing how to use the virtual file system tools (`filesys_put` and `filesys_get`) to write and read files with associated metadata.

### 1. `filesys_put` Example
**Arguments:**
```json
{
  "path": "whatsmeow/1234567890/msg_09876/request",
  "content": "Hello, this is a message content.",
  "metadata": {
    "mime_type": "text/plain",
    "sender_name": "Alice"
  }
}
```

**Response (Success):**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Successfully stored entry at whatsmeow/1234567890/msg_09876/request"
    }
  ]
}
```

### 2. `filesys_get` Example
**Arguments:**
```json
{
  "path": "whatsmeow/1234567890/msg_09876/request"
}
```

**Response (Success):**
The metadata is returned structured under a standard `sql.NullString` JSON block:
```json
{
  "content": [
    {
      "type": "text",
      "text": "{\n  \"path\": \"whatsmeow/1234567890/msg_09876/request\",\n  \"metadata\": {\n    \"String\": \"{\\\"mime_type\\\":\\\"text/plain\\\",\\\"sender_name\\\":\\\"Alice\\\"}\",\n    \"Valid\": true\n  },\n  \"content\": \"SGVsbG8sIHRoaXMgaXMgYSBtZXNzYWdlIGNvbnRlbnQu\",\n  \"timestamp\": \"2026-06-18T19:40:05Z\"\n}"
    }
  ]
}
```
*Note: The `content` field contains the Base64-encoded representation of the content bytes.*


### Router & App Management
- `router_get_apps`: Retrieve provisioned apps for a user.
- `router_set_apps`: Provision apps for a user.
- `router_delete_apps`: Remove provisioned apps.
- `router_get_state`: Retrieve current routing session state.
- `router_set_state`: Update routing session state.
- `router_clear_state`: Clear routing session state.

## 🧠 Autonomous Agent Mode

You can turn an AI agent into the "brain" of your WhatsApp account.

1. **Disable Internal ADK Logic**:
   In `config/config.yaml`:
   ```yaml
   adk:
     enabled: false
   ```
2. **Start the Gateway**: `./bin/gateway`
3. **Prompt your Agent**:
   Give your agent (e.g., Gemini CLI or Claude Code) this system instruction:
   > "Monitor WhatsApp messages using `get_recent_messages`. If you see a new request from a user, process it using your internal tools/knowledge and reply using `send_message`. You can send text and multi-modal media (base64)."
