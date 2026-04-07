package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open store db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping store db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate store db: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS blacklisted_numbers (
			phone TEXT PRIMARY KEY,
			reason TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS whatsmeow_contacts (
			our_jid TEXT NOT NULL,
			their_jid TEXT NOT NULL,
			full_name TEXT,
			short_name TEXT,
			push_name TEXT,
			business_name TEXT,
			PRIMARY KEY (our_jid, their_jid)
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS whatsmeow_commands (
			id SERIAL PRIMARY KEY,
			command TEXT NOT NULL,
			payload JSONB,
			status TEXT NOT NULL DEFAULT 'pending',
			result JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS filesys (
			path TEXT PRIMARY KEY,
			metadata JSONB,
			content BYTEA,
			tmstamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_filesys_metadata ON filesys USING GIN (metadata);
	`)
	return err
}

type Command struct {
	ID        int64           `json:"id"`
	Command   string          `json:"command"`
	Payload   json.RawMessage `json:"payload"`
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (s *Store) EnqueueCommand(ctx context.Context, cmd string, payload interface{}) (int64, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	var id int64
	err = s.db.QueryRowContext(ctx,
		"INSERT INTO whatsmeow_commands (command, payload, status) VALUES ($1, $2, 'pending') RETURNING id",
		cmd, payloadJSON,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("enqueue command: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateCommandStatus(ctx context.Context, id int64, status string, result interface{}) error {
	var resultJSON []byte
	var err error
	if result != nil {
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx,
		"UPDATE whatsmeow_commands SET status = $1, result = $2, updated_at = NOW() WHERE id = $3",
		status, resultJSON, id,
	)
	if err != nil {
		return fmt.Errorf("update command status: %w", err)
	}
	return nil
}

func (s *Store) PollPendingCommands(ctx context.Context) ([]Command, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, command, payload, status, result, created_at, updated_at FROM whatsmeow_commands WHERE status = 'pending' ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("poll pending commands: %w", err)
	}
	defer rows.Close()

	var commands []Command
	for rows.Next() {
		var c Command
		var payload, result []byte
		err := rows.Scan(&c.ID, &c.Command, &payload, &c.Status, &result, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan command row: %w", err)
		}
		c.Payload = payload
		c.Result = result
		commands = append(commands, c)
	}
	return commands, rows.Err()
}

func (s *Store) WaitForCommand(ctx context.Context, id int64, timeout time.Duration) (*Command, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			var c Command
			var payload, result []byte
			err := s.db.QueryRowContext(ctx,
				"SELECT id, command, payload, status, result, created_at, updated_at FROM whatsmeow_commands WHERE id = $1",
				id,
			).Scan(&c.ID, &c.Command, &payload, &c.Status, &result, &c.CreatedAt, &c.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("get command: %w", err)
			}
			c.Payload = payload
			c.Result = result

			if c.Status == "completed" || c.Status == "failed" {
				return &c, nil
			}
		}
	}
}

func (s *Store) PutFile(ctx context.Context, path string, metadata interface{}, content []byte, timestamp time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO filesys (path, metadata, content, tmstamp) 
		 VALUES ($1, $2, $3, $4) 
		 ON CONFLICT (path) DO UPDATE SET 
			metadata = EXCLUDED.metadata, 
			content = EXCLUDED.content, 
			tmstamp = EXCLUDED.tmstamp`,
		path, metadata, content, timestamp,
	)
	if err != nil {
		return fmt.Errorf("put file: %w", err)
	}
	return nil
}

func (s *Store) IsBlacklisted(ctx context.Context, phone string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM blacklisted_numbers WHERE phone = $1", phone,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return true, nil
}

func (s *Store) AddBlacklist(ctx context.Context, phone, reason string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO blacklisted_numbers (phone, reason, created_at) VALUES ($1, $2, $3) ON CONFLICT (phone) DO NOTHING",
		phone, reason, time.Now().UTC(),
	)
	return err
}

func (s *Store) RemoveBlacklist(ctx context.Context, phone string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM blacklisted_numbers WHERE phone = $1", phone,
	)
	return err
}

type BlacklistedNumber struct {
	Phone     string    `json:"phone"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListBlacklist(ctx context.Context) ([]BlacklistedNumber, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT phone, reason, created_at FROM blacklisted_numbers ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list blacklist: %w", err)
	}
	defer rows.Close()

	var numbers []BlacklistedNumber
	for rows.Next() {
		var n BlacklistedNumber
		if err := rows.Scan(&n.Phone, &n.Reason, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan blacklist row: %w", err)
		}
		numbers = append(numbers, n)
	}
	return numbers, rows.Err()
}

type Contact struct {
	OurJID       string `json:"our_jid"`
	TheirJID     string `json:"their_jid"`
	FullName     string `json:"full_name"`
	ShortName    string `json:"short_name"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
}

func (s *Store) ListContacts(ctx context.Context, query string) ([]Contact, error) {
	var rows *sql.Rows
	var err error

	if query == "" {
		rows, err = s.db.QueryContext(ctx,
			"SELECT our_jid, their_jid, full_name, short_name, push_name, business_name FROM whatsmeow_contacts ORDER BY full_name ASC LIMIT 100",
		)
	} else {
		q := "%" + query + "%"
		rows, err = s.db.QueryContext(ctx,
			`SELECT our_jid, their_jid, full_name, short_name, push_name, business_name 
			 FROM whatsmeow_contacts 
			 WHERE full_name ILIKE $1 OR push_name ILIKE $1 OR their_jid ILIKE $1 
			 ORDER BY full_name ASC LIMIT 100`,
			q,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var fullName, shortName, pushName, businessName sql.NullString
		err := rows.Scan(&c.OurJID, &c.TheirJID, &fullName, &shortName, &pushName, &businessName)
		if err != nil {
			return nil, fmt.Errorf("scan contact row: %w", err)
		}
		c.FullName = fullName.String
		c.ShortName = shortName.String
		c.PushName = pushName.String
		c.BusinessName = businessName.String
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

type FileEntry struct {
	Path      string          `json:"path"`
	Metadata  sql.NullString  `json:"metadata"`
	Content   []byte          `json:"content,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

func (s *Store) GetFilesysLogs(ctx context.Context, phone string, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	pathPattern := "whatsmeow/" + phone + "/%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, metadata, 
		        CASE WHEN (metadata->>'mime_type' = 'text/plain') THEN content ELSE NULL END as content,
		        tmstamp 
		 FROM filesys 
		 WHERE path LIKE $1 
		 ORDER BY tmstamp DESC 
		 LIMIT $2`,
		pathPattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get filesys logs: %w", err)
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		if err := rows.Scan(&e.Path, &e.Metadata, &e.Content, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan filesys row: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) GetLatestGlobalMessages(ctx context.Context, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	// Filter for request/response paths to avoid sync chatter
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, metadata, 
		        CASE WHEN (metadata->>'mime_type' = 'text/plain') THEN content ELSE NULL END as content,
		        tmstamp 
		 FROM filesys 
		 WHERE path LIKE 'whatsmeow/%/request' OR path LIKE 'whatsmeow/%/response'
		 ORDER BY tmstamp DESC 
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest global messages: %w", err)
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		if err := rows.Scan(&e.Path, &e.Metadata, &e.Content, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan filesys row: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) QueryFilesys(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
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
				// Try to treat as string if it looks like one, or keep as byte array
				entry[col] = string(b)
			} else {
				entry[col] = val
			}
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) GetFile(ctx context.Context, path string) (*FileEntry, error) {
	var e FileEntry
	err := s.db.QueryRowContext(ctx,
		"SELECT path, metadata, content, tmstamp FROM filesys WHERE path = $1",
		path,
	).Scan(&e.Path, &e.Metadata, &e.Content, &e.Timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	return &e, nil
}

func (s *Store) DeleteFile(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM filesys WHERE path = $1", path)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (s *Store) ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := "SELECT path, metadata, content, tmstamp FROM filesys"
	var args []interface{}
	if prefix != "" {
		query += " WHERE path LIKE $1"
		args = append(args, prefix+"%")
	}
	query += " ORDER BY tmstamp DESC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		if err := rows.Scan(&e.Path, &e.Metadata, &e.Content, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan filesys row: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
