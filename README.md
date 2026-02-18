# WhatsADK - WhatsApp to ADK Gateway

A Go utility that connects WhatsApp via QR code and proxies messages to a remote ADK Agent service.

## Features

- Connect to WhatsApp using QR code scanning
- Persistent WhatsApp session storage (SQLite)
- Proxy incoming WhatsApp messages to remote ADK Agent
- Support for both `/run` (single response) and `/run_sse` (streaming) endpoints
- Per-user session management on the ADK service
- JWT authentication with RS256 (asymmetric) signing — includes `user_id` and `channel` custom claims
- **Reverse OTP verification** — apps send a JWT token to the user's WhatsApp; the gateway verifies the sender's phone matches the token's claim and posts a signed callback to confirm identity
- Configurable via YAML file or environment variables

## Requirements

- Go 1.23+
- CGO enabled (for SQLite)
- Running ADK Agent service (local or remote)

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
| `VERIFICATION_ENABLED` | No | Enable reverse OTP verification (`true`) |
| `VERIFICATION_CALLBACK_TIMEOUT` | No | Timeout for verification callback HTTP requests (default: `10s`) |
| `VERIFICATION_DATABASE_URL` | No | PostgreSQL connection string for blacklist store |
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

## Reverse OTP Verification

The gateway supports a two-factor Reverse OTP flow where third-party apps can verify a user's phone number ownership via WhatsApp and deliver an OTP for login:

1. The app generates a signed JWT containing the user's phone number, app name, and challenge ID
2. The user sends this token as a WhatsApp message to the gateway (via a deep link)
3. The gateway verifies:
   - The sender's number is not blacklisted
   - The token signature against the app's registered public key
   - The token hasn't expired
   - The sender's WhatsApp phone number matches the `mobile` claim (E.164 digits comparison), or the sender is a DevOps number
4. On success, the gateway constructs the callback URL from **static per-app config** (not from the JWT), POSTs a signed callback JWT, and receives an OTP in the response
5. The gateway relays the OTP to the user via WhatsApp reply
6. The user enters the OTP in the app's login screen to complete authentication

**Two-factor assurance:** Factor 1 — WhatsApp message (proves phone ownership); Factor 2 — OTP entry in browser (proves session continuity).

**Security design:**
- **No `callback_url` in JWT** — callback destination is derived from static config to prevent SSRF
- **`challenge_id` bound in callback JWT** — prevents confused deputy / cross-challenge replay attacks
- **Redirects disallowed** on callback HTTP client
- **Number blacklisting** via PostgreSQL at the gateway level
- **DevOps override** — configured phone numbers bypass phone mismatch check for testing/operations

### Configuration

```yaml
verification:
  enabled: true
  callback_timeout: "10s"
  database_url: "postgres://localhost:5432/whatsadk?sslmode=disable"  # PostgreSQL for blacklisted numbers
  devops_numbers:           # E.164 digits (no + prefix) allowed to bypass phone mismatch
    - "910000000000"
  apps:
    my-app:
      public_key_path: "secrets/my_app_public.pem"
      callback_base_url: "https://api.my-app.com/api/v1/auth/whatsapp"
  messages:
    otp_delivery: "🔐 Your verification code is: %s\n\nEnter this code in the app to complete login. This code expires in 5 minutes."
    expired: "❌ Verification failed. The link may have expired."
    phone_mismatch: "❌ Verification failed. Please send from the registered number."
    blacklisted: "🚫 This number has been blocked."
    error: "⚠️ Something went wrong. Please try again."
```

Each app must register its RSA public key and callback base URL. The backend callback must return `{"otp":"..."}` in the 200 response body.

**Blacklisted numbers** are stored in PostgreSQL. The `blacklisted_numbers` table is auto-created on first run. Numbers are in E.164 digits format (e.g. `910987654321`). Manage entries directly via `psql`:

```bash
# Add a blacklisted number
psql "$VERIFICATION_DATABASE_URL" -c "INSERT INTO blacklisted_numbers (phone, reason) VALUES ('910987654321', 'spam') ON CONFLICT DO NOTHING;"

# Remove a blacklisted number
psql "$VERIFICATION_DATABASE_URL" -c "DELETE FROM blacklisted_numbers WHERE phone = '910987654321';"

# List all blacklisted numbers
psql "$VERIFICATION_DATABASE_URL" -c "SELECT * FROM blacklisted_numbers;"
```

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

## Architecture

For a detailed architecture overview, see [ARCHITECTURE.md](ARCHITECTURE.md).

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
