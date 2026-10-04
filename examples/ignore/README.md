# Ignore Agent Example (Silent Ignore)

This example demonstrates a deterministic ADK agent that uses the **Silent Ignore** feature of the WhatsApp Gateway. It only responds to users listed in a `whitelist.json` file; all other users are silently ignored, and the reason is recorded in the gateway's storage.

## Scenario

1. **User** sends a message to the WhatsApp number.
2. **Gateway** receives the message and forwards it to the **Ignore Agent**.
3. **Agent** checks the user's mobile number against `whitelist.json`.
4. **If Whitelisted:** The agent responds with a greeting.
5. **If NOT Whitelisted:** The agent sends a special `application/x-adk-silent-ignore` signal.
6. **Gateway** detects the signal, suppresses the WhatsApp reply, and logs the ignore event to the `sqlite-p2p` filesys storage.

## Prerequisites

- Pure Go embedded **SQLite P2P** backend (no external database server required).
- Go 1.26+ installed.

## Database Storage & File Locations

This example uses the embedded `sqlite-p2p` storage backend (`sqlite-p2p://data/whatsadk_p2p.db?wal=true`):

- **Path Resolution:** The database path `data/whatsadk_p2p.db` resolves relative to the current working directory from which the gateway process is started.
  - Running from the repository root creates `./data/whatsadk_p2p.db`.
  - Running from within `examples/ignore/` creates `./examples/ignore/data/whatsadk_p2p.db`.
- **Files Created:**
  - `whatsadk_p2p.db`: Main SQLite database containing WhatsApp session state and silent ignore filesys audit logs.
  - `whatsadk_p2p.db-wal`: Write-Ahead Log for high-performance concurrent writes.
  - `whatsadk_p2p.db-shm`: Shared-Memory index file.
- **Directory Creation:** The `data/` directory is automatically created on startup if absent.
- **Absolute Paths:** Supply an absolute path (e.g., `sqlite-p2p:///var/data/whatsadk_p2p.db?wal=true`) for a persistent fixed location.

---

## Setup Instructions (Single WhatsApp Number)

### 1. Configure the Whitelist

Edit `examples/ignore/whitelist.json` and add your WhatsApp mobile number (including country code, e.g., `910000000000`).

```json
[
  "910000000000"
]
```

### 2. Build the Gateway

In the root directory, run:

```bash
make build
```

### 3. Start the Ignore Agent

Navigate to the `examples/ignore` directory and run the agent as a web API server:

```bash
cd examples/ignore
go run main.go web api
```

The agent is now listening on port 8080.

### 4. Configure and Start the Gateway

In a new terminal, navigate back to the root directory and run the gateway using the example configuration:

```bash
./bin/gateway -config examples/ignore/config.yaml
```

### 5. Test

1. Send a message from a **whitelisted** number. You should receive a response.
2. Send a message from a **non-whitelisted** number. You will receive NO response, but the gateway terminal will log:
   `Silently ignoring message from {userID}. Reason: User not in whitelist`

---

## Running Two Instances for Two WhatsApp Numbers (Same Machine)

To serve **two WhatsApp numbers simultaneously** on the same machine using `sqlite-p2p`, you run two separate gateway processes with distinct storage files and node IDs while keeping the same `swarm_topic`:

### Requirements & Configuration Rules

| Setting | Instance 1 (`config_num1.yaml`) | Instance 2 (`config_num2.yaml`) | Rule |
| --- | --- | --- | --- |
| `whatsapp.store_dsn` | `sqlite-p2p://data/wa_num1.db?wal=true` | `sqlite-p2p://data/wa_num2.db?wal=true` | **Must be distinct** to keep WhatsApp session keys isolated |
| `verification.database_url` | `sqlite-p2p://data/p2p_num1.db?wal=true` | `sqlite-p2p://data/p2p_num2.db?wal=true` | **Must be distinct** to avoid SQLite file lock conflicts |
| `p2p.node_id` | `ignore-gateway-node-1` | `ignore-gateway-node-2` | **Must be unique** to prevent Autobase node identity collision |
| `p2p.swarm_topic` | `whatsadk-mesh-topic` | `whatsadk-mesh-topic` | **Keep same** so both nodes sync audit logs and blacklists via P2P |
| `p2p.db_path` | `data/p2p_num1.db` | `data/p2p_num2.db` | **Must be distinct** matching verification database path |

### Multi-Instance Launch Steps

1. **Start the single backend Agent** (Terminal 1):

   ```bash
   cd examples/ignore
   go run main.go web api webui
   ```

2. **Start Gateway Instance 1 for WhatsApp Number 1** (Terminal 2):

   ```bash
   ./bin/gateway -config examples/ignore/config_num1.yaml
   ```

   *Scan the terminal QR code with your first phone.*

3. **Start Gateway Instance 2 for WhatsApp Number 2** (Terminal 3):

   ```bash
   ./bin/gateway -config examples/ignore/config_num2.yaml
   ```

   *Scan the terminal QR code with your second phone.*

Both numbers will forward interactions to the same Ignore Agent backend, while synchronizing audit records and verification state over the local P2P swarm.

### Backend Agent Topologies

You have two architectural options for connecting your multi-number gateways to ADK agents:

#### Option A: Single Shared Agent (Default)

Both gateway instances point their `adk.endpoint` to the same agent (e.g. `http://localhost:8080/api`).

- **How it works:** The ADK server handles requests concurrently and isolates conversation state by the sender's WhatsApp phone number (`UserID`).
- **Best for:** When both WhatsApp numbers should run the exact same bot behavior (e.g. load-balanced customer support).

#### Option B: Dedicated Agent per WhatsApp Number

Each gateway instance points to its own ADK agent server running on a distinct port or URL.

- **How it works:**
  - Gateway 1 (`config_num1.yaml`): points to `http://localhost:8080/api` (`app_name: "SupportAgent"`).
  - Gateway 2 (`config_num2.yaml`): points to `http://localhost:8000/api` (`app_name: "AdminAgent"`).
- **Best for:** When each WhatsApp number represents a different persona, brand, or specialized assistant.

---

## Files

- `main.go`: The ADK agent implementation with whitelist logic and silent ignore signaling.
- `whitelist.json`: List of mobile numbers allowed to interact with the agent.
- `config.yaml`: Default configuration for a single WhatsApp gateway instance.
- `config_num1.yaml`: Configuration for WhatsApp number 1 in a multi-instance P2P setup.
- `config_num2.yaml`: Configuration for WhatsApp number 2 in a multi-instance P2P setup.
