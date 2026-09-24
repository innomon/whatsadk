package store

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

var dollarParamRegex = regexp.MustCompile(`\$[0-9]+`)

// GetFilesysLogs retrieves filesys message logs for a specific phone number.
func (b *Backend) GetFilesysLogs(ctx context.Context, phone string, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	pathPattern := "whatsmeow/" + phone + "/%"
	query := `
		SELECT path, metadata, 
		       CASE WHEN (metadata->>'mime_type' = 'text/plain') THEN content ELSE NULL END as content,
		       tmstamp 
		FROM filesys 
		WHERE path LIKE ? 
		ORDER BY tmstamp DESC 
		LIMIT ?
	`

	rows, err := b.db.QueryContext(ctx, query, pathPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("get filesys logs: %w", err)
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

// GetLatestGlobalMessages retrieves the latest global message entries (request/response).
func (b *Backend) GetLatestGlobalMessages(ctx context.Context, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT path, metadata, 
		       CASE WHEN (metadata->>'mime_type' = 'text/plain') THEN content ELSE NULL END as content,
		       tmstamp 
		FROM filesys 
		WHERE path LIKE 'whatsmeow/%/request' OR path LIKE 'whatsmeow/%/response'
		ORDER BY tmstamp DESC 
		LIMIT ?
	`

	rows, err := b.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get latest global messages: %w", err)
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

// QueryFilesys executes an arbitrary SQL query against the filesys view.
// It normalizes Postgres-style $1, $2 placeholders to SQLite ? placeholders.
func (b *Backend) QueryFilesys(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	normalizedQuery := dollarParamRegex.ReplaceAllString(query, "?")

	rows, err := b.db.QueryContext(ctx, normalizedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		entry := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				entry[col] = string(b)
			} else {
				entry[col] = val
			}
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}
