package store

import (
	"context"
	"database/sql"
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
		`SELECT path, metadata, content, tmstamp 
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
