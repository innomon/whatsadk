# Product Guide - WhatsADK

## Product Overview

WhatsADK is a high-performance Go gateway and agent interaction platform bridging WhatsApp with Google ADK (Agent Development Kit) services. It provides dual-mode WhatsApp connectivity (Multi-Device QR linking via `whatsmeow` and official Meta WABA Cloud API), multi-modal media transformation (image/audio/video normalization), persistent decentralized storage via SQLite-P2P mesh replication, model context protocol (MCP) servers for autonomous AI agents, and interactive TUI interface tools.

## Target Audience

- **Autonomous AI Agents & Developers:** Systems using MCP or A2A protocols to interact with WhatsApp users.
- **Enterprise & Business Bot Operators:** Teams running production WhatsApp workflows via official WABA or linked device multi-device mode.
- **Heterogeneous System Administrators:** Operators deploying gateways on edge hardware (e.g., Raspberry Pi) while orchestrating MCP and agent execution on desktop/cloud workstations (e.g., Mac mini, Ubuntu).

## Key Features & Goals

1. **Dual Gateway Connectivity:** Seamless operation via `whatsmeow` linked devices or official WABA webhooks.
2. **Decentralized Storage & Replication:** Embedded pure-Go SQLite-P2P storage with automatic LAN/WAN mesh replication and replication gating.
3. **Multi-Modal Media Bridge:** Automatic image scaling (896x896 JPEG), audio transcoding (16kHz Mono WAV), and media routing.
4. **Model Context Protocol (MCP) Integration:** Stdio-based MCP server providing full agent control over messaging, contacts, file system logs, blocklists, and WhatsApp groups.
5. **Interactive TUI & Command Utilities:** Terminal-based user interfaces and handcrafted CLI tooling for gateway monitoring, debugging, and database management.
6. **Robust Security & Verification:** RS256 JWT auth, EdDSA-based WhatsApp OAuth, and reverse OTP verification.
