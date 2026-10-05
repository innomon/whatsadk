# Specification: Interactive Command TUI & MCP UI with SQLite-P2P Swarm

## Track Overview
Build a terminal user interface (`cmd/tui` / `bin/tui`) powered by Charm (Bubble Tea / Lip Gloss) that provides an interactive, multi-format (Plain text, Markdown, A2UI) shell for executing handcrafted commands, SQL queries, and all registered WhatsADK MCP tools over an embedded bidirectional SQLite-P2P swarm connection.

## Key Functional Requirements

1. **Command Execution & Input Parsing:**
   - **Direct Terminal Input:** First word is case-insensitive command name; subsequent words are parameters.
   - **Slash Modal Input (`/`):** Typing `/` pops up a modal box displaying command help, parameter input fields, and prefilled/dynamic prompt controls.
   - Direct commands and slash modal commands invoke the exact same handcrafted handlers.

2. **Handcrafted Command Registry & Dynamic Param Resolver:**
   - Handcrafted command registry (strictly NO Cobra/spf13/pflag).
   - Configured via `config.yaml` command schema (`name`, `handler`, `help`, `params`: `name`, `type`, `value`).
   - Dynamically prompts the user for any parameters missing values.

3. **MCP Tools & SQL Handlers:**
   - Exposes all MCP tools (`send_message`, `query_contacts`, `jid_to_phone`, `list_groups`, `list_group_members`, `get_recent_messages`, `filesys_sql_select`, `blacklist_add`, `blacklist_remove`, etc.) as first-class TUI commands.
   - Direct SQL execution handler (`sql`) for local and P2P SQLite tables.

4. **Multi-Format Output Rendering:**
   - Renders output in **Plain text**, **Markdown**, and **A2UI** (Agent-to-User Interface) structured components.

5. **SQLite-P2P Swarm Integration:**
   - Starts embedded `sqlite-p2p` from `config.yaml`, joins swarm topic, and maintains bidirectional replication.
