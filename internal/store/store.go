package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type storeBackend interface {
	Close() error
	EnqueueCommand(ctx context.Context, cmd string, payload interface{}) (int64, error)
	UpdateCommandStatus(ctx context.Context, id int64, status string, result interface{}) error
	PollPendingCommands(ctx context.Context) ([]Command, error)
	WaitForCommand(ctx context.Context, id int64, timeout time.Duration) (*Command, error)
	PutFile(ctx context.Context, path string, metadata interface{}, content []byte, timestamp time.Time) error
	IsBlacklisted(ctx context.Context, phone string) (bool, error)
	AddBlacklist(ctx context.Context, phone, reason string) error
	RemoveBlacklist(ctx context.Context, phone string) error
	ListBlacklist(ctx context.Context) ([]BlacklistedNumber, error)
	ListContacts(ctx context.Context, query string) ([]Contact, error)
	GetFilesysLogs(ctx context.Context, phone string, limit int) ([]FileEntry, error)
	GetLatestGlobalMessages(ctx context.Context, limit int) ([]FileEntry, error)
	QueryFilesys(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error)
	GetFile(ctx context.Context, path string) (*FileEntry, error)
	DeleteFile(ctx context.Context, path string) error
	ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error)
	GetAllContacts(ctx context.Context) ([]Contact, error)
	PutContact(ctx context.Context, contact Contact) error
	GetAllCommands(ctx context.Context) ([]Command, error)
	PutCommand(ctx context.Context, cmd Command) error
	GetAllFiles(ctx context.Context) ([]FileEntry, error)
	ResetSequence(ctx context.Context) error
}

type Store struct {
	backend storeBackend
}

type sqlStore struct {
	db *sql.DB
}

func IsSurrealDB(dsn string) bool {
	return strings.HasPrefix(dsn, "surrealdb://") ||
		strings.HasPrefix(dsn, "ws://") ||
		strings.HasPrefix(dsn, "wss://") ||
		strings.HasPrefix(dsn, "http://") ||
		strings.HasPrefix(dsn, "https://")
}

func Open(dsn string) (*Store, error) {
	if IsSurrealDB(dsn) {
		backend, err := openSurrealDB(dsn)
		if err != nil {
			return nil, err
		}
		return &Store{backend: backend}, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open store db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping store db: %w", err)
	}

	s := &sqlStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate store db: %w", err)
	}

	return &Store{backend: s}, nil
}

func (s *Store) Close() error {
	return s.backend.Close()
}

func (s *sqlStore) Close() error {
	return s.db.Close()
}

func (s *Store) EnqueueCommand(ctx context.Context, cmd string, payload interface{}) (int64, error) {
	return s.backend.EnqueueCommand(ctx, cmd, payload)
}

func (s *Store) UpdateCommandStatus(ctx context.Context, id int64, status string, result interface{}) error {
	return s.backend.UpdateCommandStatus(ctx, id, status, result)
}

func (s *Store) PollPendingCommands(ctx context.Context) ([]Command, error) {
	return s.backend.PollPendingCommands(ctx)
}

func (s *Store) WaitForCommand(ctx context.Context, id int64, timeout time.Duration) (*Command, error) {
	return s.backend.WaitForCommand(ctx, id, timeout)
}

func (s *Store) PutFile(ctx context.Context, path string, metadata interface{}, content []byte, timestamp time.Time) error {
	return s.backend.PutFile(ctx, path, metadata, content, timestamp)
}

func (s *Store) IsBlacklisted(ctx context.Context, phone string) (bool, error) {
	return s.backend.IsBlacklisted(ctx, phone)
}

func (s *Store) AddBlacklist(ctx context.Context, phone, reason string) error {
	return s.backend.AddBlacklist(ctx, phone, reason)
}

func (s *Store) RemoveBlacklist(ctx context.Context, phone string) error {
	return s.backend.RemoveBlacklist(ctx, phone)
}

func (s *Store) ListBlacklist(ctx context.Context) ([]BlacklistedNumber, error) {
	return s.backend.ListBlacklist(ctx)
}

func (s *Store) ListContacts(ctx context.Context, query string) ([]Contact, error) {
	return s.backend.ListContacts(ctx, query)
}

func (s *Store) GetFilesysLogs(ctx context.Context, phone string, limit int) ([]FileEntry, error) {
	return s.backend.GetFilesysLogs(ctx, phone, limit)
}

func (s *Store) GetLatestGlobalMessages(ctx context.Context, limit int) ([]FileEntry, error) {
	return s.backend.GetLatestGlobalMessages(ctx, limit)
}

func (s *Store) DatabaseType() string {
	switch s.backend.(type) {
	case *sqlStore:
		return "postgres"
	case *surrealStore:
		return "surrealdb"
	default:
		return "unknown"
	}
}

func (s *Store) QueryFilesys(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	return s.backend.QueryFilesys(ctx, query, args...)
}

func (s *Store) GetFile(ctx context.Context, path string) (*FileEntry, error) {
	return s.backend.GetFile(ctx, path)
}

func (s *Store) DeleteFile(ctx context.Context, path string) error {
	return s.backend.DeleteFile(ctx, path)
}

func (s *Store) ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error) {
	return s.backend.ListFiles(ctx, prefix, limit)
}

func (s *Store) GetAllContacts(ctx context.Context) ([]Contact, error) {
	return s.backend.GetAllContacts(ctx)
}

func (s *Store) PutContact(ctx context.Context, contact Contact) error {
	return s.backend.PutContact(ctx, contact)
}

func (s *Store) GetAllCommands(ctx context.Context) ([]Command, error) {
	return s.backend.GetAllCommands(ctx)
}

func (s *Store) PutCommand(ctx context.Context, cmd Command) error {
	return s.backend.PutCommand(ctx, cmd)
}

func (s *Store) GetAllFiles(ctx context.Context) ([]FileEntry, error) {
	return s.backend.GetAllFiles(ctx)
}

func (s *Store) ResetSequence(ctx context.Context) error {
	return s.backend.ResetSequence(ctx)
}

func (s *sqlStore) migrate() error {
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

func (s *sqlStore) EnqueueCommand(ctx context.Context, cmd string, payload interface{}) (int64, error) {
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

func (s *sqlStore) UpdateCommandStatus(ctx context.Context, id int64, status string, result interface{}) error {
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

func (s *sqlStore) PollPendingCommands(ctx context.Context) ([]Command, error) {
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

func (s *sqlStore) WaitForCommand(ctx context.Context, id int64, timeout time.Duration) (*Command, error) {
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

func (s *sqlStore) PutFile(ctx context.Context, path string, metadata interface{}, content []byte, timestamp time.Time) error {
	var metadataJSON interface{}
	var err error
	if metadata != nil {
		switch m := metadata.(type) {
		case []byte:
			metadataJSON = m
		case string:
			metadataJSON = m
		default:
			metadataJSON, err = json.Marshal(metadata)
			if err != nil {
				return fmt.Errorf("marshal metadata: %w", err)
			}
		}
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO filesys (path, metadata, content, tmstamp) 
		 VALUES ($1, $2, $3, $4) 
		 ON CONFLICT (path) DO UPDATE SET 
			metadata = EXCLUDED.metadata, 
			content = EXCLUDED.content, 
			tmstamp = EXCLUDED.tmstamp`,
		path, metadataJSON, content, timestamp,
	)
	if err != nil {
		return fmt.Errorf("put file: %w", err)
	}
	return nil
}

func (s *sqlStore) IsBlacklisted(ctx context.Context, phone string) (bool, error) {
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

func (s *sqlStore) AddBlacklist(ctx context.Context, phone, reason string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO blacklisted_numbers (phone, reason, created_at) VALUES ($1, $2, $3) ON CONFLICT (phone) DO NOTHING",
		phone, reason, time.Now().UTC(),
	)
	return err
}

func (s *sqlStore) RemoveBlacklist(ctx context.Context, phone string) error {
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

func (s *sqlStore) ListBlacklist(ctx context.Context) ([]BlacklistedNumber, error) {
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

func (s *sqlStore) ListContacts(ctx context.Context, query string) ([]Contact, error) {
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
	Path      string         `json:"path"`
	Metadata  sql.NullString `json:"metadata"`
	Content   []byte         `json:"content,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

func (s *sqlStore) GetFilesysLogs(ctx context.Context, phone string, limit int) ([]FileEntry, error) {
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

func (s *sqlStore) GetLatestGlobalMessages(ctx context.Context, limit int) ([]FileEntry, error) {
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

func (s *sqlStore) QueryFilesys(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
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

func (s *sqlStore) GetFile(ctx context.Context, path string) (*FileEntry, error) {
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

func (s *sqlStore) DeleteFile(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM filesys WHERE path = $1", path)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (s *sqlStore) ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error) {
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

func (s *sqlStore) GetAllContacts(ctx context.Context) ([]Contact, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT our_jid, their_jid, full_name, short_name, push_name, business_name FROM whatsmeow_contacts ORDER BY full_name ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("get all contacts: %w", err)
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

func (s *sqlStore) PutContact(ctx context.Context, c Contact) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO whatsmeow_contacts (our_jid, their_jid, full_name, short_name, push_name, business_name)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (our_jid, their_jid) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			short_name = EXCLUDED.short_name,
			push_name = EXCLUDED.push_name,
			business_name = EXCLUDED.business_name`,
		c.OurJID, c.TheirJID,
		sqlNullString(c.FullName), sqlNullString(c.ShortName),
		sqlNullString(c.PushName), sqlNullString(c.BusinessName),
	)
	return err
}

func (s *sqlStore) GetAllCommands(ctx context.Context) ([]Command, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, command, payload, status, result, created_at, updated_at FROM whatsmeow_commands ORDER BY id ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("get all commands: %w", err)
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

func (s *sqlStore) PutCommand(ctx context.Context, c Command) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO whatsmeow_commands (id, command, payload, status, result, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
			command = EXCLUDED.command,
			payload = EXCLUDED.payload,
			status = EXCLUDED.status,
			result = EXCLUDED.result,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at`,
		c.ID, c.Command, []byte(c.Payload), c.Status, []byte(c.Result), c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (s *sqlStore) GetAllFiles(ctx context.Context) ([]FileEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT path, metadata, content, tmstamp FROM filesys ORDER BY tmstamp ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("get all files: %w", err)
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

func (s *sqlStore) ResetSequence(ctx context.Context) error {
	var maxID sql.NullInt64
	err := s.db.QueryRowContext(ctx, "SELECT MAX(id) FROM whatsmeow_commands").Scan(&maxID)
	if err != nil {
		return err
	}
	if !maxID.Valid {
		return nil
	}
	// PostgreSQL: Set sequence next value to max(id) + 1
	_, err = s.db.ExecContext(ctx,
		fmt.Sprintf("SELECT setval(pg_get_serial_sequence('whatsmeow_commands', 'id'), %d)", maxID.Int64),
	)
	return err
}

func sqlNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
