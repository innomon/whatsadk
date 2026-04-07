# Implementation Plan: MCP FileSys SQL & CRUD Tools

This plan details the steps to implement the SQL SELECT and CRUD tools for the `filesys` table in the WhatsADK project.

## 1. Research & Analysis
- [x] Identified `internal/store/store.go` as the core database access layer.
- [x] Analyzed `cmd/mcp/main.go` for tool registration.
- [x] Confirmed the `filesys` table schema: `path` (PK), `metadata` (JSONB), `content` (BYTEA), `tmstamp`.

## 2. Store Layer Enhancements (`internal/store/store.go`)
- [x] Add `QueryFilesys(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error)`:
    - Should handle dynamic columns based on the query result.
- [x] Add `GetFile(ctx context.Context, path string) (*FileEntry, error)`:
    - Fetches a single record.
- [x] Add `DeleteFile(ctx context.Context, path string) error`:
    - Deletes a single record.
- [x] Add `ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error)`:
    - Lists files with a prefix filter.

## 3. MCP Handler Logic (`cmd/mcp/main.go`)
- [x] Implement `FileSysSQLSelect(ctx, store, query)` handler.
    - Check if the query starts with `SELECT`.
    - Return rows in JSON format.
- [x] Implement `FileSysPut(ctx, store, args)` handler.
    - Path, Metadata, Content.
    - Use `store.PutFile`.
- [x] Implement `FileSysGet(ctx, store, path)` handler.
- [x] Implement `FileSysDelete(ctx, store, path)` handler.
- [x] Implement `FileSysList(ctx, store, prefix, limit)` handler.

## 4. MCP Tool Registration (`cmd/mcp/main.go`)
- [x] Register `filesys_sql_select` with `query` arg.
- [x] Register `filesys_put` with `path`, `metadata`, `content` args.
- [x] Register `filesys_get` with `path` arg.
- [x] Register `filesys_delete` with `path` arg.
- [x] Register `filesys_list` with `prefix`, `limit` args.

## 5. Verification
- [x] Build check: `go build ./...` passed.
- [ ] Test the `filesys_sql_select` tool with a simple query.
- [ ] Test CRUD operations (Put -> Get -> List -> Delete).
- [ ] Verify security constraint (Rejection of `UPDATE`/`DELETE` queries).
