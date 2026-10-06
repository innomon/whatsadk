# WhatsADK Interactive Command TUI & MCP Shell

An interactive, multi-format (Plain text, Markdown, A2UI) terminal user interface shell for WhatsADK. The TUI allows executing handcrafted commands, custom SQL queries, and all Model Context Protocol (MCP) tools directly over an embedded, bidirectional `sqlite-p2p` swarm connection.

## 🚀 Key Features

- **Direct Terminal Prompt:**
  - Type commands directly (e.g. `sql query="SELECT * FROM filesys LIMIT 5"`, `send_message jid="15551234567@s.whatsapp.net" text="Hello"`).
  - First word is interpreted as the command name (case-insensitive). Positional arguments are automatically mapped to parameter definitions.
- **Slash Modal Popup (`/`):**
  - Press `/` at an empty prompt to open the Slash Command modal dialog.
  - View command help text, parameter definitions, and defaults.
  - Fill parameter inputs interactively or dynamically when values are missing.
- **MCP Tools & SQL Handlers:**
  - Includes pre-registered handcrafted handlers for all MCP tools (`send_message`, `query_contacts`, `jid_to_phone`, `list_groups`, `list_group_members`, `get_recent_messages`, `filesys_sql_select`, `blacklist_add`, `blacklist_remove`, etc.).
  - Handcrafted command execution engine (strictly NO `cobra`/`pflag`).
- **Multi-Format Output Viewport:**
  - **Plain Text:** Status messages and raw logs.
  - **Markdown:** Formatted output with headers, tables, and code blocks.
  - **A2UI:** Structured Agent-to-User Interface component boxes.
- **SQLite-P2P Swarm Integration:**
  - Reads `p2p` configuration from `config.yaml`, joins the specified P2P swarm topic, and displays real-time mesh connection status in the status bar.

## 🛠 Usage

### Build

```bash
make build
# Binary created at bin/tui
```

### Run

```bash
./cmd/run.sh tui -config config.yaml
# Or directly:
./bin/tui -config config.yaml
```

### Configuration (`config.yaml`)

```yaml
p2p:
  enabled: true
  node_id: "tui-node-1"
  swarm_topic: "whatsadk-mesh-topic"
  db_path: "data/tui_p2p.db"

commands:
  - name: "sql"
    handler: "sql_exec"
    help: "Execute custom SQL query against local/P2P filesys"
    params:
      - name: "query"
        type: "string"
        help: "SQL query string"
```
