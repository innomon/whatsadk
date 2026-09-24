package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const blacklistKeyPrefix = "whatsadk:blacklist:"

func blacklistKey(phone string) string {
	return blacklistKeyPrefix + phone
}

// IsBlacklisted returns true if the phone number is present in the blacklist.
func (b *Backend) IsBlacklisted(ctx context.Context, phone string) (bool, error) {
	var exists int
	err := b.db.QueryRowContext(ctx,
		"SELECT 1 FROM blacklisted_numbers WHERE phone = ?", phone,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return true, nil
}

// AddBlacklist records a phone number on the blacklist with a reason.
func (b *Backend) AddBlacklist(ctx context.Context, phone, reason string) error {
	item := BlacklistedNumber{
		Phone:     phone,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	}
	rawMeta, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal blacklist: %w", err)
	}
	return b.putRecord(ctx, blacklistKey(phone), rawMeta, nil)
}

// RemoveBlacklist removes a phone number from the blacklist.
func (b *Backend) RemoveBlacklist(ctx context.Context, phone string) error {
	return b.deleteRecord(ctx, blacklistKey(phone))
}

// ListBlacklist returns all blacklisted numbers ordered by created_at descending.
func (b *Backend) ListBlacklist(ctx context.Context) ([]BlacklistedNumber, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT phone, reason, created_at FROM blacklisted_numbers ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list blacklist: %w", err)
	}
	defer rows.Close()

	var numbers []BlacklistedNumber
	for rows.Next() {
		var n BlacklistedNumber
		var tsStr string
		if err := rows.Scan(&n.Phone, &n.Reason, &tsStr); err != nil {
			return nil, fmt.Errorf("scan blacklist row: %w", err)
		}
		if tsStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				n.CreatedAt = t
			} else if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				n.CreatedAt = t
			}
		}
		numbers = append(numbers, n)
	}
	return numbers, rows.Err()
}
