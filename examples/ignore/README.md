# Ignore Agent Example (Silent Ignore)

This example demonstrates a deterministic ADK agent that uses the **Silent Ignore** feature of the WhatsApp Gateway. It only responds to users listed in a `whitelist.json` file; all other users are silently ignored, and the reason is recorded in the gateway's storage.

## Scenario
1. **User** sends a message to the WhatsApp number.
2. **Gateway** receives the message and forwards it to the **Ignore Agent**.
3. **Agent** checks the user's mobile number against `whitelist.json`.
4. **If Whitelisted:** The agent responds with a greeting.
5. **If NOT Whitelisted:** The agent sends a special `application/x-adk-silent-ignore` signal.
6. **Gateway** detects the signal, suppresses the WhatsApp reply, and logs the ignore event.

## Prerequisites
- A running PostgreSQL instance for WhatsApp session storage.
- Go 1.25+ installed.

## Setup Instructions

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
Navigate to the `examples/ignore` directory and run the agent as a web API server.

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

## Files
- `main.go`: The ADK agent implementation with whitelist logic and silent ignore signaling.
- `whitelist.json`: List of mobile numbers allowed to interact with the agent.
- `config.yaml`: Configuration for the gateway to connect to this local agent.
