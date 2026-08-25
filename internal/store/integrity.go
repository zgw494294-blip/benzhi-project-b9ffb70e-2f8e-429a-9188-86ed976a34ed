package store

import (
	"context"
	"fmt"
	"stage-rigging-release/internal/domain"
)

func (s *SQLite) validateDatabase(ctx context.Context) error {
	var quickCheck string
	if err := s.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&quickCheck); err != nil {
		return fmt.Errorf("执行 SQLite quick_check: %w", err)
	}
	if quickCheck != "ok" {
		return fmt.Errorf("%w: SQLite quick_check 返回 %s", domain.ErrCorrupt, quickCheck)
	}
	foreignRows, err := s.db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("执行外键检查: %w", err)
	}
	if foreignRows.Next() {
		var table, parent string
		var rowID, foreignKeyID int64
		if scanErr := foreignRows.Scan(&table, &rowID, &parent, &foreignKeyID); scanErr != nil {
			foreignRows.Close()
			return scanErr
		}
		foreignRows.Close()
		return fmt.Errorf("%w: 表 %s 的行 %d 存在无效外键 %d", domain.ErrCorrupt, table, rowID, foreignKeyID)
	}
	if err = foreignRows.Close(); err != nil {
		return err
	}
	if err = s.validateImmutableTriggers(ctx); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM inspection_batches ORDER BY id`)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = s.Get(ctx, id); err != nil {
			return fmt.Errorf("校验批次 %s: %w", id, err)
		}
	}
	return nil
}

func (s *SQLite) validateImmutableTriggers(ctx context.Context) error {
	required := []string{
		"frozen_snapshots_no_update", "frozen_snapshots_no_delete",
		"credentials_no_update", "credentials_no_delete",
		"load_tests_no_update", "load_tests_no_delete",
		"audit_no_update", "audit_no_delete",
		"technical_reviews_no_update", "technical_reviews_no_delete",
	}
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='trigger'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := make(map[string]bool, len(required))
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return err
		}
		found[name] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, name := range required {
		if !found[name] {
			return fmt.Errorf("%w: 缺少只追加保护触发器 %s", domain.ErrCorrupt, name)
		}
	}
	return nil
}
