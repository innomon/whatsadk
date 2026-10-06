# Specification: TUI SQLite-P2P Swarm Sync & Co-located DB Discovery

## Track Overview
Fix P2P replication and sync between the WhatsADK TUI (`cmd/tui`) and running gateway instances (including multi-instance setups like `p2p_num1.db` and `p2p_num2.db` in staging environments).

## Key Functional Requirements

1. **P2P Swarm Synchronization & Peer Discovery:**
   - Ensure TUI joins the matching P2P `swarm_topic` and connects to active gateway nodes (via localhost/LAN peer discovery or `peer_addrs`).
   - Enable automated two-way replication so commands enqueued by TUI (`send_message`, `blacklist_add`, etc.) and database changes from both gateway instances replicate to TUI in real-time.

2. **Co-Located Database Auto-Detection & Multi-Store Fallback:**
   - If P2P network swarm port or feed lock is detected on the local host (co-located in `/home/innomon/staging/whatsadk/`), TUI automatically detects existing SQLite P2P database files (`data/p2p_num1.db`, `data/p2p_num2.db`, `data/whatsadk_p2p.db`).
   - Opens direct SQLite WAL mode connection allowing safe concurrent reads and command enqueueing across all active local instances.

3. **Status Bar & Mesh Sync Visibility:**
   - Display real-time P2P sync status, connected peer count, and active DB backend mode (`P2P Mesh (N Peers)` or `Direct Local WAL (p2p_num1.db)`) in the TUI status bar.
