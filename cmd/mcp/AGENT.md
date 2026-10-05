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

The MCP server connects to the WhatsADK database or joins the decentralized SQLite-P2P mesh:

- **SQLite-P2P (Decentralized Mesh)**: Recommended when the MCP server runs on a separate machine from the gateway, or in a decentralized cluster. Configure `p2p.enabled: true` with a local database file and peer addresses pointing to the gateway.
- **PostgreSQL**: `whatsapp.store_dsn` or `verification.database_url` (e.g. `postgres://localhost:5432/whatsadk?sslmode=disable`).
- **SurrealDB**: `surrealdb` configuration block (or `surrealdb://` DSN).

## 🌐 Remote Machine Deployment & P2P Synchronization

In many production or development setups, the **WhatsADK Gateway** runs on a dedicated server or Raspberry Pi (holding the WhatsApp Web session), while the **MCP Server** runs on a developer laptop, a separate workstation, or an AI agent container (Claude Desktop, Gemini CLI, or Pi).

With `sqlite-p2p`, the MCP server runs as an independent peer node in the mesh:

```
┌──────────────────────────────────────┐          ┌──────────────────────────────────────┐
│       WhatsADK Gateway Host          │          │        AI Agent / MCP Host           │
│  (e.g., Raspberry Pi / Server)       │          │  (e.g., Laptop / Claude / Gemini)    │
│                                      │          │                                      │
│  ┌────────────────────────────────┐  │  P2P Mesh│  ┌────────────────────────────────┐  │
│  │ whatsmeow Gateway Session      │  │  Changes │  │ AI Agent (Claude, Gemini, etc.)│  │
│  └───────────────▲────────────────┘  │  Replic. │  └───────────────▲────────────────┘  │
│                  │ Polls commands    │  ◄═════► │                  │ MCP stdio tools   │
│  ┌───────────────▼────────────────┐  │  (Port   │  ┌───────────────▼────────────────┐  │
│  │ SQLite-P2P DB: p2p_gateway.db  │  │   4001)  │  │ SQLite-P2P DB: mcp_p2p.db      │  │
│  │ Node: "gateway-node-1"         │  │          │  │ Node: "mcp-agent-node"         │  │
│  └────────────────────────────────┘  │          │  └────────────────────────────────┘  │
└──────────────────────────────────────┘          └──────────────────────────────────────┘
```

### How Synchronization Works

1. **Local Embedded Database**: The remote MCP server maintains its own local SQLite database (e.g. `data/mcp_p2p.db`), guaranteeing zero network latency for reads (`get_recent_messages`, `query_contacts`, `filesys_get`).
2. **Peer Discovery & Connectivity**:
   - **Same Local Network (LAN)**: If the Gateway and MCP machine are on the same WiFi/Ethernet network, `sqlite-p2p` auto-discovers the Gateway via UDP multicast beacon without requiring manual IP configuration.
   - **Different Networks / Remote WAN**: Specify the gateway's IP or hostname in `p2p.peer_addrs` (or via `P2P_PEER_ADDRS`), e.g. `["192.168.1.100:4001"]` or `["gateway.internal:4001"]`. The MCP server dials the Gateway directly and establishes an encrypted connection.
3. **Command Queue Propagation (`send_message`, `blacklist_add`, `blacklist_remove`)**:
   - When the agent calls `send_message`, MCP enqueues the command in its local `whatsmeow_commands` table with status `pending`.
   - The P2P replication engine transmits the changeset to the remote Gateway.
   - The Gateway executes the command on WhatsApp and writes status `completed` (or `failed`) with the delivery result.
   - The status changeset replicates back to the MCP server.
   - MCP's `WaitForCommand` detects the completion and returns confirmation to the AI agent.
4. **Co-located (Same-Machine) Fallback**:
   - If the MCP server runs on the **same machine** as the Gateway, `whatsadk-mcp` automatically detects if the P2P swarm port (4001) or hypercore feed is locked by the active gateway, and transparently falls back to direct SQLite WAL mode, allowing safe concurrent database access without conflicts.

### Remote MCP Configuration (`config/mcp_remote.yaml`)

```yaml
p2p:
  enabled: true
  node_id: "mcp-agent-client"
  swarm_topic: "whatsadk-mesh-topic"  # Must match the Gateway's topic
  swarm_port: 0                       # 0 = bind to any available local port
  db_path: "data/mcp_p2p.db"          # Local database for this MCP node
  enable_wal: true
  auto_sync: true
  peer_addrs:
    - "192.168.1.100:4001"             # IP/hostname and SwarmPort of the Gateway
  replication:
    mode: "all"
```

## 🚀 How to Run (Manual)

The MCP server communicates over standard input/output (`stdio` transport):

```bash
./bin/whatsadk-mcp -config /config/mcp_remote.yaml
```

## 🤖 Integration with AI Agents

### Gemini CLI

Add the server to your Gemini CLI configuration (`~/.gemini/settings.json`):

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "args": ["-config", "/path/to/whatsadk/config/mcp_remote.yaml"]
    }
  }
}
```

Or configure via environment variables directly:

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "P2P_ENABLED": "true",
        "P2P_NODE_ID": "mcp-gemini-client",
        "P2P_SWARM_TOPIC": "whatsadk-mesh-topic",
        "P2P_PEER_ADDRS": "192.168.1.100:4001",
        "P2P_DB_PATH": "data/mcp_p2p.db"
      }
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
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "P2P_ENABLED": "true",
        "P2P_NODE_ID": "mcp-claude-client",
        "P2P_SWARM_TOPIC": "whatsadk-mesh-topic",
        "P2P_PEER_ADDRS": "192.168.1.100:4001",
        "P2P_DB_PATH": "data/mcp_p2p.db"
      }
    }
  }
}
```

### pi.dev (Pi Coding Agent)

Create or update `.pi/mcp.json`:

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "P2P_ENABLED": "true",
        "P2P_NODE_ID": "mcp-pi-client",
        "P2P_SWARM_TOPIC": "whatsadk-mesh-topic",
        "P2P_PEER_ADDRS": "192.168.1.100:4001",
        "P2P_DB_PATH": "data/mcp_p2p.db"
      },
      "lifecycle": "lazy"
    }
  }
}
```

### OpenCode / Block Goose

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "args": ["-config", "/path/to/whatsadk/config/mcp_remote.yaml"]
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
- `jid_to_phone` (alias `get_phone_from_jid`): Extract international phone number (`E.164` format) and details from a WhatsApp JID (e.g. `15551234567:2@s.whatsapp.net` -> `+15551234567`). Resolves cached LIDs if available.
- `get_recent_messages`: Retrieve recent message logs globally or for a specific user.
- `send_message`: Send multi-modal messages (text and/or media). Supports `context_type` (enum: `"recommendation"`, `"notification"`, `"advertisement"`, `"system"`, `"response"`) and `msg_ref` (original request message ID being replied to) to link the reply.
- `get_database_type`: Discover the active database backend type (`postgres`, `surrealdb`, or `sqlite-p2p`).

### WhatsApp Groups & Members

- `list_groups` (aliases `get_groups`, `get_joined_groups`): List all joined WhatsApp groups along with their subjects/names, topics/descriptions, owner JID, and complete participant/member lists (participant JIDs, admin status, super admin status).
- `list_group_members` (alias `get_group_info`): List members/participants for a specific WhatsApp group JID (e.g. `12036301234567890@g.us`), including admin and super admin status.

### Virtual File System (filesys)

- `filesys_sql_select`: Execute custom SELECT queries for advanced filtering.
- `filesys_put`: Create or update entries in the virtual file system.
- `filesys_get`: Retrieve specific entries by path.
- `filesys_delete`: Remove entries from the file system.
- `filesys_list`: List entries with prefix filtering.

## 💾 Querying filesys with SQL/SurrealQL Dialects

Since the `filesys_sql_select` tool forwards the query directly to the database backend without translation, you must construct the query depending on the database engine.

Use the `get_database_type` tool first to discover the active database type (`postgres`, `surrealdb`, or `sqlite-p2p`).

### 🗄 filesys Table/Collection/View Schema

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

#### SQLite-P2P

In SQLite-P2P, `filesys` is a high-performance SQLite VIEW projected over the unified `crm_store` table:

```sql
CREATE VIEW filesys AS
SELECT
    substr(key, 18) AS path,
    CASE 
        WHEN json_extract(metadata, '$._is_null') = 1 THEN NULL
        ELSE json_remove(metadata, '$._tmstamp')
    END AS metadata,
    data AS content,
    json_extract(metadata, '$._tmstamp') AS tmstamp
FROM crm_store
WHERE key LIKE 'whatsadk:filesys:%';
```

Here are dialect-specific SQL examples for common operations on the `filesys` schema:

### 1. Retrieve the Latest 5 Logs

- **Postgres**:

  ```sql
  SELECT path, metadata->>'mime_type' AS mime_type, tmstamp 
  FROM filesys 
  ORDER BY tmstamp DESC 
  LIMIT 5
  ```

- **SurrealDB**:

  ```surrealql
  SELECT path, metadata, tmstamp 
  FROM filesys 
  ORDER BY tmstamp DESC 
  LIMIT 5
  ```

- **SQLite-P2P**:

  ```sql
  SELECT path, json_extract(metadata, '$.mime_type') AS mime_type, tmstamp 
  FROM filesys 
  ORDER BY tmstamp DESC 
  LIMIT 5
  ```

### 2. Search Messages by Path Prefix (e.g. a particular phone number)

- **Postgres**:

  ```sql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE path LIKE 'whatsmeow/1234567890/%' 
  ORDER BY tmstamp DESC
  ```

- **SurrealDB**:

  ```surrealql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE path CONTAINS 'whatsmeow/1234567890/' 
  ORDER BY tmstamp DESC
  ```

- **SQLite-P2P**:

  ```sql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE path LIKE 'whatsmeow/1234567890/%' 
  ORDER BY tmstamp DESC
  ```

### 3. Filter by Metadata Fields (JSON / Document search)

In PostgreSQL, `metadata` is stored as a native `JSONB` column. In SurrealDB, it is stored as a JSON string. In SQLite-P2P, SQLite JSON functions (`json_extract`) are used.

- **Postgres**:

  ```sql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE metadata->>'mime_type' = 'text/plain' 
  ORDER BY tmstamp DESC
  ```

- **SurrealDB**:

  ```surrealql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE metadata CONTAINS '"mime_type":"text/plain"' 
  ORDER BY tmstamp DESC
  ```

- **SQLite-P2P**:

  ```sql
  SELECT path, tmstamp 
  FROM filesys 
  WHERE json_extract(metadata, '$.mime_type') = 'text/plain' 
  ORDER BY tmstamp DESC
  ```

### 4. Search Content by Substring (Text Message)

- **Postgres** (Note: `content` is a `BYTEA` column in Postgres, so it must be cast/encoded to match text):

  ```sql
  SELECT path, encode(content, 'escape') AS message 
  FROM filesys 
  WHERE encode(content, 'escape') LIKE '%hello%'
  ```

- **SurrealDB**:

  ```surrealql
  SELECT path, content 
  FROM filesys 
  WHERE content CONTAINS 'hello'
  ```

- **SQLite-P2P** (`content` is stored as `BLOB` / text bytes):

  ```sql
  SELECT path, CAST(content AS TEXT) AS message 
  FROM filesys 
  WHERE CAST(content AS TEXT) LIKE '%hello%'
  ```

## 🌐 SQLite-P2P Replication Gating & Decentralized Mesh

When running with `sqlite-p2p` backend, the MCP server accesses an embedded, pure Go decentralized store that can replicate across nodes via Autobase and Hyperswarm.

Replication access is gated using cryptographic public keys (`[32]byte` hex strings) in three modes configured under `p2p.replication` in `config.yaml`:

- `all`: Unrestricted peer synchronization.
- `whitelist`: Replicate only with authorized peer public keys.
- `blacklist`: Block unauthorized peer public keys.

For detailed SQLite-P2P storage architecture and configuration, see [docs/sqlite-p2p-storage.md](/docs/sqlite-p2p-storage.md).

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
