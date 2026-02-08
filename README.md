# WhatsADK - WhatsApp to ADK Gateway

A Go utility that connects WhatsApp via QR code and proxies messages to a remote ADK Agent service.

## Features

- Connect to WhatsApp using QR code scanning
- Persistent WhatsApp session storage (SQLite)
- Proxy incoming WhatsApp messages to remote ADK Agent
- Support for both `/run` (single response) and `/run_sse` (streaming) endpoints
- Per-user session management on the ADK service
- JWT authentication with RS256 (asymmetric) signing — includes `user_id` and `channel` custom claims
- Configurable via YAML file or environment variables

## Requirements

- Go 1.23+
- CGO enabled (for SQLite)
- Running ADK Agent service (local or remote)

## Project Structure

```
whatsadk/
├── cmd/gateway/main.go          # Application entry point
├── internal/
│   ├── auth/
│   │   ├── claims.go            # JWT custom claims (user_id, channel)
│   │   ├── jwt.go               # RS256 JWT token generator
│   │   └── jwt_test.go          # JWT tests
│   ├── config/config.go         # YAML config loader with env overrides
│   ├── agent/client.go          # ADK REST/SSE client
│   └── whatsapp/client.go       # WhatsApp client with QR authentication
├── config/config.yaml           # Default configuration
├── go.mod
└── README.md
```

## Installation

```bash
go build -o whatsadk ./cmd/gateway
```

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ADK_ENDPOINT` | No | ADK service URL (default: `http://localhost:8000`) |
| `ADK_APP_NAME` | No | Agent application name |
| `ADK_API_KEY` | No | API key for authenticated endpoints |
| `AUTH_JWT_PRIVATE_KEY_PATH` | No | Path to RSA private key PEM file for JWT auth |
| `CONFIG_FILE` | No | Path to config file |

### Config File

The gateway searches for `config.yaml` in this order:
1. Path passed via `-config` flag
2. Path in `CONFIG_FILE` env var
3. `./config.yaml`
4. `./config/config.yaml`
5. Executable directory

Example `config/config.yaml`:
```yaml
whatsapp:
  store_path: "whatsapp.db"    # SQLite database for WhatsApp session
  log_level: "INFO"            # DEBUG, INFO, WARN, ERROR
  whitelisted_users:           # Phone numbers allowed regardless of country
    - "1234567890"

adk:
  endpoint: "http://localhost:8000"  # ADK service URL
  app_name: "my_agent"               # Agent app name registered in ADK
  streaming: false                    # Use SSE streaming (true) or single response (false)
  # api_key: set via ADK_API_KEY environment variable

auth:
  jwt:
    # private_key_path: "secrets/jwt_private.pem"  # RSA private key (PEM) for RS256 signing
    # issuer: "whatsadk-gateway"
    # audience: "adk-agent"
    # ttl: "2m"                                     # Token lifetime (default: 2m)
```

## Usage

### 1. Start your ADK Agent

First, ensure your ADK agent is running. For example:

```bash
# Python
adk api_server

# Go
go run agent.go web api

# Or connect to a remote ADK Studio instance
```

### 2. Run the Gateway

```bash
# With default config (localhost:8000)
./whatsadk

# With custom endpoint
ADK_ENDPOINT=https://my-adk-service.example.com ./whatsadk

# With custom config file
./whatsadk -config /path/to/config.yaml
```

### 3. Link WhatsApp (First Run)

1. Run the gateway - a QR code will appear in terminal
2. Open WhatsApp on your phone
3. Go to **Settings** > **Linked Devices**
4. Tap **Link a Device**
5. Scan the QR code

### Subsequent Runs

The session is persisted in `whatsapp.db`. Future runs will reconnect automatically.

## How It Works

```
┌──────────────┐     ┌─────────────────┐     ┌───────────────┐
│  WhatsApp    │────▶│  WhatsADK       │────▶│  ADK Service  │
│  User        │◀────│  Gateway        │◀────│  (Remote)     │
└──────────────┘     └─────────────────┘     └───────────────┘
       │                     │                      │
       │              HTTP POST /run                │
       │              or /run_sse                   │
       │                     │                      │
       │              ┌──────┴──────┐               │
       │              │  SQLite DB  │               │
       │              │  (WhatsApp  │               │
       │              │   session)  │               │
       │              └─────────────┘               │
```

### Flow

1. User sends a WhatsApp message
2. Gateway receives message via whatsmeow library
3. Gateway creates/reuses session on ADK service (`POST /apps/{app}/users/{user}/sessions/{session}`)
4. Gateway sends message to ADK agent (`POST /run` or `POST /run_sse`)
5. ADK agent processes message and returns response
6. Gateway sends response back to user via WhatsApp

### API Endpoints Used

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/apps/{app}/users/{user}/sessions/{session}` | POST | Create or reuse session |
| `/run` | POST | Send message, get single response |
| `/run_sse` | POST | Send message, stream response via SSE |

## JWT Authentication

The gateway supports RS256 (asymmetric) JWT authentication for requests to the ADK service. When enabled, each request includes a short-lived Bearer token with custom claims:

- `user_id` — the WhatsApp sender's phone number
- `channel` — always `"whatsapp"`

### Setup

1. Generate an RSA key pair:
   ```bash
   openssl genrsa -out secrets/jwt_private.pem 2048
   openssl rsa -in secrets/jwt_private.pem -pubout -o secrets/jwt_public.pem
   ```

2. Configure the private key path in `config.yaml`:
   ```yaml
   auth:
     jwt:
       private_key_path: "secrets/jwt_private.pem"
       issuer: "whatsadk-gateway"
       audience: "adk-agent"
       ttl: "2m"
   ```

3. Share `jwt_public.pem` with the ADK service for token verification.

When `private_key_path` is not set, JWT auth is disabled and the gateway falls back to static API key authentication (if configured).

For the ADK Go server-side verification implementation, see [docs/adk-jwt-auth-server.md](docs/adk-jwt-auth-server.md).

## Connecting to Different ADK Services

### Local Development Server
```yaml
adk:
  endpoint: "http://localhost:8000"
  app_name: "my_agent"
```

### ADK Studio / Cloud Run
```yaml
adk:
  endpoint: "https://your-adk-instance.run.app"
  app_name: "production_agent"
  # api_key: set via ADK_API_KEY for authenticated endpoints
```

### Vertex AI Agent Engine
```yaml
adk:
  endpoint: "https://your-agent-engine-endpoint"
  app_name: "deployed_agent"
  # api_key: use service account or OAuth token
```

## Dependencies

- [whatsmeow](https://github.com/tulir/whatsmeow) - WhatsApp Web multidevice API
- [ADK](https://github.com/google/adk-go) - Google Agent Development Kit (remote service)

## Notes

- Only private (non-group) messages are processed
- Messages from self are ignored
- User access control: whitelisted users are always allowed; non-whitelisted users must have an Indian phone number (+91), otherwise they receive a rejection message
- Each WhatsApp user gets their own session on the ADK service
- Session history is managed by the ADK service

## License

Apache 2.0
