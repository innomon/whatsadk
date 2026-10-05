# Technology Stack - WhatsADK

## Programming Language
- **Language:** Go 1.26.5+

## User Interface & TUI
- **TUI Framework:** Charm Bubble Tea (`github.com/charmbracelet/bubbletea`)
- **Styling & Layout:** Charm Lip Gloss (`github.com/charmbracelet/lipgloss`), Bubbles (`github.com/charmbracelet/bubbles`)
- **Rendering Support:** Plain text, Markdown rendering (`glamour` / Lip Gloss formatting), A2UI component rendering

## Database & Storage
- **Decentralized Storage:** Embedded pure-Go `sqlite-p2p` (P2P mesh replication, WAL mode, replication gating)
- **Relational Storage:** PostgreSQL (`github.com/lib/pq`)
- **Multi-Model Storage:** SurrealDB (`github.com/surrealdb/surrealdb.go`)

## Protocols & Integrations
- **WhatsApp Web Protocol:** `whatsmeow` (`go.mau.fi/whatsmeow`)
- **ADK / Agent Integration:** `google.golang.org/adk/v2`, `a2aproject/a2a-go`, `google.golang.org/genai`
- **MCP Server Protocol:** `github.com/modelcontextprotocol/go-sdk`
- **Auth & Cryptography:** RS256 JWT (`golang-jwt/jwt/v5`), Ed25519/EdDSA WhatsApp OAuth, `go-pear` access control

## CLI & Command Registry
- **Command Architecture:** Handcrafted command registries (strictly NO Cobra/pflag / `spf13` libraries)
