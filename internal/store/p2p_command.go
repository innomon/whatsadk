package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const commandKeyPrefix = "whatsadk:command:"

func commandKey(id int64) string {
	return fmt.Sprintf("%s%d", commandKeyPrefix, id)
}

func (b *Backend) getCommand(ctx context.Context, id int64) (*Command, error) {
	var c Command
	var payloadStr, resultStr sql.NullString
	var createdAtStr, updatedAtStr string
	err := b.db.QueryRowContext(ctx,
		"SELECT id, command, payload, status, result, created_at, updated_at FROM whatsmeow_commands WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Command, &payloadStr, &c.Status, &resultStr, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}
	if payloadStr.Valid {
		c.Payload = json.RawMessage(payloadStr.String)
	}
	if resultStr.Valid {
		c.Result = json.RawMessage(resultStr.String)
	}
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			c.CreatedAt = t
		} else if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			c.CreatedAt = t
		}
	}
	if updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			c.UpdatedAt = t
		} else if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			c.UpdatedAt = t
		}
	}
	return &c, nil
}

// EnqueueCommand adds a new command in pending status and returns its generated ID.
func (b *Backend) EnqueueCommand(ctx context.Context, cmd string, payload interface{}) (int64, error) {
	var payloadBytes []byte
	var err error
	if payload != nil {
		switch p := payload.(type) {
		case []byte:
			payloadBytes = p
		case string:
			payloadBytes = []byte(p)
		case json.RawMessage:
			payloadBytes = p
		default:
			payloadBytes, err = json.Marshal(payload)
			if err != nil {
				return 0, fmt.Errorf("marshal command payload: %w", err)
			}
		}
	}

	id := b.cmdSeq.Add(1)
	now := time.Now().UTC()

	c := Command{
		ID:        id,
		Command:   cmd,
		Payload:   payloadBytes,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}

	rawMeta, err := json.Marshal(c)
	if err != nil {
		return 0, fmt.Errorf("marshal command metadata: %w", err)
	}

	if err := b.putRecord(ctx, commandKey(id), rawMeta, nil); err != nil {
		return 0, fmt.Errorf("enqueue command: %w", err)
	}

	return id, nil
}

// UpdateCommandStatus updates the status and optional result for a command.
func (b *Backend) UpdateCommandStatus(ctx context.Context, id int64, status string, result interface{}) error {
	c, err := b.getCommand(ctx, id)
	if err != nil {
		return fmt.Errorf("command not found: %w", err)
	}

	var resultBytes []byte
	if result != nil {
		switch r := result.(type) {
		case []byte:
			resultBytes = r
		case string:
			resultBytes = []byte(r)
		case json.RawMessage:
			resultBytes = r
		default:
			resultBytes, err = json.Marshal(result)
			if err != nil {
				return fmt.Errorf("marshal command result: %w", err)
			}
		}
	}

	c.Status = status
	c.Result = resultBytes
	c.UpdatedAt = time.Now().UTC()

	rawMeta, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal updated command: %w", err)
	}

	return b.putRecord(ctx, commandKey(id), rawMeta, nil)
}

// PollPendingCommands fetches all commands currently in 'pending' status ordered by creation timestamp.
func (b *Backend) PollPendingCommands(ctx context.Context) ([]Command, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT id, command, payload, status, result, created_at, updated_at FROM whatsmeow_commands WHERE status = 'pending' ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("poll pending commands: %w", err)
	}
	defer rows.Close()

	var commands []Command
	for rows.Next() {
		var c Command
		var payloadStr, resultStr sql.NullString
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&c.ID, &c.Command, &payloadStr, &c.Status, &resultStr, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan command row: %w", err)
		}
		if payloadStr.Valid {
			c.Payload = json.RawMessage(payloadStr.String)
		}
		if resultStr.Valid {
			c.Result = json.RawMessage(resultStr.String)
		}
		if createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
				c.CreatedAt = t
			} else if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				c.CreatedAt = t
			}
		}
		if updatedAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
				c.UpdatedAt = t
			} else if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				c.UpdatedAt = t
			}
		}
		commands = append(commands, c)
	}
	return commands, rows.Err()
}

// WaitForCommand blocks until the command reaches 'completed' or 'failed' status or timeout occurs.
func (b *Backend) WaitForCommand(ctx context.Context, id int64, timeout time.Duration) (*Command, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if c, err := b.getCommand(ctx, id); err == nil && (c.Status == "completed" || c.Status == "failed") {
		return c, nil
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			c, err := b.getCommand(ctx, id)
			if err != nil {
				continue
			}
			if c.Status == "completed" || c.Status == "failed" {
				return c, nil
			}
		}
	}
}

// PutCommand writes or overwrites a command record directly.
func (b *Backend) PutCommand(ctx context.Context, cmd Command) error {
	if cmd.ID > b.cmdSeq.Load() {
		b.cmdSeq.Store(cmd.ID)
	}
	rawMeta, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}
	return b.putRecord(ctx, commandKey(cmd.ID), rawMeta, nil)
}

// GetAllCommands retrieves all commands ordered by ID ascending.
func (b *Backend) GetAllCommands(ctx context.Context) ([]Command, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT id, command, payload, status, result, created_at, updated_at FROM whatsmeow_commands ORDER BY id ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("get all commands: %w", err)
	}
	defer rows.Close()

	var commands []Command
	for rows.Next() {
		var c Command
		var payloadStr, resultStr sql.NullString
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&c.ID, &c.Command, &payloadStr, &c.Status, &resultStr, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan command row: %w", err)
		}
		if payloadStr.Valid {
			c.Payload = json.RawMessage(payloadStr.String)
		}
		if resultStr.Valid {
			c.Result = json.RawMessage(resultStr.String)
		}
		if createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
				c.CreatedAt = t
			} else if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				c.CreatedAt = t
			}
		}
		if updatedAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
				c.UpdatedAt = t
			} else if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				c.UpdatedAt = t
			}
		}
		commands = append(commands, c)
	}
	return commands, rows.Err()
}

// ResetSequence sets the command sequence counter to the maximum existing command ID.
func (b *Backend) ResetSequence(ctx context.Context) error {
	var maxID sql.NullInt64
	err := b.db.QueryRowContext(ctx, "SELECT MAX(id) FROM whatsmeow_commands").Scan(&maxID)
	if err != nil {
		return err
	}
	if maxID.Valid {
		b.cmdSeq.Store(maxID.Int64)
	} else {
		b.cmdSeq.Store(0)
	}
	return nil
}
