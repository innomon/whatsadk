# Specification: MCP FileSys SQL & CRUD Tools

This specification defines new Model Context Protocol (MCP) tools for interacting with the `filesys` table in the WhatsADK gateway. These tools allow for flexible querying and direct management of the persistent message and media storage.

## 1. Overview
The `filesys` table is a key-value store where the `path` is the primary key. It stores:
- **Metadata**: JSONB object containing mime-type, sender JID, message ID, etc.
- **Content**: BYTEA (binary data) for message text or media files.
- **Timestamp**: When the entry was created/updated.

## 2. Tool Definitions

### 2.1. `filesys_sql_select`
Executes a raw SQL `SELECT` query against the `filesys` table.

- **Name**: `filesys_sql_select`
- **Description**: Execute a custom SELECT query on the filesys table for advanced filtering and aggregation.
- **Arguments**:
  - `query` (string, required): The SQL SELECT statement.
- **Constraints**:
  - The query MUST start with `SELECT` (case-insensitive).
  - The tool will return a list of maps representing the rows.

### 2.2. `filesys_put`
Upserts (creates or updates) an entry in the `filesys` table.

- **Name**: `filesys_put`
- **Description**: Create or update a file/entry in the virtual file system.
- **Arguments**:
  - `path` (string, required): The unique path (e.g., `whatsmeow/12345/request`).
  - `metadata` (object, optional): A JSON object of metadata.
  - `content` (string, optional): The content of the file. If it's a media file, this should be base64 encoded.
- **Behavior**: Uses `ON CONFLICT (path) DO UPDATE`.

### 2.3. `filesys_get`
Retrieves a single entry from the `filesys` table by its path.

- **Name**: `filesys_get`
- **Description**: Read a file's metadata and content from the filesys.
- **Arguments**:
  - `path` (string, required): The exact path to retrieve.

### 2.4. `filesys_delete`
Removes an entry from the `filesys` table.

- **Name**: `filesys_delete`
- **Description**: Delete an entry from the filesys.
- **Arguments**:
  - `path` (string, required): The exact path to delete.

### 2.5. `filesys_list`
Lists entries with an optional path prefix.

- **Name**: `filesys_list`
- **Description**: List files in the filesys with optional prefix filtering.
- **Arguments**:
  - `prefix` (string, optional): Filter paths starting with this string (e.g., `whatsmeow/`).
  - `limit` (number, optional): Maximum number of entries to return (default 50).

## 3. Security Considerations
- The `filesys_sql_select` tool is restricted to `SELECT` statements to prevent unauthorized `UPDATE`, `DELETE`, or `DROP` operations via this specific tool.
- All operations are performed using the application's database credentials.
