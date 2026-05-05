package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) GetSetting(ctx context.Context, userID, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE user_id = ? AND key = ?`, userID, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) UpsertSetting(ctx context.Context, userID, key, value string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(user_id, key, value, updated_at) VALUES(?, ?, ?, ?)
		 ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		userID, key, value, now,
	)
	if err != nil {
		return fmt.Errorf("upsert setting %s: %w", key, err)
	}
	return nil
}

func (s *Store) GetAllSettings(ctx context.Context, userID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		result[key] = value
	}
	return result, rows.Err()
}

// CopyDefaultSettings seeds a per-user copy of the default global settings on
// registration, so each new user starts with a private settings row they can
// edit independently.
func (s *Store) CopyDefaultSettings(ctx context.Context, userID string, defaults map[string]string) error {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed settings: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for k, v := range defaults {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings(user_id, key, value, updated_at) VALUES(?, ?, ?, ?)
			 ON CONFLICT(user_id, key) DO NOTHING`,
			userID, k, v, now,
		); err != nil {
			return fmt.Errorf("Error Seeding Setting %s: %w", k, err)
		}
	}
	return tx.Commit()
}
