# Product Guidelines - WhatsADK

## Design & UI Principles
- **Terminal First & Aesthetic TUI:** All CLI and terminal user interfaces MUST use clean, high-contrast layouts powered by Charm (Bubble Tea, Lip Gloss) with intuitive keyboard navigation.
- **Clear Multi-Format Display:** Support rendering plain text, A2UI structured components, and rich Markdown formatting.
- **Explicit Operational Feedback:** Provide immediate status indicators, clear progress messages, and descriptive error messages for all commands and network operations.

## Architecture & Code Principles
- **Handcrafted Registries:** Never use Cobra/pflag (spf13) in Go projects. Implement handcrafted command registries for CLI flags, subcommands, and slash commands.
- **Decentralized First:** Design for peer-to-peer mesh storage (`sqlite-p2p`) enabling independent gateway and agent operations across heterogeneous networks.
- **Workspace-Root Relative Paths:** Maintain clean relative paths for doc links, script references, and graph nodes.
- **Single Canonical Documentation:** Keep central documentation (e.g. `README.md`, `cmd/mcp/AGENT.md`) up to date after any functional or architectural changes.

## Security & Reliability Guidelines
- **Zero Hardcoded Secrets:** Configuration must be supplied via `config.yaml` or environment variables.
- **Explicit Error Handling:** Never ignore errors with `_`. Always return early and log actionable context.
- **Clean Fallbacks:** Provide clear fallback behaviors when remote nodes, gateway instances, or network streams time out.
