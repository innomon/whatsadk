# WhatsADK: Routing and Application Provisioning Flow

This document describes the routing engine architecture and app provisioning flow in **WhatsADK**, detailing how the gateway manages multiple agent applications per user using the virtual file system, MCP server, and the Router Agent.

---

## 1. Overview

By default, the WhatsADK Gateway routes all messages to a single, globally configured ADK application (`cfg.ADK.AppName`). However, in multi-agent environments, different users may have access to different sets of applications. 

The **Router Agent** (located in [examples/router](file:///home/innomon/orez/adk/whatsadk/examples/router/)) acts as a meta-agent that intercepts incoming messages, resolves the user's provisioned application set, handles disambiguation/selection menus, and forwards execution to the appropriate downstream agent.

---

## 2. Storage & Virtual File System (VFS) Keys

The router flow relies on two dynamic files stored in the gateway's `filesys` table/collection:

### A. Allowed Apps List (`router/<userID>/apps.json`)
Stores a JSON array of string identifiers representing the ADK applications provisioned (or allowed) for a specific user.
* **Path key in VFS**: `router/<userID>/apps.json`
* **Format**:
  ```json
  ["shopper-agent", "weather-agent", "support-bot"]
  ```

### B. Routing State (`router/<userID>/state.json`)
Stores the current interactive routing session status for the user (e.g. if they are in the middle of choosing an app from a menu).
* **Path key in VFS**: `router/<userID>/state.json`
* **Format**:
  ```json
  {
    "status": "awaiting_disambiguation",
    "options": ["shopper-agent", "weather-agent"],
    "app": ""
  }
  ```

---

## 3. MCP Provisioning Interface (Tools)

Local AI agents can inspect and modify a user's provisioned apps and state using the following Model Context Protocol (MCP) tools:

* **`router_get_apps`**: Reads `router/<userID>/apps.json` and returns the array of allowed app names.
* **`router_set_apps`**: Marshals a string slice of application names and saves it to `router/<userID>/apps.json`.
* **`router_delete_apps`**: Deletes the `router/<userID>/apps.json` file.
* **`router_get_state`**: Retrieves `router/<userID>/state.json`.
* **`router_set_state`**: Saves or updates `router/<userID>/state.json`.
* **`router_clear_state`**: Clears the session routing state by deleting `router/<userID>/state.json`.

---

## 4. End-to-End Routing Lifecycle

When a message is received from a WhatsApp user, the Router Agent executes the following lifecycle:

```mermaid
graph TD
    A[Incoming User Message] --> B[Read router/<userID>/apps.json]
    B --> C{Any apps found?}
    C -->|No| D[Fallback to Default App]
    C -->|Yes| E{How many apps?}
    
    D --> F[Execute Target App]
    
    E -->|Single App| F
    E -->|Multiple Apps| G[Read router/<userID>/state.json]
    
    G --> H{Awaiting selection?}
    H -->|No| I[Generate Disambiguation Options]
    I --> J[Save status='awaiting_disambiguation' to state.json]
    J --> K[Send Selection Menu to User]
    
    H -->|Yes| L[Process User Input]
    L --> M{Matches option index/name?}
    M -->|No| N[LLM / Intent Classification]
    M -->|Yes| O[Resolve Target App]
    N --> O
    O --> P[Update state.json status='active' with selected app]
    P --> F
    
    F --> Q{Is app local/in-process?}
    Q -->|Yes| R[Execute via RunnerManager]
    Q -->|No| S[Forward via HTTP Client to remote a2aURL]
```

### Steps:
1. **App Discovery**: The router checks `router/<userID>/apps.json` to fetch the allowed apps. If empty, the default application is used.
2. **Direct Route**: If only a single app is provisioned, the message is immediately routed to that application.
3. **Disambiguation / Multi-App State**: 
   * If multiple apps are allowed, the router checks `router/<userID>/state.json`.
   * If the state is not awaiting selection, the router generates a menu list of options (e.g., `[1] Shopper Assistant, [2] Weather Agent`) and sends it back to the user on WhatsApp, writing the pending options state to `router/<userID>/state.json`.
   * When the user replies, the router intercepts the reply, validates it against the active options index/names, or uses an LLM classification step to select the matching application.
4. **Target Execution**:
   * Once selected, the target app is stored in the state.
   * If the target app is configured locally in `shopper-agentic.yaml`, it executes in-process via `RunnerManager`.
   * Otherwise, the router acts as an HTTP proxy, forwarding the message payload directly to the app's external endpoint (`a2aURL`).
