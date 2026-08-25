package store

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"time"
)

type SQLite struct {
	db  *sql.DB
	now func() time.Time
}

func Open(ctx context.Context, path string) (*SQLite, error) {
	dsn := path
	if path == "" {
		dsn = "file:rigging-release.db"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键: %w", err)
	}
	if _, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用 WAL: %w", err)
	}
	s := &SQLite{db: db, now: time.Now}
	if err = s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err = s.validateDatabase(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLite) Close() error                   { return s.db.Close() }
func (s *SQLite) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
