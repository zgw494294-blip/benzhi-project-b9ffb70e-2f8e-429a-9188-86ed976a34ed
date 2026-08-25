package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"stage-rigging-release/internal/domain"
)

func idempotent(ctx context.Context, tx *sql.Tx, key, action, batchID string) (*domain.InspectionBatch, bool, error) {
	var oldAction, oldBatch string
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT action,batch_id,response_json FROM idempotency_results WHERE idempotency_key=?`, key).Scan(&oldAction, &oldBatch, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if oldAction != action || (batchID != "" && oldBatch != batchID) {
		return nil, false, domain.ErrIdempotency
	}
	var b domain.InspectionBatch
	if err = json.Unmarshal(raw, &b); err != nil {
		return nil, false, err
	}
	if err = b.ValidateIntegrity(); err != nil {
		return nil, false, err
	}
	return &b, true, nil
}

func (s *SQLite) Create(ctx context.Context, key string, b *domain.InspectionBatch) (*domain.InspectionBatch, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if replayed, replay, e := idempotent(ctx, tx, key, "create_batch", ""); e != nil {
		return nil, false, e
	} else if replay {
		return replayed, true, nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO inspection_batches(id,venue_name,stage_zone,performance_at,owner_name,status,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, b.ID, b.VenueName, b.StageZone, timeText(b.PerformanceAt), b.OwnerName, b.Status, b.Version, timeText(b.CreatedAt), timeText(b.UpdatedAt))
	if err != nil {
		return nil, false, err
	}
	if err = saveResult(ctx, tx, key, "create_batch", b); err != nil {
		return nil, false, err
	}
	if err = appendAudit(ctx, tx, b, "create_batch"); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return b, false, nil
}

func (s *SQLite) Mutate(ctx context.Context, id string, expected int64, key, action string, fn func(*domain.InspectionBatch) error) (*domain.InspectionBatch, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if replayed, replay, e := idempotent(ctx, tx, key, action, id); e != nil {
		return nil, false, e
	} else if replay {
		return replayed, true, nil
	}
	b, err := loadBatch(ctx, tx, id)
	if err != nil {
		return nil, false, err
	}
	if b.Version != expected {
		return nil, false, domain.ErrVersion
	}
	if err = fn(b); err != nil {
		return nil, false, err
	}
	b.Version++
	b.UpdatedAt = s.now().UTC()
	if err = b.ValidateIntegrity(); err != nil {
		return nil, false, err
	}
	if err = saveAggregate(ctx, tx, b, expected); err != nil {
		return nil, false, err
	}
	if err = saveResult(ctx, tx, key, action, b); err != nil {
		return nil, false, err
	}
	if err = appendAudit(ctx, tx, b, action); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return b, false, nil
}

func saveAggregate(ctx context.Context, tx *sql.Tx, b *domain.InspectionBatch, expected int64) error {
	res, err := tx.ExecContext(ctx, `UPDATE inspection_batches SET venue_name=?,stage_zone=?,performance_at=?,owner_name=?,status=?,version=?,updated_at=? WHERE id=? AND version=?`, b.VenueName, b.StageZone, timeText(b.PerformanceAt), b.OwnerName, b.Status, b.Version, timeText(b.UpdatedAt), b.ID, expected)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return domain.ErrVersion
	}
	for _, p := range b.Points {
		_, err = tx.ExecContext(ctx, `INSERT INTO rigging_points(id,batch_id,point_code,hoist_serial,rope_spec,rated_load_kg,planned_load_kg,position_note,locked_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET locked_at=excluded.locked_at`, p.ID, b.ID, p.PointCode, p.HoistSerial, p.RopeSpec, p.RatedLoadKg, p.PlannedLoadKg, p.PositionNote, nullableTime(p.LockedAt))
		if err != nil {
			return err
		}
	}
	for _, t := range b.Tests {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO load_tests(id,batch_id,rigging_point_id,attempt_no,target_load_kg,measured_load_kg,hold_seconds,displacement_mm,result,recorded_by,recorded_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, t.ID, b.ID, t.RiggingPointID, t.AttemptNo, t.TargetLoadKg, t.MeasuredLoadKg, t.HoldSeconds, t.DisplacementMm, t.Result, t.RecordedBy, timeText(t.RecordedAt))
		if err != nil {
			return err
		}
	}
	for _, d := range b.Deviations {
		_, err = tx.ExecContext(ctx, `INSERT INTO deviations(id,batch_id,load_test_id,rigging_point_id,severity,symptom,required_action,assignee,remediation_evidence,status,closed_by_test_id,closed_at,assignee_confirmed,confirmed_by,confirmed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET severity=excluded.severity, remediation_evidence=excluded.remediation_evidence,status=excluded.status,closed_by_test_id=excluded.closed_by_test_id,closed_at=excluded.closed_at,assignee_confirmed=excluded.assignee_confirmed,confirmed_by=excluded.confirmed_by,confirmed_at=excluded.confirmed_at`, d.ID, b.ID, d.LoadTestID, d.RiggingPointID, d.Severity, d.Symptom, d.RequiredAction, d.Assignee, d.RemediationEvidence, d.Status, nullString(d.ClosedByTestID), nullableTime(d.ClosedAt), d.AssigneeConfirmed, d.ConfirmedBy, nullableTime(d.ConfirmedAt))
		if err != nil {
			return err
		}
	}
	if b.Snapshot != nil {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO frozen_snapshots(id,batch_id,digest,content,created_at) VALUES(?,?,?,?,?)`, b.Snapshot.ID, b.ID, b.Snapshot.Digest, b.Snapshot.Content, timeText(b.Snapshot.CreatedAt))
		if err != nil {
			return err
		}
	}
	if b.Review != nil {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO technical_reviews(id,batch_id,approved_by,approval_note,point_count,tested_point_count,passed_point_count,closed_deviation_count,reviewed_at) VALUES(?,?,?,?,?,?,?,?,?)`, b.Review.ID, b.ID, b.Review.ApprovedBy, b.Review.ApprovalNote, b.Review.PointCount, b.Review.TestedPointCount, b.Review.PassedPointCount, b.Review.ClosedDeviationCount, timeText(b.Review.ReviewedAt))
		if err != nil {
			return err
		}
	}
	if b.Credential != nil {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO release_credentials(id,batch_id,serial_number,snapshot_digest,approved_by,approval_note,issued_at,verification_status) VALUES(?,?,?,?,?,?,?,?)`, b.Credential.ID, b.ID, b.Credential.SerialNumber, b.Credential.SnapshotDigest, b.Credential.ApprovedBy, b.Credential.ApprovalNote, timeText(b.Credential.IssuedAt), b.Credential.VerificationStatus)
		if err != nil {
			return err
		}
	}
	return nil
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func saveResult(ctx context.Context, tx *sql.Tx, key, action string, b *domain.InspectionBatch) error {
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_results(idempotency_key,action,batch_id,response_json,created_at) VALUES(?,?,?,?,?)`, key, action, b.ID, raw, timeText(b.UpdatedAt))
	return err
}
func appendAudit(ctx context.Context, tx *sql.Tx, b *domain.InspectionBatch, action string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(batch_id,action,batch_version,occurred_at) VALUES(?,?,?,?)`, b.ID, action, b.Version, timeText(b.UpdatedAt))
	return err
}
