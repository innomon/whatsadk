# WhatsADK Examples Directory

This directory contains standalone examples demonstrating different ADK Agent integrations with the WhatsADK Gateway.

---

## Example Projects

| Directory | Description | Integration Type |
|---|---|---|
| [`hello/`](hello/README.md) | Simple deterministic agent responding to greetings with a capability list. | ADK REST API (`/api/run`) |
| [`ignore/`](ignore/README.md) | Silent Ignore agent filtering non-whitelisted users without sending WhatsApp responses. | ADK REST API with `application/x-adk-silent-ignore` |
| [`router/`](router/README.md) | Intelligent router agent with multi-app disambiguation, LLM classification, and in-process execution. | In-Process Runner & HTTP Proxy |
| [`aigen-gateway/`](aigen-gateway/README.md) | Direct gateway mapping to AIGenApp multi-agent runtime with RS256 JWT auth and SSE streaming. | REST & SSE Streaming (`/api/adk2app/run_sse`) |

---

## Embedded SQLite P2P Storage

All examples are pre-configured to use WhatsADK's pure Go, embedded **`sqlite-p2p`** storage backend:

- **DSN Format:** `sqlite-p2p://data/whatsadk_p2p.db?wal=true`
- **Zero Server Setup:** No external PostgreSQL or SurrealDB instances are needed.
- **Working Directory Resolution:** Relative paths like `data/whatsadk_p2p.db` are created relative to the working directory where the gateway is executed.
  - From repository root: `./data/whatsadk_p2p.db`
  - From inside an example folder: `./examples/<name>/data/whatsadk_p2p.db`
- **Files Generated in WAL Mode:**
  - `whatsadk_p2p.db`: Main SQLite database (unified `crm_store` and session credentials).
  - `whatsadk_p2p.db-wal`: SQLite Write-Ahead Log.
  - `whatsadk_p2p.db-shm`: SQLite Shared-Memory index.
- **Automatic Directory Creation:** Any missing parent directories (such as `data/`) are automatically created at startup.
- **Absolute Paths:** Supply an absolute path (e.g., `sqlite-p2p:///var/lib/whatsadk/whatsadk_p2p.db?wal=true`) for production or fixed deployments.

---

## Dual-Database Architecture (`store_dsn` vs `p2p.db_path`)

When configuring WhatsADK, two database storage settings serve distinct purposes:

1. **`whatsapp.store_dsn` (WhatsApp Protocol & E2E Encryption):**
   - Managed by `whatsmeow`.
   - Stores cryptographic session tokens, Signal double-ratchet keys, and device credentials.
   - **Scope:** Local and private to that specific WhatsApp phone number. Must be kept in a separate file per phone.

2. **`p2p.db_path` / `verification.database_url` (Gateway Application Store):**
   - Managed by WhatsADK's embedded `sqlite-p2p` backend.
   - Stores virtual file system audit logs (`filesys`), blacklists, contact rosters, and routing state.
   - **Scope:** Replicated across all cluster nodes sharing the same `swarm_topic`.

