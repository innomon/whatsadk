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
The MCP server requires access to the same database as the main Gateway. Ensure your `config.yaml` is correctly configured with the PostgreSQL DSN.

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
- `send_message`: Send multi-modal messages (text and/or media).

### Virtual File System (filesys)
- `filesys_sql_select`: Execute custom SELECT queries for advanced filtering.
- `filesys_put`: Create or update entries in the virtual file system.
- `filesys_get`: Retrieve specific entries by path.
- `filesys_delete`: Remove entries from the file system.
- `filesys_list`: List entries with prefix filtering.

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
