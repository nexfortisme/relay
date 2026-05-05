package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", sqliteDSNWithPragmas(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Allow concurrent requests to use separate SQLite connections.
	// Combined with WAL + busy_timeout pragmas this helps avoid SQLITE_BUSY
	// during short-lived write contention.
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(12)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func sqliteDSNWithPragmas(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return path + separator +
		"_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=synchronous(NORMAL)"
}
