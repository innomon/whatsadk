# WhatsADK - WhatsApp to ADK Gateway

A Go utility that connects WhatsApp via QR code and proxies messages to a remote ADK Agent service.

## Features

- Connect to WhatsApp using QR code scanning
- Persistent WhatsApp session storage (PostgreSQL)
- Proxy incoming WhatsApp messages to remote ADK Agent
- **Two-Way Media Bridge** — Automatically intercept, transform, and forward WhatsApp media (images, audio, video) to the ADK agent. Also supports sending media back from the agent to the WhatsApp user.
- Support for both `/run` (single response) and `/run_sse` (streaming) endpoints
- Per-user session management on the ADK service
- JWT authentication with RS256 (asymmetric) signing — includes `user_id` and `channel` custom claims
- **WhatsApp OAuth** — Ed25519/EdDSA-based login flow; SPA users authenticate by sending an AUTH message via WhatsApp deep link and receive a signed JWT for ADK API access
- **Reverse OTP verification** — apps send a JWT token to the user's WhatsApp; the gateway verifies the sender's phone matches the token's claim and posts a signed callback to confirm identity
- Configurable via YAML file or environment variables

## Requirements

- Go 1.25+
- PostgreSQL database
- **FFmpeg** — Required for the **Media Bridge** to process audio (Opus to WAV) and video (sampling frames). Ensure it is available in your system `PATH`.
  - **Ubuntu/Debian:** `sudo apt update && sudo apt install ffmpeg`
  - **macOS:** `brew install ffmpeg`
  - **Windows:** `choco install ffmpeg` or download from [ffmpeg.org](https://ffmpeg.org/download.html)
- Running ADK Agent service (local or remote)

## Installation

The project uses a `Makefile` to manage builds. All binaries are generated in the `bin/` directory.

```bash
# Build all binaries (gateway, simulators, etc.)
make build

# The binaries will be available in:
# bin/gateway
# bin/keygen
# bin/simulator
# bin/adksim
```

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ADK_ENDPOINT` | No | ADK service URL (default: `http://localhost:8000/api`) |
| `ADK_APP_NAME` | No | Agent application name |
| `ADK_API_KEY` | No | API key for authenticated endpoints |
| `AUTH_JWT_PRIVATE_KEY_PATH` | No | Path to RSA private key PEM file for JWT auth |
| `OAUTH_ENABLED` | No | Enable WhatsApp OAuth login (`true`) |
| `OAUTH_KEY_PATH` | No | Path to Ed25519 private key PEM file for OAuth |
| `OAUTH_SPA_URL` | No | SPA base URL for OAuth redirect (e.g., `https://chat.myadk.app`) |
| `VERIFICATION_ENABLED` | No | Enable reverse OTP verification (`true`) |
| `VERIFICATION_CALLBACK_TIMEOUT` | No | Timeout for verification callback HTTP requests (default: `10s`) |
| `WHATSAPP_STORE_DSN` | No | PostgreSQL connection string for WhatsApp session storage |
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
  store_dsn: "postgres://localhost:5432/whatsadk?sslmode=disable"  # PostgreSQL for WhatsApp session
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
./bin/gateway

# With custom endpoint
ADK_ENDPOINT=https://my-adk-service.example.com ./bin/gateway

# With custom config file
./bin/gateway -config /path/to/config.yaml
```

### 3. Link WhatsApp (First Run)

1. Run the gateway - a QR code will appear in terminal
2. Open WhatsApp on your phone
3. Go to **Settings** > **Linked Devices**
4. Tap **Link a Device**
5. Scan the QR code

### Subsequent Runs

The session is persisted in PostgreSQL. Future runs will reconnect automatically.

## Examples

The project includes several examples in the `examples/` directory to help you get started:

- **[Hello Work](examples/hello)**: A simple deterministic agent that responds to "hello" with its capabilities.
- **[Ignore Agent (Silent Ignore)](examples/ignore)**: Demonstrates the "Silent Ignore" feature. It only responds to users in a `whitelist.json` file and silently ignores all others, recording the reason in the gateway storage.

To run an example, follow the instructions in its respective `README.md`.

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
       │              │ PostgreSQL  │               │
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

### Silent Ignore Message

The ADK agent can instruct the gateway to **not** send a reply to the user while still recording the reason for ignoring the message. This is useful for off-topic queries or when the agent determines no response is necessary.

To trigger a silent ignore, the ADK response should include an `inlineData` part:
- **mimeType**: `application/x-adk-silent-ignore`
- **data**: Base64-encoded reason string (e.g., "User is off-topic")

The gateway will log the reason and record it in the `filesys` table as a "response" with an error metadata `Ignored: <reason>`.

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

## WhatsApp OAuth (EdDSA)

The gateway can act as an Identity Provider, allowing SPA users to authenticate via WhatsApp. The flow uses Ed25519/EdDSA-signed JWTs for compact tokens (~350 chars) suitable for delivery via WhatsApp deep links.

**How it works:**
1. SPA generates an Ed25519 key pair and a nonce, then opens a `wa.me` deep link: `AUTH <pubkey> <nonce>`
2. User sends the message to the gateway's WhatsApp number
3. Gateway signs a JWT with the user's phone number and sends back a login link
4. SPA receives the JWT via URL fragment and uses it for ADK API calls

### Setup

1. Generate an Ed25519 key pair:
   ```bash
   go run ./cmd/keygen -out secrets/oauth_ed25519.pem
   ```

2. Configure in `config.yaml`:
   ```yaml
   auth:
     oauth:
       enabled: true
       key_path: "secrets/oauth_ed25519.pem"
       spa_url: "https://chat.myadk.app"
       issuer: "whatsadk-gateway"
       audience: "adk-cloud-proxy"
       ttl: "24h"
       rate_limit: 5
   ```

3. Share the Ed25519 **public key** (printed by `keygen`) with the ADK server for JWT verification.

For the full specification, see [docs/whatsapp-auth-specification.md](docs/whatsapp-auth-specification.md).
For the implementation plan, see [docs/whatsapp-auth-implementation_plan.md](docs/whatsapp-auth-implementation_plan.md).

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

## Database Management

The gateway utilizes PostgreSQL for session storage, contact synchronization, and global blacklisting.

### Contact & Message Storage

The `whatsmeow` library automatically manages session and contact data.

| Feature | Handled By | Default Storage Behavior |
| :--- | :--- | :--- |
| **Sessions & Keys** | `sqlstore` | Automatically saved to DB. |
| **Contacts** | App State Sync | Automatically saved to `whatsmeow_contacts`. |
| **Messages** | `filesys` table | Persistent storage for requests and responses. |

### Schema: `filesys`

The `filesys` table stores both incoming messages (requests) and outgoing responses.

```sql
CREATE TABLE filesys (
    path    TEXT PRIMARY KEY,           -- Format: whatsmeow/<phone>/<uniqueID>/<request|response>
    metadata JSONB,                      -- Mime type and additional metadata (e.g., errors)
    content  BYTEA,                      -- Message content (text/plain)
    tmstamp  TIMESTAMPTZ DEFAULT NOW()  -- Message timestamp
);

CREATE INDEX idx_filesys_metadata ON filesys USING GIN (metadata);
```

### Schema: `whatsmeow_contacts`

The `whatsmeow_contacts` table is automatically populated and updated as `whatsmeow` receives sync events from WhatsApp.

```sql
CREATE TABLE whatsmeow_contacts (
    our_jid       TEXT, -- The JID of the local user session
    their_jid     TEXT, -- The JID of the contact
    full_name     TEXT, -- Contact name from the local address book
    short_name    TEXT, -- A shortened version of the contact name
    push_name     TEXT, -- The name the contact set for themselves
    business_name TEXT, -- Business name (if applicable)
    PRIMARY KEY (our_jid, their_jid)
);
```

### Global Blacklist

Users added to the PostgreSQL blacklist are blocked from all interactions. Manage entries directly via `psql`:

```bash
# Add a blacklisted number or LID
psql "$VERIFICATION_DATABASE_URL" -c "INSERT INTO blacklisted_numbers (phone, reason) VALUES ('13061129773287', 'spam') ON CONFLICT DO NOTHING;"

# Remove a blacklisted number
psql "$VERIFICATION_DATABASE_URL" -c "DELETE FROM blacklisted_numbers WHERE phone = '13061129773287';"

# List all blacklisted numbers
psql "$VERIFICATION_DATABASE_URL" -c "SELECT * FROM blacklisted_numbers;"
```

### Manual Contact Export

If the database is running in a Docker container (e.g., `whatsadk-db`), you can export the contact list to a text file:

```bash
docker exec -i whatsadk-db psql -U postgres -d <database_name> -c "SELECT * FROM whatsmeow_contacts;" > contacts.txt
```

## Model Context Protocol (MCP)

WhatsADK includes an MCP server that allows AI agents (like Claude Code, Cursor, and OpenCode) to interact with your WhatsApp contacts and blacklist directly.

### Tools Available:
- `blacklist_add`: Block a phone number/JID (Local Shadow Ban + Remote WhatsApp Block).
- `blacklist_remove`: Unblock a phone number/JID (Local Shadow Ban + Remote WhatsApp Unblock).
- `blacklist_get_remote`: Fetch the official blocklist from WhatsApp servers.
- `query_contacts`: Search for WhatsApp contacts by name or JID.
- `get_message_logs`: Retrieve recent message logs for a specific user.
- `send_message`: Send multi-modal messages (text/media).
- `filesys_sql_select`: Execute custom SELECT queries on message logs.
- `filesys_put`: Create or update entries in the virtual file system.
- `filesys_get`: Retrieve specific entries by path.
- `filesys_delete`: Remove entries from the file system.
- `filesys_list`: List entries with prefix filtering.

### Configuration for Claude Code / Claude Desktop:

Add the following to your `claude_desktop_config.json` or equivalent:

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "CONFIG_FILE": "/path/to/whatsadk/config/config.yaml"
      }
    }
  }
}
```

### Configuration for pi.dev (Pi Coding Agent):

Create or update `.pi/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "CONFIG_FILE": "/path/to/whatsadk/config/config.yaml"
      },
      "lifecycle": "lazy"
    }
  }
}
```

### Configuration for openCode:

Add to your `~/.config/open-code/mcp.json`:

```json
{
  "mcpServers": {
    "whatsadk": {
      "command": "/path/to/whatsadk/bin/whatsadk-mcp",
      "env": {
        "CONFIG_FILE": "/path/to/whatsadk/config/config.yaml"
      }
    }
  }
}
```

### Autonomous Mode (Agentic Control)

You can allow external agents (Claude Code, pi.dev, OpenCode) to drive WhatsApp conversations autonomously by disabling the default ADK response.

1. **Disable ADK** in `config/config.yaml`:
   ```yaml
   adk:
     enabled: false
   ```
2. **Start the Gateway**: `./bin/gateway`
3. **Prompt your Agent**:
   Give your agent (e.g., Claude Code) the following instruction:
   > "Monitor WhatsApp messages using `get_recent_messages`. If you see a new request from a user, process it and reply using `send_message`. You can send text and multi-modal media (base64)."

The agent will now act as the "brain" of your WhatsApp account, using the MCP bridge to read and write messages.

## Simulator & Testing

WhatsADK includes two simulators for testing the gateway flows without a physical device.

### 1. WhatsApp Simulator (Simulator)

Simulates the **WhatsApp ➔ Gateway ➔ ADK** flow. It presents a WhatsApp-like TUI, allowing you to send messages and media to your ADK agent.

- **TUI Interface**: Interactive chat-like interface.
- **Attachments**: Simulate sending images, audio, video, and documents via `/attach`.
- **Multimedia Reception**: Automatically saves media from the agent to `media_received/`.
- **Usage**: `./bin/simulator`

### 2. ADK Reverse Simulator (ADKSim)

Simulates the **Gateway ➔ ADK** flow. It acts as the ADK server, allowing you to manually provide the "AI" responses in a TUI. This is useful for testing how the gateway handles complex agent responses (e.g., specific media sequencing or slow responses).

- **HTTP Server**: Implements the ADK API (`/run`, `/run_sse`) on port 8080 (default).
- **Manual Response**: Type text or use `/attach <path>` to send files back to WhatsApp.
- **Inbound Media**: Saves incoming media from WhatsApp to `adk_media_received/`.
- **Usage**: `./bin/adksim -port 8080`

#### TUI Commands (both simulators)

| Command | Description |
|---------|-------------|
| `/help` | List all available commands |
| `/attach <path> [caption/mime]` | Attach a file to the next message |
| `/clear` | Clear the chat history |
| `/set_sender <phone>` | (WhatsApp Simulator only) Change simulated sender |
| `/export <filename>` | Export session to JSON |
| `/replay <filename>` | Replay session from JSON |

#### Export a Real WhatsApp Session

To debug a specific agent failure using a real conversation log:

```bash
./bin/simulator export <phone_number> <output_file.json>
```

The exported JSON can then be loaded into the simulator using `/replay` to reproduce and fix agentic workflow issues.

## Architecture

For a detailed architecture overview, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Dependencies

- [whatsmeow](https://github.com/tulir/whatsmeow) - WhatsApp Web multidevice API
- [ADK](https://github.com/google/adk-go) - Google Agent Development Kit (remote service)

## License

Apache 2.0
