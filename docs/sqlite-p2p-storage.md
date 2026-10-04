# SQLite P2P Storage Backend

WhatsADK includes a native, decentralized, pure Go storage backend powered by `sqlite-p2p` and `go-pear` as an embedded alternative to PostgreSQL and SurrealDB.

---

## 1. DSN Schemes & Configuration

You can enable the SQLite P2P storage backend by configuring your `WHATSADK_STORE_DSN` or passing a compatible DSN to `store.Open(dsn)`:

| Scheme | Description | Example |
|---|---|---|
| `sqlite-p2p://` | Canonical SQLite P2P DSN | `sqlite-p2p://whatsadk.db?wal=true` |
| `sqlite://` | Standard SQLite path | `sqlite://data/whatsadk.db` |
| `p2p://` | P2P swarm connection | `p2p://cluster.db` |
| `pear://` | Pear/Holepunch swarm | `pear://peer.db` |
| In-Memory | Transient testing backend | `sqlite-p2p://:memory:` |

---

## 2. Replication Gating & Node Access Control

WhatsADK supports cryptographic node access control and replication gating via `go-pear/pkg/policy`. Peer-to-peer replication can be configured in three modes:

1. **`all` (Default)**: Open discovery and unrestricted peer replication.
2. **`whitelist`**: Strict replication access permitting only explicitly authorized peer public keys (`[32]byte` 64-character hexadecimal Ed25519 public keys).
3. **`blacklist`**: Permissive replication blocking explicitly unauthorized peer public keys.

### YAML Configuration (`config/config.yaml`)

```yaml
p2p:
  enabled: true
  node_id: "whatsadk-node-1"
  swarm_topic: "whatsadk-mesh-topic"
  swarm_port: 0
  bootstrap:
    - "192.168.1.100:43219"
  peer_addrs: []
  db_path: "data/whatsadk_p2p.db"
  enable_wal: true
  enable_crypto: false
  auto_sync: true
  replication:
    mode: "whitelist"  # "all", "whitelist", or "blacklist"
    whitelist:
      - "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    blacklist:
      - "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
```

### Environment Variable Overrides

- `P2P_ENABLED`: `true` or `false`
- `P2P_NODE_ID`: Unique node identifier string
- `P2P_SWARM_TOPIC`: Discovery topic
- `P2P_SWARM_PORT`: P2P listener port (0 for random)
- `P2P_DB_PATH`: Database file path
- `P2P_ENABLE_WAL`: `true` or `false`
- `P2P_ENABLE_CRYPTO`: `true` or `false`
- `P2P_AUTO_SYNC`: `true` or `false`
- `P2P_REPLICATION_MODE`: `all`, `whitelist`, or `blacklist`

---

## 3. Architecture & Dual-Database Roles

WhatsADK separates protocol-level WhatsApp session persistence from application-level mesh data:

```
┌─────────────────────────────────────────────────────────────┐
│                    WhatsADK Gateway Node                    │
│                                                             │
│   ┌──────────────────────┐       ┌──────────────────────┐   │
│   │   whatsmeow Client   │       │   WhatsADK Store     │   │
│   │   (WhatsApp Protocol)│       │   (App / Filesys)    │   │
│   └──────────┬───────────┘       └──────────┬───────────┘   │
│              │                              │               │
│              ▼                              ▼               │
│     [ store_dsn ]                     [ p2p.db_path ]       │
│   (Local Session DB)              (P2P Replicated Store)    │
│   - E2EE Signal Keys              - Virtual Filesys Logs    │
│   - Linked Device Token           - Shared Blacklists       │
│   - WhatsApp Pre-keys             - Router App Mappings     │
│   (Isolated per Phone)            - Contacts & Outbox       │
│                                             │               │
└─────────────────────────────────────────────┼───────────────┘
                                              │ Hyperswarm P2P
                                              ▼ (swarm_topic)
                                    [ Other Gateway Nodes ]
```

| Attribute | `whatsapp.store_dsn` | `p2p.db_path` / `verification.database_url` |
|---|---|---|
| **Owner / Layer** | [whatsmeow](https://github.com/tulir/whatsmeow) (WhatsApp Protocol Engine) | WhatsADK Gateway (`internal/store`) |
| **Data Stored** | Cryptographic session keys, Signal protocol identity keys, pre-keys, noise keys, device tokens. | Virtual filesystem logs (`filesys`), blacklists, contacts, command queues, router states. |
| **Replication Scope** | **Local only** (Private to that specific WhatsApp phone number). | **P2P Swarm Replicated** (Syncs across all nodes sharing the `swarm_topic`). |
| **Multi-Instance Rule** | **Must be distinct** per phone number to prevent session/key collision. | **Must be distinct file paths** per process on the same machine to prevent file locks. |

---

## 4. SQL Compatibility Views

The backend stores all WhatsApp entities inside a unified, generic key-value table (`crm_store`) and projects them via high-performance SQLite views:

- `filesys`: Virtual filesystem logs and binary media (JPEG, PNG, WebP, MP4, OGG, WAV).
- `whatsmeow_contacts`: Contact roster entities.
- `whatsmeow_commands`: Asynchronous command execution queue.
- `blacklisted_numbers`: Spam and moderation phone blacklist.

---

## 5. Code Example

```go
package main

import (
	"context"
	"log"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
)

func main() {
	ctx := context.Background()

	// Load configuration with P2P gating
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Open store with P2P node configuration and replication policy
	st, err := store.OpenP2PFromNodeConfig(cfg.P2P.ToNodeConfig())
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	// Enqueue asynchronous WhatsApp command
	cmdID, err := st.EnqueueCommand(ctx, "send_message", map[string]string{
		"phone": "+15551234567",
		"text":  "Hello from WhatsaDK SQLite P2P!",
	})
	if err != nil {
		log.Fatalf("failed to enqueue command: %v", err)
	}
	log.Printf("Enqueued command ID: %d", cmdID)
}
```
