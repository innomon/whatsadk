# Specification: Reverse ADK Simulator (ADKSim)

## Overview
`adksim` is a TUI-based tool that simulates an ADK (Agent Development Kit) server. It allows manual testing of the WhatsADK gateway by acting as the AI agent. It follows the ADK API protocol, receiving prompts from the gateway and allowing a human to provide the agent's response.

## Goals
- Test the **ADK ➔ WhatsApp** flow with manual intervention.
- Debug multi-media handling and message sequencing in the gateway.
- Simulate various agent behaviors (slow responses, streaming, errors) without a real backend.

## Architecture
- **HTTP Server**: Listens for ADK API calls from the gateway.
- **TUI (Bubble Tea)**: Displays incoming requests and captures human responses.
- **Request Manager**: Matches asynchronous HTTP requests with TUI user input.

## Functional Requirements

### 1. ADK API Implementation
The simulator MUST implement the following endpoints:
- `POST /apps/:appName/users/:userID/sessions/:sessionID`:
    - Always returns `200 OK`.
- `POST /run`:
    - Receives `RunRequest` (JSON).
    - Blocks until the human operator provides a response in the TUI.
    - Returns `[]Event` (JSON).
- `POST /run_sse`:
    - Receives `RunRequest` with `streaming: true`.
    - Returns `text/event-stream`.
    - Streams `Event` objects as they are produced in the TUI.

### 2. TUI Features
- **Incoming View**: 
    - Display User ID and Session ID.
    - Show text parts and local file paths for media parts.
- **Input Area**:
    - TextArea for typing responses.
    - Status bar showing connection state and pending requests.
- **Commands**:
    - `/attach <path> [mime]`: Include a file in the next response.
    - `/help`: Show available commands.
    - `/clear`: Clear the chat history.

### 3. Media Handling
- **Inbound**: Decodes `InlineData` (base64) from requests and saves to `adk_media_received/`.
- **Outbound**: Reads local files, detects mime-types, and encodes as `InlineData` for responses.

### 4. Configuration
- Port: Default `8080`.
- Media Directory: Default `adk_media_received/`.
