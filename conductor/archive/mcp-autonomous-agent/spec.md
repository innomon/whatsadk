# Specification: Autonomous MCP-Driven WhatsApp Agent

This specification defines the architecture and interfaces required to allow external AI agents (via MCP) to drive WhatsApp conversations autonomously, optionally bypassing the built-in ADK integration.

## 1. Objectives
- Enable MCP tools to send multi-modal messages (Text, Image, Audio, Video, Document).
- Provide a mechanism for agents to "poll" or "stream" new incoming messages.
- Allow disabling the default ADK forwarding logic to prevent dual-response conflicts.
- Standardize the IPC protocol for message delivery.

## 2. Configuration Changes
The `ADKConfig` will be updated to include an `Enabled` flag.

```yaml
adk:
  enabled: false  # Default is true
  endpoint: "..."
```

When `adk.enabled` is `false`, the Gateway will:
1. Continue to sync contacts and log all incoming messages to the `filesys` table.
2. **NOT** call the ADK `ChatParts` API.
3. **NOT** send any automatic replies.
4. Wait for external commands via the `whatsmeow_commands` table.

## 3. Enhanced IPC Protocol

### 3.1 New Command: `send_message`
The `whatsmeow_commands` table will support a `send_message` type.

**Payload Structure:**
```json
{
  "jid": "1234567890@s.whatsapp.net",
  "text": "Hello from MCP!",
  "media": [
    {
      "data": "base64...",
      "mime_type": "image/jpeg",
      "filename": "optional.jpg"
    }
  ]
}
```

### 3.2 Result Structure:
On success, the `result` field will contain the WhatsApp Message ID.

## 4. MCP Tooling

### 4.1 `send_message` (New)
- **Arguments:** `jid` (string, required), `text` (string, optional), `media_base64` (string, optional), `mime_type` (string, optional).
- **Behavior:** Enqueues a `send_message` command and waits up to 10 seconds for completion.

### 4.2 `get_recent_messages` (Enhanced)
- **Arguments:** `jid` (string, optional), `limit` (int, default 20).
- **Behavior:** Returns a chronological list of incoming and outgoing messages from the `filesys` table for the specified user (or globally if JID is omitted).

## 5. Autonomous Agent Workflow
To operate autonomously (like OpenCode, pi.dev, or Claude Code), the agent should be prompted with the following "Inner Monologue" loop:

1. **Observe**: Call `get_recent_messages` to check for new "request" entries.
2. **Think**: Analyze the user's intent.
3. **Act**: Call `send_message` to respond.
4. **Repeat**: Sleep/wait and repeat.

## 6. Multi-modal Handling
- **Incoming**: The Gateway already downloads and stores media in `filesys`. The MCP tool `get_recent_messages` will provide the `path` to these files or their base64 content.
- **Outgoing**: The MCP tool will accept base64 data, which the Gateway will upload to WhatsApp servers using the existing `sendMediaPart` logic.
