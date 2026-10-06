# SQLite P2P Storage Backend

WhatsADK includes a native, decentralized, pure Go storage backend powered by `sqlite-p2p` and `go-pear` as an embedded alternative to PostgreSQL and SurrealDB.

---

## 1. DSN Schemes & Configuration

You can enable the SQLite P2P storage backend by configuring your `WHATSADK_STORE_DSN` or passing a compatible DSN to `store.Open(dsn)`:

| Scheme | Description | Example |
| --- | --- | --- |
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

## 3. Network Topologies (LAN, Multi-Host, WAN & VPN)

WhatsADK's P2P storage mesh connects instances across local machines, local networks (LAN), or the public internet (WAN) using pure Go Hyperswarm and Noise XX encryption.

### Topology 1: Same Machine (Multiple WhatsApp Numbers)

When running multiple gateway instances on the same host (e.g. to serve multiple numbers concurrently), configure distinct TCP swarm ports and connect via localhost:

- **Gateway 1 (`config_num1.yaml`)**:

  ```yaml
  p2p:
    enabled: true
    node_id: "gateway-node-1"
    swarm_topic: "whatsadk-mesh-topic"
    swarm_port: 43211
    db_path: "data/p2p_num1.db"
  ```

* **Gateway 2 (`config_num2.yaml`)**:

  ```yaml
  p2p:
    enabled: true
    node_id: "gateway-node-2"
    swarm_topic: "whatsadk-mesh-topic"
    swarm_port: 43212
    peer_addrs:
      - "127.0.0.1:43211"
    db_path: "data/p2p_num2.db"
  ```

### Topology 2: Same LAN / Local Wi-Fi (Different Computers)

When running gateways on separate computers connected to the same local network / router:

- **Zero-Config LAN Auto-Discovery**:
  Every node automatically broadcasts and listens for UDP discovery beacons on port `49736`. Nodes sharing the same `swarm_topic` detect each other's LAN IP (e.g., `192.168.1.50:43211`) and peer automatically without manual configuration.
- **Deterministic Direct Address (Recommended for static subnets)**:
  Point directly to the LAN IP of the target machine:

  ```yaml
  p2p:
    peer_addrs:
      - "192.168.1.50:43211"
  ```

### Topology 3: Different Networks / Over the Internet (WAN & Remote Cloud)

When gateways are located on different networks (e.g., Raspberry Pi at home syncing with a cloud server or office):

#### Option A: Mesh VPN (Tailscale / WireGuard) — Recommended

Install Tailscale or WireGuard on both machines. They receive private virtual IPs (e.g., `100.x.y.z`). Add the remote peer's mesh IP to `peer_addrs`:

```yaml
p2p:
  peer_addrs:
    - "100.64.0.15:43211"
```

Replication occurs over the private encrypted WireGuard tunnel with zero port forwarding required.

#### Option B: Public DHT Bootstrap Node (`dht-seed`)

Run a lightweight Hyperswarm rendezvous seed on a public server / VPS:

```bash
./dht-seed -port 43210
```

Then configure both remote gateways with the public seed address in `bootstrap`:

```yaml
p2p:
  bootstrap:
    - "203.0.113.10:43210"
```

Both gateways register on the DHT topic. Hyperswarm then coordinates NAT hole punching (UDX) to establish a direct, end-to-end encrypted Noise XX session between the two remote machines.

---

## 4. Architecture & Dual-Database Roles

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
| --- | --- | --- |
| **Owner / Layer** | [whatsmeow](https://github.com/tulir/whatsmeow) (WhatsApp Protocol Engine) | WhatsADK Gateway (`internal/store`) |
| **Data Stored** | Cryptographic session keys, Signal protocol identity keys, pre-keys, noise keys, device tokens. | Virtual filesystem logs (`filesys`), blacklists, contacts, command queues, router states. |
| **Replication Scope** | **Local only** (Private to that specific WhatsApp phone number). | **P2P Swarm Replicated** (Syncs across all nodes sharing the `swarm_topic`). |
| **Multi-Instance Rule** | **Must be distinct** per phone number to prevent session/key collision. | **Must be distinct file paths** per process on the same machine to prevent file locks. |

---

## 5. SQL Compatibility Views

The backend stores all WhatsApp entities inside a unified, generic key-value table (`crm_store`) and projects them via high-performance SQLite views:

- `filesys`: Virtual filesystem logs and binary media (JPEG, PNG, WebP, MP4, OGG, WAV).
- `whatsmeow_contacts`: Contact roster entities.
- `whatsmeow_commands`: Asynchronous command execution queue.
- `blacklisted_numbers`: Spam and moderation phone blacklist.

---

## 6. Code Example

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
