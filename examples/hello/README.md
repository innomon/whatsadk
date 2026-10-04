# Hello Work Agent Example

This example demonstrates a deterministic ADK agent integrated with the WhatsApp Gateway. The agent responds to "hello" with a predefined list of capabilities.

## Scenario

1. **User** sends "hello" to the WhatsApp number.
2. **Gateway** receives the message and forwards it to the **Hello Work Agent**.
3. **Agent** recognizes the greeting and responds with a list of tasks.
4. **Gateway** relays the response back to the user on WhatsApp.

## Prerequisites

- Pure Go embedded **SQLite P2P** storage backend (no external database server required).
- Go 1.26+ installed.

## Database Storage & File Locations

This example uses the embedded `sqlite-p2p` backend (`sqlite-p2p://data/whatsadk_p2p.db?wal=true`):

- **Relative Path Resolution:** The database path `data/whatsadk_p2p.db` resolves relative to the current working directory where the process is launched.
  - Running from the repository root (`./bin/gateway -config examples/hello/config.yaml`) creates `./data/whatsadk_p2p.db`.
  - Running from within `examples/hello/` creates `./examples/hello/data/whatsadk_p2p.db`.
- **Generated Files:**
  - `whatsadk_p2p.db`: Main SQLite database storing WhatsApp session credentials, device keys, and gateway tables.
  - `whatsadk_p2p.db-wal`: SQLite Write-Ahead Log for high-concurrency operations.
  - `whatsadk_p2p.db-shm`: SQLite Shared-Memory index file.
- **Directory Creation:** The gateway automatically creates the parent directory (`data/`) on startup if it does not exist.
- **Absolute Path:** You can specify an absolute path (e.g., `sqlite-p2p:///var/lib/whatsadk/whatsadk_p2p.db?wal=true`) to fix the storage path.

## Setup Instructions

### 1. Build the Gateway

In the root directory, run:

```bash
make build
```

### 2. Start the Hello Work Agent

Navigate to the `examples/hello` directory and run the agent as a web API server.

**Standard (Port 8080):**

```bash
cd examples/hello
go run main.go web api
```

**Custom Port (e.g., 8000):**
*Note: The `--port` flag must come before the `api` subcommand.*

```bash
go run main.go web --port 8000 api
```

The agent is now listening. If you used port 8000, it's at `http://localhost:8000`.

**Verify:**
[http://localhost:8000/api/list-apps](http://localhost:8000/api/list-apps)

### 3. Configure and Start the Gateway

In a new terminal, navigate back to the root directory and run the gateway using the example configuration:

```bash
./bin/gateway -config examples/hello/config.yaml
```

### 4. Link WhatsApp

If it's your first time running the gateway:

1. Scan the QR code displayed in the terminal with your WhatsApp app (**Linked Devices** > **Link a Device**).
2. Once connected, send a message saying "hello" to your gateway's WhatsApp number.

## Files

- `main.go`: The ADK agent implementation using `google.golang.org/adk`.
- `config.yaml`: Configuration for the gateway to connect to this local agent using `sqlite-p2p` backend.
