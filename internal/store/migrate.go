package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const supportedSchemaVersion = 2

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS inspection_batches (
		id TEXT PRIMARY KEY, venue_name TEXT NOT NULL, stage_zone TEXT NOT NULL,
		performance_at TEXT NOT NULL, owner_name TEXT NOT NULL, status TEXT NOT NULL,
		version INTEGER NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS rigging_points (
		id TEXT PRIMARY KEY, batch_id TEXT NOT NULL REFERENCES inspection_batches(id),
		point_code TEXT NOT NULL, hoist_serial TEXT NOT NULL, rope_spec TEXT NOT NULL,
		rated_load_kg REAL NOT NULL, planned_load_kg REAL NOT NULL, position_note TEXT NOT NULL,
		locked_at TEXT, UNIQUE(batch_id, point_code), UNIQUE(batch_id, hoist_serial))`,
	`CREATE TABLE IF NOT EXISTS load_tests (
		id TEXT PRIMARY KEY, batch_id TEXT NOT NULL REFERENCES inspection_batches(id),
		rigging_point_id TEXT NOT NULL REFERENCES rigging_points(id), attempt_no INTEGER NOT NULL,
		target_load_kg REAL NOT NULL, measured_load_kg REAL NOT NULL, hold_seconds INTEGER NOT NULL,
		displacement_mm REAL NOT NULL, result TEXT NOT NULL, recorded_by TEXT NOT NULL, recorded_at TEXT NOT NULL,
		UNIQUE(rigging_point_id, attempt_no))`,
	`CREATE TABLE IF NOT EXISTS deviations (
		id TEXT PRIMARY KEY, batch_id TEXT NOT NULL REFERENCES inspection_batches(id),
		load_test_id TEXT NOT NULL REFERENCES load_tests(id), rigging_point_id TEXT NOT NULL REFERENCES rigging_points(id),
		severity TEXT NOT NULL, symptom TEXT NOT NULL, required_action TEXT NOT NULL, assignee TEXT NOT NULL,
		remediation_evidence TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, closed_by_test_id TEXT REFERENCES load_tests(id), closed_at TEXT,
		assignee_confirmed INTEGER NOT NULL DEFAULT 0, confirmed_by TEXT NOT NULL DEFAULT '', confirmed_at TEXT)`,
	`CREATE TABLE IF NOT EXISTS frozen_snapshots (
		id TEXT PRIMARY KEY, batch_id TEXT NOT NULL UNIQUE REFERENCES inspection_batches(id),
		digest TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS technical_reviews (
		id TEXT PRIMARY KEY, batch_id TEXT NOT NULL UNIQUE REFERENCES inspection_batches(id),
		approved_by TEXT NOT NULL, approval_note TEXT NOT NULL, point_count INTEGER NOT NULL,
		tested_point_count INTEGER NOT NULL, passed_point_count INTEGER NOT NULL,
		closed_deviation_count INTEGER NOT NULL, reviewed_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS credential_sequence (id INTEGER PRIMARY KEY AUTOINCREMENT)`,
	`CREATE TABLE IF NOT EXISTS release_credentials (
		id TEXT PRIMARY KEY, batch_id TEXT NOT NULL UNIQUE REFERENCES inspection_batches(id),
		serial_number INTEGER NOT NULL UNIQUE, snapshot_digest TEXT NOT NULL, approved_by TEXT NOT NULL,
		approval_note TEXT NOT NULL, issued_at TEXT NOT NULL, verification_status TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS idempotency_results (
		idempotency_key TEXT PRIMARY KEY, action TEXT NOT NULL, batch_id TEXT NOT NULL,
		response_json BLOB NOT NULL, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS audit_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT, batch_id TEXT NOT NULL REFERENCES inspection_batches(id),
		action TEXT NOT NULL, batch_version INTEGER NOT NULL, occurred_at TEXT NOT NULL)`,
	`CREATE TRIGGER IF NOT EXISTS frozen_snapshots_no_update BEFORE UPDATE ON frozen_snapshots BEGIN SELECT RAISE(ABORT,'frozen snapshot is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS frozen_snapshots_no_delete BEFORE DELETE ON frozen_snapshots BEGIN SELECT RAISE(ABORT,'frozen snapshot is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS credentials_no_update BEFORE UPDATE ON release_credentials BEGIN SELECT RAISE(ABORT,'credential is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS credentials_no_delete BEFORE DELETE ON release_credentials BEGIN SELECT RAISE(ABORT,'credential is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS load_tests_no_update BEFORE UPDATE ON load_tests BEGIN SELECT RAISE(ABORT,'load test is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS load_tests_no_delete BEFORE DELETE ON load_tests BEGIN SELECT RAISE(ABORT,'load test is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS audit_no_update BEFORE UPDATE ON audit_events BEGIN SELECT RAISE(ABORT,'audit event is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS audit_no_delete BEFORE DELETE ON audit_events BEGIN SELECT RAISE(ABORT,'audit event is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS technical_reviews_no_update BEFORE UPDATE ON technical_reviews BEGIN SELECT RAISE(ABORT,'technical review is immutable'); END`,
	`CREATE TRIGGER IF NOT EXISTS technical_reviews_no_delete BEFORE DELETE ON technical_reviews BEGIN SELECT RAISE(ABORT,'technical review is immutable'); END`,
	`CREATE INDEX IF NOT EXISTS idx_points_batch ON rigging_points(batch_id)`,
	`CREATE INDEX IF NOT EXISTS idx_tests_batch ON load_tests(batch_id)`,
	`CREATE INDEX IF NOT EXISTS idx_deviations_batch ON deviations(batch_id)`,
}

func (s *SQLite) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, schemaStatements[0]); err != nil {
		return err
	}
	var version int
	err = tx.QueryRowContext(ctx, "SELECT version FROM schema_meta LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_meta(version) VALUES(0)"); err != nil {
			return err
		}
		version = 0
	} else if err != nil {
		return err
	}
	if version > supportedSchemaVersion {
		return fmt.Errorf("数据库 schemaVersion %d 高于程序支持的 %d", version, supportedSchemaVersion)
	}
	for _, statement := range schemaStatements[1:] {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行迁移: %w", err)
		}
	}
	if version < 2 {
		// Existing version-1 databases need the new responsibility confirmation fields.
		for _, column := range []string{"assignee_confirmed INTEGER NOT NULL DEFAULT 0", "confirmed_by TEXT NOT NULL DEFAULT ''", "confirmed_at TEXT"} {
			if _, e := tx.ExecContext(ctx, "ALTER TABLE deviations ADD COLUMN "+column); e != nil {
				if !strings.Contains(e.Error(), "duplicate column") {
					return e
				}
			}
		}
	}
	if version < supportedSchemaVersion {
		if _, err = tx.ExecContext(ctx, "UPDATE schema_meta SET version=?", supportedSchemaVersion); err != nil {
			return err
		}
	}
	return tx.Commit()
}
