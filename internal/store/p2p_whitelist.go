package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const whitelistKeyPrefix = "whatsadk:whitelist:"

func whitelistKey(phone string) string {
	return whitelistKeyPrefix + phone
}

// IsWhitelisted returns true if the phone number is present in the whitelist.
func (b *Backend) IsWhitelisted(ctx context.Context, phone string) (bool, error) {
	var exists int
	err := b.db.QueryRowContext(ctx,
		"SELECT 1 FROM whitelisted_numbers WHERE phone = ?", phone,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check whitelist: %w", err)
	}
	return true, nil
}

// AddWhitelist records a phone number on the whitelist with a reason.
func (b *Backend) AddWhitelist(ctx context.Context, phone, reason string) error {
	item := WhitelistedNumber{
		Phone:     phone,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	}
	rawMeta, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal whitelist: %w", err)
	}
	return b.putRecord(ctx, whitelistKey(phone), rawMeta, nil)
}

// RemoveWhitelist removes a phone number from the whitelist.
func (b *Backend) RemoveWhitelist(ctx context.Context, phone string) error {
	return b.deleteRecord(ctx, whitelistKey(phone))
}

// ListWhitelist returns all whitelisted phone numbers.
func (b *Backend) ListWhitelist(ctx context.Context) ([]WhitelistedNumber, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT phone, reason, created_at FROM whitelisted_numbers ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list whitelist: %w", err)
	}
	defer rows.Close()

	var numbers []WhitelistedNumber
	for rows.Next() {
		var n WhitelistedNumber
		var createdAtStr sql.NullString
		if err := rows.Scan(&n.Phone, &n.Reason, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scan whitelist row: %w", err)
		}
		if createdAtStr.Valid {
			n.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr.String)
			if n.CreatedAt.IsZero() {
				n.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr.String)
			}
		}
		numbers = append(numbers, n)
	}
	return numbers, rows.Err()
}
