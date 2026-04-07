# Hello Work Agent Example

This example demonstrates a deterministic ADK agent integrated with the WhatsApp Gateway. The agent responds to "hello" with a predefined list of capabilities.

## Scenario
1. **User** sends "hello" to the WhatsApp number.
2. **Gateway** receives the message and forwards it to the **Hello Work Agent**.
3. **Agent** recognizes the greeting and responds with a list of tasks.
4. **Gateway** relays the response back to the user on WhatsApp.

## Prerequisites
- A running PostgreSQL instance for WhatsApp session storage.
  - **Quick Start with Docker (Persistent Data in `sandbox/`):**
    ```bash
    mkdir -p sandbox/db
    docker run --name whatsadk-db \
      -e POSTGRES_DB=whatsadk \
      -e POSTGRES_PASSWORD=whatsadk \
      -v "$(pwd)/sandbox/db:/var/lib/postgresql" \
      -p 5433:5432 -d postgres
    ```
- Go 1.23+ installed.

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
- `config.yaml`: Configuration for the gateway to connect to this local agent.
