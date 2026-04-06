# Implementation Plan: Autonomous MCP-Driven WhatsApp Agent

This plan outlines the steps to enable external AI agents to drive WhatsApp conversations via MCP.

## Phase 1: Configuration & Store Updates 🛠️

### 1.1 `internal/config/config.go`
- [ ] Add `Enabled bool` to `ADKConfig`.
- [ ] Set `Enabled` to `true` by default in `Load()`.

### 1.2 `internal/store/store.go`
- [ ] Add `GetLatestGlobalMessages(ctx, limit int)` to `Store`.
- [ ] Enhance `GetFilesysLogs` to optionally include `content` for small text messages.

## Phase 2: Gateway Execution Layer 🚀

### 2.1 `internal/whatsapp/client.go`
- [ ] Update `handleMessage` to respect `c.cfg.ADK.Enabled`.
- [ ] Add `send_message` to `handleCommand` switch.
- [ ] **Command Implementation**:
    - [ ] `SendMessage(jid, text string, media []MediaPayload) error`
    - [ ] Refactor `sendMediaPart` and `sendTextMessage` to be callable from the command handler.

## Phase 3: MCP Server Layer 🤖

### 3.1 `cmd/mcp/main.go`
- [ ] Register `send_message` tool.
- [ ] Register `get_recent_messages` tool (unifying and enhancing the old `get_message_logs`).

## Phase 4: Agent Configuration (Documentation) 📚

### 4.1 `README.md`
- [ ] Add "Autonomous Mode" section.
- [ ] Provide example prompt/config for Claude Code, OpenCode, and pi.dev.

## Success Criteria
- **Bypass**: Setting `adk.enabled: false` results in zero automatic replies from the Gateway.
- **Multi-modal**: Sending a base64 image via MCP tool `send_message` successfully delivers it to a WhatsApp user.
- **Observability**: `get_recent_messages` accurately shows the "request" (incoming) and "response" (outgoing) flow for any user.
- **Agent Integration**: An agent can be given a high-level task and use these tools to execute a conversation.
