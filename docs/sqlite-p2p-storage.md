# SQLite P2P Storage Backend

WhatsaDK includes a native, decentralized, pure Go storage backend powered by [sqlite-p2p](file:///home/innomon/B204-zone/owly-sewa/sqlite-p2p) as an embedded alternative to PostgreSQL and SurrealDB.

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

## 2. Architecture & SQL Compatibility Views

The backend stores all WhatsApp entities inside a unified, generic key-value table (`crm_store`) and projects them via high-performance SQLite views:

- `filesys`: Virtual filesystem logs and binary media (JPEG, PNG, WebP, MP4, OGG, WAV).
- `whatsmeow_contacts`: Contact roster entities.
- `whatsmeow_commands`: Asynchronous command execution queue.
- `blacklisted_numbers`: Spam and moderation phone blacklist.

---

## 3. Code Example

```go
package main

import (
	"context"
	"log"

	"github.com/innomon/whatsadk/internal/store"
)

func main() {
	ctx := context.Background()

	// Open embedded SQLite P2P store
	st, err := store.Open("sqlite-p2p://whatsadk.db?wal=true")
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
