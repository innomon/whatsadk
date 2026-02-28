# Architecture

This document describes the architecture of **WhatsADK**, a Go gateway that bridges WhatsApp messaging with Google's Agent Development Kit (ADK) services.

## High-Level Overview

```
┌──────────────┐          ┌──────────────────────────────┐          ┌───────────────┐
│  WhatsApp    │◀────────▶│        WhatsADK Gateway      │────────▶│  ADK Agent    │
│  Users       │  whatsmeow│                              │  HTTP   │  Service      │
│              │  (WebSocket)│  ┌────────┐  ┌───────────┐ │  REST   │  (Remote)     │
└──────────────┘          │  │PostgreSQL│  │ JWT Auth  │ │  /SSE   └───────────────┘
                          │  │ Session  │  │ (RS256)   │ │
                          │  └────────┘  └───────────┘ │
                          │       ┌──────────────┐      │          ┌───────────────┐
                          │       │ Verification │──────│─────────▶│  3rd-Party    │
                          │       │ Handler      │      │ Callback │  Apps         │
                          │       └──────────────┘      │          └───────────────┘
                          └──────────────────────────────┘
```

The gateway is a single long-running process that connects to WhatsApp via the whatsmeow library, listens for incoming messages, and proxies them to a remote ADK agent over HTTP. Responses from the agent are relayed back to the WhatsApp user.

## Directory Structure

```
whatsadk/
├── cmd/
│   ├── gateway/main.go              # Application entry point & dependency wiring
│   └── keygen/main.go               # Ed25519 key pair generator for OAuth
├── internal/
│   ├── config/config.go             # YAML configuration loader with env overrides
│   ├── whatsapp/client.go           # WhatsApp client, QR auth, message routing
│   ├── agent/client.go              # ADK HTTP client (REST & SSE modes)
│   ├── auth/
│   │   ├── claims.go                # JWT custom claims struct (user_id, channel)
│   │   ├── jwt.go                   # RS256 JWT token generator
│   │   ├── eddsa.go                 # Ed25519 key loading (PEM/seed)
│   │   ├── oauth_token.go           # EdDSA JWT generator for OAuth flow
│   │   ├── oauth_handler.go         # AUTH command parser, rate limiter, deep link builder
│   │   ├── key_registry.go          # Public key registry for app verification
│   │   └── verify_token.go          # Verification token detection & validation
│   └── verification/
│       └── handler.go               # Reverse OTP verification handler
├── config/config.yaml               # Default configuration file
└── docs/                            # Design docs and reference material
```

## Package Responsibilities

### `cmd/gateway` — Entry Point

The `main.go` file orchestrates startup:

1. Loads configuration via `config.Load()`
2. Optionally initializes `auth.JWTGenerator` (RS256 signing)
3. Optionally initializes `verification.Handler` (reverse OTP)
4. Optionally initializes `auth.OAuthHandler` (EdDSA WhatsApp OAuth)
5. Creates `agent.Client` (ADK HTTP client)
6. Creates `whatsapp.Client`, connects, and enters the run loop

All dependencies are wired manually — no DI framework is used.

### `internal/config` — Configuration

Loads a `config.yaml` file with a well-defined search order:

1. `-config` CLI flag
2. `CONFIG_FILE` environment variable
3. `./config.yaml`
4. `./config/config.yaml`
5. Executable directory (`config.yaml` or `config/config.yaml`)

After loading YAML, applies sensible defaults and overrides from environment variables (`ADK_ENDPOINT`, `ADK_APP_NAME`, `ADK_API_KEY`, etc.).

**Key config structs:** `Config`, `WhatsAppConfig`, `ADKConfig`, `AuthConfig`, `VerificationConfig`

### `internal/whatsapp` — WhatsApp Client

Wraps the [whatsmeow](https://github.com/tulir/whatsmeow) library to provide:

- **QR code authentication** — displays QR in terminal on first run
- **Persistent sessions** — stored in PostgreSQL via `sqlstore`
- **Message event handling** — routes incoming messages through a pipeline:
  1. Ignores messages from self and group chats
  2. Extracts text from conversation or extended text messages
  3. Checks for verification tokens → delegates to `verification.Handler`
  4. Checks for AUTH commands → delegates to `auth.OAuthHandler` (if enabled)
  5. Applies access control (whitelist + India country code filter)
  6. Forwards to `agent.Client.Chat()` and sends the response back
- **Graceful shutdown** — listens for `SIGINT`/`SIGTERM`

### `internal/agent` — ADK Client

HTTP client for the ADK Agent service. Supports two modes:

- **`/run` (synchronous)** — POSTs a `RunRequest`, receives a JSON array of `Event` objects
- **`/run_sse` (streaming)** — POSTs to `/run_sse` with `Accept: text/event-stream`, parses SSE `data:` lines

Both modes extract the final model response from the event list (last non-partial `model` event). The client also manages per-user sessions via `POST /apps/{app}/users/{user}/sessions/{session}`.

Authentication is layered: JWT (RS256) takes priority over static API key.

### `internal/auth` — Authentication

#### JWT Generator (`jwt.go`)

Generates short-lived RS256 tokens with custom claims:

- `user_id` — WhatsApp sender's phone number
- `channel` — always `"whatsapp"`
- Standard claims: `iss`, `aud`, `iat`, `exp`

Supports both default audience (`Token()`) and per-audience tokens (`TokenWithAudience()`) used for verification callbacks.

#### Key Registry (`key_registry.go`)

Loads RSA public keys for registered third-party apps from PEM files. Used by the verification subsystem to validate incoming verification tokens.

#### OAuth (EdDSA) Authentication (`eddsa.go`, `oauth_token.go`, `oauth_handler.go`)

Provides WhatsApp-based OAuth login using Ed25519/EdDSA-signed JWTs:

- `LoadEdDSAKey()` — loads an Ed25519 private key from PEM (PKCS#8) or raw 32-byte seed
- `OAuthTokenGenerator` — creates EdDSA JWTs with `sub` (phone), `iss`, `aud`, `nonce`, and `pubkey` claims
- `OAuthHandler` — parses `AUTH <pubkey> <nonce>` WhatsApp messages, validates the public key, enforces per-phone rate limits (default 5/hour), and returns a deep link containing the signed JWT

The resulting JWT is ~350 characters — compact enough for WhatsApp URL delivery.

#### Verification Token Detection (`verify_token.go`)

- `IsVerificationToken()` — quick heuristic check: starts with `eyJ`, has 3 dot-separated parts, and contains the required claims (`mobile`, `app_name`, `callback_url`)
- `VerifyVerificationToken()` — full cryptographic verification using the app's registered public key

### `internal/verification` — Reverse OTP

Handles the reverse OTP verification flow:

1. Detects verification tokens in incoming WhatsApp messages
2. Looks up the app's public key from `KeyRegistry`
3. Cryptographically verifies the token signature and expiry
4. Validates the sender's phone number matches the `mobile` claim
5. Signs a callback JWT (with audience set to the app name)
6. POSTs the signed JWT to the app's `callback_url`
7. Returns a user-facing confirmation/error message

## Data Flow

### Standard Message Flow

```
WhatsApp User
    │
    ▼ (message via whatsmeow WebSocket)
whatsapp.Client.handleMessage()
    │
    ├─ Skip: from self, group chat, empty text
    │
    ├─ Check: Is it a verification token? → verification.Handler
    │
    ├─ Check: Is it an AUTH command? → auth.OAuthHandler (EdDSA JWT → deep link)
    │
    ├─ Check: Is user allowed? (whitelist / +91 prefix)
    │
    ▼
agent.Client.Chat()
    │
    ├─ EnsureSession() → POST /apps/{app}/users/{user}/sessions/{session}
    │
    ├─ chatRun()    → POST /run      (if streaming=false)
    │  or
    ├─ chatSSE()    → POST /run_sse  (if streaming=true)
    │
    ▼
extractFinalResponse() → last non-partial model event
    │
    ▼
whatsapp.Client → SendMessage() back to user
```

### Reverse OTP Verification Flow

```
3rd-Party App                     WhatsApp User                Gateway
    │                                  │                          │
    ├─ Generate signed JWT ──────────▶ │                          │
    │  (mobile, app_name,              │                          │
    │   callback_url, challenge_id)    │                          │
    │                                  ├─ Send JWT as message ──▶ │
    │                                  │                          ├─ Detect token (IsVerificationToken)
    │                                  │                          ├─ Lookup app public key
    │                                  │                          ├─ Verify signature & expiry
    │                                  │                          ├─ Match sender phone vs claim
    │                                  │                          ├─ Sign callback JWT
    │  ◀─── POST callback_url ────────────────────────────────────┤
    │       (Bearer: signed JWT)       │                          │
    │                                  │ ◀── Confirmation msg ────┤
```

### WhatsApp OAuth Flow

```
SPA (Browser)                  WhatsApp User             Gateway                    ADK Server
    │                               │                       │                          │
    ├─ Generate Ed25519 key pair    │                       │                          │
    ├─ Generate nonce               │                       │                          │
    ├─ Open wa.me deep link ──────▶ │                       │                          │
    │  AUTH <pubkey> <nonce>         │                       │                          │
    │                               ├─ Send message ──────▶ │                          │
    │                               │                       ├─ Parse AUTH command       │
    │                               │                       ├─ Validate pubkey (32B)    │
    │                               │                       ├─ Check rate limit         │
    │                               │                       ├─ Sign EdDSA JWT           │
    │                               │ ◀── Deep link reply ──┤                          │
    │                               │  <SPA>/auth#token=JWT │                          │
    │ ◀── User clicks link ─────── │                       │                          │
    ├─ Parse JWT + nonce from #     │                       │                          │
    ├─ Store JWT                    │                       │                          │
    ├─ Authorization: Bearer JWT ──────────────────────────────────────────────────────▶│
    │                               │                       │                          ├─ Verify EdDSA sig
    │ ◀────────────────────────────────────────────────────────────────── API response ─┤
```

## Key Dependencies

| Dependency | Purpose |
|---|---|
| [whatsmeow](https://github.com/tulir/whatsmeow) | WhatsApp Web multi-device API (WebSocket) |
| [lib/pq](https://github.com/lib/pq) | PostgreSQL driver for WhatsApp session persistence |
| [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) | RS256 & EdDSA JWT token generation and parsing |
| [qrterminal](https://github.com/mdp/qrterminal) | QR code rendering in terminal |
| [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | YAML configuration parsing |
| [protobuf](https://pkg.go.dev/google.golang.org/protobuf) | WhatsApp message protocol buffer serialization |

## Security Model

- **JWT Auth (RS256)** — asymmetric signing ensures the ADK service can verify requests without sharing the private key. Tokens are short-lived (default 2 minutes).
- **OAuth (EdDSA)** — Ed25519-signed JWTs (~350 chars) for WhatsApp deep-link delivery. The JWT binds the user's phone number to the SPA's ephemeral public key. Rate-limited to 5 AUTH requests per phone per hour.
- **API Key fallback** — when JWT is not configured, a static API key can be used (less secure, suitable for development).
- **Verification token validation** — incoming tokens are cryptographically verified against pre-registered app public keys. Phone number matching prevents token forwarding attacks.
- **Access control** — users are filtered by whitelist or India country code prefix. Non-allowed users receive a rejection message.

## Build & Test

```bash
# Build
go build -o whatsadk ./cmd/gateway

# Run all tests
go test ./...

# Run specific test
go test -run TestName ./internal/auth/
```
