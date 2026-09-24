package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const filesysKeyPrefix = "whatsadk:filesys:"

func filesysKey(path string) string {
	return filesysKeyPrefix + path
}

func (b *Backend) putRecord(ctx context.Context, key string, metadata json.RawMessage, data []byte) error {
	if b.engine != nil {
		return b.engine.PutLocal(ctx, key, metadata, data)
	}
	return b.tracker.Put(ctx, key, metadata, data)
}

func (b *Backend) deleteRecord(ctx context.Context, key string) error {
	if b.engine != nil {
		return b.engine.DeleteLocal(ctx, key)
	}
	return b.tracker.Delete(ctx, key)
}

// PutFile stores a virtual file entry into crm_store with metadata and timestamp.
func (b *Backend) PutFile(ctx context.Context, path string, metadata interface{}, content []byte, timestamp time.Time) error {
	tsStr := timestamp.UTC().Format(time.RFC3339Nano)

	var metaMap map[string]interface{}
	if metadata == nil {
		metaMap = map[string]interface{}{
			"_tmstamp": tsStr,
			"_is_null": 1,
		}
	} else {
		switch m := metadata.(type) {
		case []byte:
			if len(m) > 0 {
				_ = json.Unmarshal(m, &metaMap)
			}
		case string:
			if len(m) > 0 {
				_ = json.Unmarshal([]byte(m), &metaMap)
			}
		case map[string]interface{}:
			metaMap = make(map[string]interface{}, len(m)+1)
			for k, v := range m {
				metaMap[k] = v
			}
		default:
			marshaled, err := json.Marshal(metadata)
			if err != nil {
				return fmt.Errorf("marshal metadata: %w", err)
			}
			_ = json.Unmarshal(marshaled, &metaMap)
		}

		if metaMap == nil {
			metaMap = make(map[string]interface{})
		}
		metaMap["_tmstamp"] = tsStr
	}

	rawMeta, err := json.Marshal(metaMap)
	if err != nil {
		return fmt.Errorf("encode filesys metadata: %w", err)
	}

	return b.putRecord(ctx, filesysKey(path), rawMeta, content)
}

// GetFile retrieves a single file entry by its path.
func (b *Backend) GetFile(ctx context.Context, path string) (*FileEntry, error) {
	var e FileEntry
	var tsStr string
	err := b.db.QueryRowContext(ctx,
		"SELECT path, metadata, content, tmstamp FROM filesys WHERE path = ?",
		path,
	).Scan(&e.Path, &e.Metadata, &e.Content, &tsStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	if tsStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
			e.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
			e.Timestamp = t
		}
	}

	return &e, nil
}

// DeleteFile removes a file entry from storage.
func (b *Backend) DeleteFile(ctx context.Context, path string) error {
	return b.deleteRecord(ctx, filesysKey(path))
}

// ListFiles lists files with an optional path prefix and limit, ordered latest first.
func (b *Backend) ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := "SELECT path, metadata, content, tmstamp FROM filesys"
	var args []interface{}
	if prefix != "" {
		query += " WHERE path LIKE ?"
		args = append(args, prefix+"%")
	}
	query += " ORDER BY tmstamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		var tsStr string
		if err := rows.Scan(&e.Path, &e.Metadata, &e.Content, &tsStr); err != nil {
			return nil, fmt.Errorf("scan filesys row: %w", err)
		}
		if tsStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				e.Timestamp = t
			} else if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				e.Timestamp = t
			}
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetAllFiles retrieves all files ordered by timestamp ascending.
func (b *Backend) GetAllFiles(ctx context.Context) ([]FileEntry, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT path, metadata, content, tmstamp FROM filesys ORDER BY tmstamp ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("get all files: %w", err)
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		var tsStr string
		if err := rows.Scan(&e.Path, &e.Metadata, &e.Content, &tsStr); err != nil {
			return nil, fmt.Errorf("scan filesys row: %w", err)
		}
		if tsStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				e.Timestamp = t
			} else if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				e.Timestamp = t
			}
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
