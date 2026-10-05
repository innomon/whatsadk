# WhatsADK Database Utility (`dbutil`)

`dbutil` is a standalone command-line utility for exporting, importing, backing up, and migrating WhatsADK database records across storage backends (PostgreSQL, SurrealDB, and SQLite-P2P).

Data is transferred using the streaming-friendly **JSON Lines (JSONL)** format, encoding binary assets (such as virtual file system logs and media attachments) into Base64 JSON envelopes.

---

## 🏗 Architecture & Features

- **Decentralized SQLite-P2P Integration**:
  - Automatically loads P2P node configuration and replication policies when `p2p.enabled: true` in configuration.
  - **Live Concurrency Fallback**: If an active WhatsADK gateway process is already listening on the configured P2P swarm port or holding hypercore feed locks, `dbutil` transparently falls back to opening the SQLite-P2P database in direct WAL mode (`sqlite-p2p://<path>`), guaranteeing uninterrupted backups and exports on live gateways.
- **Direct Database Access (`-db` and `-dsn`)**:
  - Export and import directly to/from any SQLite database file or connection DSN without requiring a `config.yaml` file.
- **Handcrafted Command Registry**:
  - Implemented entirely using Go's standard library and a custom registry without external frameworks like Cobra/Pflag.
  - Fully supports global flags (`-config`, `-db`, `-dsn`, `-help`) whether placed before or after the subcommand name.
- **Zero-Downtime Migration**:
  - Stream records from a PostgreSQL or SurrealDB production deployment directly into a localized SQLite-P2P mesh node, or vice versa.

---

## 🛠 Compilation

To build the executable:

```bash
# Compile to bin/dbutil
go build -o bin/dbutil ./cmd/dbutil
```

---

## 📖 CLI Usage

```text
Usage: dbutil [options] <command> [command options]

Commands:
  export   Export database contents to a JSONL file
  import   Import database contents from a JSONL file

Options:
  -config <path>   Path to config.yaml (default: auto-detected)
  -db <path>       Direct path to SQLite database file
  -dsn <url>       Database connection DSN (sqlite://, postgres://, surrealdb://)
  -help, -h        Show help

Use 'dbutil <command> -help' for command-specific options.
```

### Global Flags

| Flag | Description |
| :--- | :--- |
| `-config <path>` | Specifies a YAML configuration file path. If omitted, checks `CONFIG_FILE` environment variable, then `./config.yaml`, `./config/config.yaml`, and executable-relative paths. |
| `-db <path>` | Directly opens a SQLite or SQLite-P2P database file (e.g. `data/p2p_num1.db`) without requiring `config.yaml`. |
| `-dsn <url>` | Directly connects to a database DSN (e.g. `postgres://...`, `surrealdb://...`, or `sqlite://...`). |
| `-help`, `-h` | Displays usage instructions without attempting a database connection. |

---

## 🚀 Subcommands

### 1. `export`

Exports database tables into a JSONL file or standard output.

```bash
# Export using default config file to default output (export.jsonl)
./bin/dbutil export

# Export directly from a SQLite-P2P database file
./bin/dbutil export -db data/p2p_num1.db -out backup.jsonl

# Export using a direct DSN
./bin/dbutil export -dsn sqlite://data/p2p_num1.db -out backup.jsonl

# Export with a custom config file
./bin/dbutil export -config /config/config.yaml -out backup.jsonl
# (or with flag before subcommand)
./bin/dbutil -config /config/config.yaml export -out backup.jsonl

# Export to stdout for piping and compression
./bin/dbutil export -db data/p2p_num1.db -out - | gzip > backup.jsonl.gz
```

#### Export Options

- `-out <path>`: Destination path for exported JSONL data. Use `-` to stream to stdout (default: `export.jsonl`).

---

### 2. `import`

Imports records from a JSONL file or standard input into the target database. Automatically synchronizes command auto-increment counters via `ResetSequence`.

```bash
# Import into a SQLite-P2P database directly
./bin/dbutil import -db data/p2p_num2.db -in backup.jsonl

# Import from stdin (e.g. decompressing a backup archive)
gunzip -c backup.jsonl.gz | ./bin/dbutil import -db data/p2p_num2.db -in -

# Import using a custom config file
./bin/dbutil import -config /config/config.yaml -in backup.jsonl
```

#### Import Options

- `-in <path>`: Source path for JSONL data. Use `-` to read from stdin (default: `export.jsonl`).

---

## 📦 Supported Entities & Record Schemas

`dbutil` exports and restores four primary tables / virtual collections:

| Entity Type | Description | Underlying Table / View |
| :--- | :--- | :--- |
| `blacklist` | Blocked phone numbers and block rationale | `blacklisted_numbers` |
| `contact` | Synchronized WhatsApp contacts and rosters | `whatsmeow_contacts` |
| `command` | Outbound and pending WhatsApp agent command queue entries | `whatsmeow_commands` |
| `file` | Virtual file system logs (`filesys`), media assets, and payloads | `filesys` |

### JSONL Schema Examples

Each line in the exported file is a self-contained JSON envelope:

```json
{"type":"blacklist","data":{"phone":"919999999999","reason":"Spam activity","created_at":"2026-10-01T12:00:00Z"}}
{"type":"contact","data":{"our_jid":"me@s.whatsapp.net","their_jid":"user@s.whatsapp.net","full_name":"Alice","short_name":"Alice","push_name":"Alice","business_name":""}}
{"type":"command","data":{"id":1,"command":"send_message","payload":{"to":"user@s.whatsapp.net","body":"Hi"},"status":"done","result":{},"created_at":"2026-10-01T12:00:00Z","updated_at":"2026-10-01T12:00:01Z"}}
{"type":"file","data":{"path":"whatsmeow/user/MSG_ID/response","metadata":"{\"mime_type\":\"text/plain\"}","content":"SGVsbG8gV29ybGQh","timestamp":"2026-10-01T12:00:00Z"}}
```

---

## 🔄 Cross-Database Migration Example

### Migrating from PostgreSQL to SQLite-P2P

1. **Export from PostgreSQL:**

   ```bash
   ./bin/dbutil export -dsn "postgres://user:pass@localhost:5432/whatsadk?sslmode=disable" -out migration.jsonl
   ```

2. **Import into SQLite-P2P:**

   ```bash
   ./bin/dbutil import -db data/p2p_num1.db -in migration.jsonl
   ```

3. **Verify record count:**

   ```bash
   sqlite3 data/p2p_num1.db "SELECT count(*) FROM filesys;"
   ```

---

## 🧪 Testing

Run package tests:

```bash
go test -v ./cmd/dbutil/...
```

Unit tests cover:

- Standalone SQLite-P2P export and import cycles ([`main_test.go`](main_test.go)).
- Handcrafted Command Registry lookup and validation.
- CLI argument parsing (`-config`, `-db`, `-dsn`, `-help`) in prefix and postfix positions.
