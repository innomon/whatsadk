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
	return err
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
	Phone     string
	Reason    string
	CreatedAt time.Time
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
