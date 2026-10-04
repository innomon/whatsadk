# ADK Router Example

This example demonstrates a sophisticated **Router Agent** that acts as a gateway to multiple other ADK agents. It can route user requests based on state, user preferences, and even use an LLM-based classifier to determine the best target agent.

## Features

- **Dynamic Routing**: Routes requests to different backend agents based on user session state.
- **In-Process Execution**: Supports running agents in-process using `agentic` configurations for better performance (no network hop).
- **Remote Routing**: Supports routing to remote agents via HTTP (A2A protocol).
- **LLM Classifier**: Uses an LLM to help identify which application the user wants to use when the input is ambiguous.
- **State Management**: Uses the embedded `sqlite-p2p` decentralized store to keep track of user-selected applications and session history.
- **Automatic Selection**: If only one application is available to a user, it routes directly. If multiple are available, it prompts for selection.

## Database Storage & File Locations

The Router uses the embedded `sqlite-p2p` storage backend configured in `router.yaml`:
- **Default DSN:** `sqlite-p2p://data/whatsadk_p2p.db?wal=true`
- **Path Resolution:** Relative paths (`data/whatsadk_p2p.db`) are created relative to the working directory where the router process is executed.
- **Files Created:**
  - `whatsadk_p2p.db`: Primary SQLite database file containing filesys logs (`router/<userID>/apps.json`, `router/<userID>/state.json`).
  - `whatsadk_p2p.db-wal`: Write-Ahead Log for concurrent read/write operations.
  - `whatsadk_p2p.db-shm`: Shared-Memory index.
- **Directory Creation:** The `data/` directory is automatically created on startup if absent.
- **Absolute Paths:** Set `database_url: "sqlite-p2p:///var/data/whatsadk_p2p.db?wal=true"` for fixed absolute path storage.

## Configuration

The router is configured via `router.yaml`.

### Example `router.yaml`

```yaml
default_app: "ignore"
database_url: "sqlite-p2p://data/whatsadk_p2p.db?wal=true"

apps:
  - appName: "admin"
    a2aURL: "http://localhost:8081"
    adkAppName: "admin-agent"
    title: "Admin Tools"
  - appName: "shopper"
    agenticConfig: "shopper-agentic.yaml"
    adkAppName: "shopper-agent"
    title: "Shopping Assistant"

classifier:
  provider: "ollama"
  model: "gemma2"
  endpoint: "http://localhost:11434/v1"
  api_key: "none"
  prompt: "The user said: '${text}'. Which of the following options does this match best? ${optios}. Return the index number (1-N). Return 0 if none match."
  fallback_message: "Sorry, I couldn't identify the application. Please reply with the number or title."

prompts:
  selection: "I found multiple applications for you. Which one would you like to use?\n\n${optios}\n\nPlease reply with a number or name."
```

### App Configuration Types

1. **Remote Agents**: Defined using `a2aURL`. The router will call these agents over HTTP.
2. **In-Process Agents**: Defined using `agenticConfig`. The router will load the agent defined in the provided `agentic` YAML file and execute it in-process using `runner.Runner`.

## Agentic Configuration (`shopper-agentic.yaml`)

For in-process agents, you provide a standard `agentic` configuration file:

```yaml
root_agent: shopper-agent

models:
  gemini-flash:
    provider: gemini
    model_id: gemini-2.0-flash-exp
    default: true

agents:
  shopper-agent:
    description: A simple shopping assistant.
    model: gemini-flash
    instruction: |
      You are a shopping assistant. help the user find products.
```

## How it Works

1. **Identity**: The router identifies the user via their `UserID`.
2. **App Discovery**: It checks the database for a list of apps allowed for that user (`router/<userID>/apps.json`).
3. **Routing Logic**:
    - If a single app is found, it routes directly.
    - If multiple apps are found, it checks `router/<userID>/state.json` to see if a selection is pending.
    - If no selection is pending, it sends a selection menu.
    - Once an app is selected (either by index, title, or LLM classification), the router executes the target app.
4. **Execution**:
    - If the target app has an `agenticConfig`, the router uses an internal `RunnerManager` to execute the agent in the same process.
    - Otherwise, it uses an HTTP client to forward the request to the `a2aURL`.

## Running the Example

1. Ensure the `sqlite-p2p` store DSN is configured in `router.yaml` (default: `sqlite-p2p://data/whatsadk_p2p.db?wal=true`).
2. Build the example:
    ```bash
    go build -o router_example main.go
    ```
3. Run the router (using the default `router.yaml`):
    ```bash
    ./router_example
    ```
4. Alternatively, run with a custom configuration file:
    ```bash
    ./router_example custom-config.yaml
    ```
