package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"stage-rigging-release/internal/domain"
	"strings"
	"time"
)

func (s *SQLite) LookupIdempotency(ctx context.Context, key, action, batchID string) (*domain.InspectionBatch, bool, error) {
	var oldAction, oldBatch string
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT action,batch_id,response_json FROM idempotency_results WHERE idempotency_key=?`, key).Scan(&oldAction, &oldBatch, &raw)
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
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, false, err
	}
	return &b, true, b.ValidateIntegrity()
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadBatch(ctx context.Context, q queryer, id string) (*domain.InspectionBatch, error) {
	b := &domain.InspectionBatch{}
	var status, performance, created, updated string
	err := q.QueryRowContext(ctx, `SELECT id,venue_name,stage_zone,performance_at,owner_name,status,version,created_at,updated_at FROM inspection_batches WHERE id=?`, id).Scan(&b.ID, &b.VenueName, &b.StageZone, &performance, &b.OwnerName, &status, &b.Version, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Status = domain.BatchStatus(status)
	b.PerformanceAt = parseTime(performance)
	b.CreatedAt = parseTime(created)
	b.UpdatedAt = parseTime(updated)
	if err = loadPoints(ctx, q, b); err != nil {
		return nil, err
	}
	if err = loadTests(ctx, q, b); err != nil {
		return nil, err
	}
	if err = loadDeviations(ctx, q, b); err != nil {
		return nil, err
	}
	if err = loadSnapshot(ctx, q, b); err != nil {
		return nil, err
	}
	if err = loadReview(ctx, q, b); err != nil {
		return nil, err
	}
	if err = loadRelease(ctx, q, b); err != nil {
		return nil, err
	}
	if err = b.ValidateIntegrity(); err != nil {
		return nil, err
	}
	return b, nil
}

func loadReview(ctx context.Context, q queryer, b *domain.InspectionBatch) error {
	var review domain.TechnicalReview
	var reviewedAt string
	err := q.QueryRowContext(ctx, `SELECT id,approved_by,approval_note,point_count,tested_point_count,passed_point_count,closed_deviation_count,reviewed_at FROM technical_reviews WHERE batch_id=?`, b.ID).Scan(&review.ID, &review.ApprovedBy, &review.ApprovalNote, &review.PointCount, &review.TestedPointCount, &review.PassedPointCount, &review.ClosedDeviationCount, &reviewedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	review.BatchID = b.ID
	review.ReviewedAt = parseTime(reviewedAt)
	b.Review = &review
	return nil
}

func loadPoints(ctx context.Context, q queryer, b *domain.InspectionBatch) error {
	rows, err := q.QueryContext(ctx, `SELECT id,point_code,hoist_serial,rope_spec,rated_load_kg,planned_load_kg,position_note,locked_at FROM rigging_points WHERE batch_id=? ORDER BY point_code`, b.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.RiggingPoint
		var locked sql.NullString
		if err = rows.Scan(&p.ID, &p.PointCode, &p.HoistSerial, &p.RopeSpec, &p.RatedLoadKg, &p.PlannedLoadKg, &p.PositionNote, &locked); err != nil {
			return err
		}
		p.BatchID = b.ID
		p.LockedAt = parseOptional(locked)
		review := p.ReviewCapacity()
		p.CapacityReview = &review
		b.Points = append(b.Points, p)
	}
	return rows.Err()
}

func loadTests(ctx context.Context, q queryer, b *domain.InspectionBatch) error {
	rows, err := q.QueryContext(ctx, `SELECT id,rigging_point_id,attempt_no,target_load_kg,measured_load_kg,hold_seconds,displacement_mm,result,recorded_by,recorded_at FROM load_tests WHERE batch_id=? ORDER BY recorded_at,id`, b.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t domain.LoadTest
		var result, at string
		if err = rows.Scan(&t.ID, &t.RiggingPointID, &t.AttemptNo, &t.TargetLoadKg, &t.MeasuredLoadKg, &t.HoldSeconds, &t.DisplacementMm, &result, &t.RecordedBy, &at); err != nil {
			return err
		}
		t.BatchID = b.ID
		t.Result = domain.TestResult(result)
		t.RecordedAt = parseTime(at)
		b.Tests = append(b.Tests, t)
	}
	return rows.Err()
}

func loadDeviations(ctx context.Context, q queryer, b *domain.InspectionBatch) error {
	rows, err := q.QueryContext(ctx, `SELECT id,load_test_id,rigging_point_id,severity,symptom,required_action,assignee,remediation_evidence,status,closed_by_test_id,closed_at,assignee_confirmed,confirmed_by,confirmed_at FROM deviations WHERE batch_id=? ORDER BY rowid`, b.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d domain.Deviation
		var severity, status string
		var closedID, closedAt, confirmedBy, confirmedAt sql.NullString
		var confirmed bool
		if err = rows.Scan(&d.ID, &d.LoadTestID, &d.RiggingPointID, &severity, &d.Symptom, &d.RequiredAction, &d.Assignee, &d.RemediationEvidence, &status, &closedID, &closedAt, &confirmed, &confirmedBy, &confirmedAt); err != nil {
			return err
		}
		d.BatchID = b.ID
		d.Severity = domain.DeviationSeverity(severity)
		d.Status = domain.DeviationStatus(status)
		d.ClosedByTestID = closedID.String
		d.ClosedAt = parseOptional(closedAt)
		d.AssigneeConfirmed, d.ConfirmedBy, d.ConfirmedAt = confirmed, confirmedBy.String, parseOptional(confirmedAt)
		b.Deviations = append(b.Deviations, d)
	}
	return rows.Err()
}

func loadSnapshot(ctx context.Context, q queryer, b *domain.InspectionBatch) error {
	var s domain.FrozenSnapshot
	var at string
	err := q.QueryRowContext(ctx, `SELECT id,digest,content,created_at FROM frozen_snapshots WHERE batch_id=?`, b.ID).Scan(&s.ID, &s.Digest, &s.Content, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	s.BatchID = b.ID
	s.CreatedAt = parseTime(at)
	b.Snapshot = &s
	return nil
}

func loadRelease(ctx context.Context, q queryer, b *domain.InspectionBatch) error {
	var c domain.ReleaseCredential
	var at string
	err := q.QueryRowContext(ctx, `SELECT id,serial_number,snapshot_digest,approved_by,approval_note,issued_at,verification_status FROM release_credentials WHERE batch_id=?`, b.ID).Scan(&c.ID, &c.SerialNumber, &c.SnapshotDigest, &c.ApprovedBy, &c.ApprovalNote, &at, &c.VerificationStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	c.BatchID = b.ID
	c.IssuedAt = parseTime(at)
	b.Credential = &c
	return nil
}

func (s *SQLite) Get(ctx context.Context, id string) (*domain.InspectionBatch, error) {
	return loadBatch(ctx, s.db, id)
}

func (s *SQLite) List(ctx context.Context, limit int) ([]domain.InspectionBatch, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM inspection_batches ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	result := make([]domain.InspectionBatch, 0, len(ids))
	for _, id := range ids {
		b, e := s.Get(ctx, id)
		if e != nil {
			return nil, e
		}
		result = append(result, *b)
	}
	return result, nil
}

func (s *SQLite) ListFiltered(ctx context.Context, f domain.BatchFilter) ([]domain.InspectionBatch, error) {
	query := `SELECT id FROM inspection_batches WHERE 1=1`
	args := make([]any, 0, 4)
	if f.Status != "" {
		query += " AND status=?"
		args = append(args, f.Status)
	}
	if f.StageZone != "" {
		query += " AND stage_zone=?"
		args = append(args, f.StageZone)
	}
	if f.OwnerName != "" {
		query += " AND owner_name=?"
		args = append(args, f.OwnerName)
	}
	if f.From != nil {
		query += " AND performance_at>=?"
		args = append(args, f.From.UTC().Format(time.RFC3339Nano))
	}
	if f.To != nil {
		query += " AND performance_at<=?"
		args = append(args, f.To.UTC().Format(time.RFC3339Nano))
	}
	limit := f.Limit
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if f.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err != nil {
			return nil, domain.Invalid("cursor", "分页游标无效")
		}
		parts := strings.SplitN(string(raw), "|", 2)
		if len(parts) != 2 || parts[1] == "" {
			return nil, domain.Invalid("cursor", "分页游标无效")
		}
		if _, err := time.Parse(time.RFC3339Nano, parts[0]); err != nil {
			return nil, domain.Invalid("cursor", "分页游标无效")
		}
		var cursorPerformance string
		if err := s.db.QueryRowContext(ctx, `SELECT performance_at FROM inspection_batches WHERE id=?`, parts[1]).Scan(&cursorPerformance); errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		} else if err != nil {
			return nil, err
		}
		if cursorPerformance != parts[0] {
			return nil, domain.Invalid("cursor", "分页游标已失效")
		}
		query += " AND (performance_at>? OR (performance_at=? AND id>?))"
		args = append(args, parts[0], parts[0], parts[1])
	}
	query += " ORDER BY performance_at ASC, id ASC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	result := make([]domain.InspectionBatch, 0, len(ids))
	for _, id := range ids {
		b, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, *b)
	}
	return result, nil
}

func (s *SQLite) GetCredential(ctx context.Context, serial int64) (*domain.ReleaseCredential, error) {
	var c domain.ReleaseCredential
	var at string
	err := s.db.QueryRowContext(ctx, `SELECT id,batch_id,serial_number,snapshot_digest,approved_by,approval_note,issued_at,verification_status FROM release_credentials WHERE serial_number=?`, serial).Scan(&c.ID, &c.BatchID, &c.SerialNumber, &c.SnapshotDigest, &c.ApprovedBy, &c.ApprovalNote, &at, &c.VerificationStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.IssuedAt = parseTime(at)
	return &c, nil
}

func (s *SQLite) NextCredentialSerial(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO credential_sequence DEFAULT VALUES`)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLite) ListAudit(ctx context.Context, batchID string, limit int) ([]domain.AuditEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,batch_id,action,batch_version,occurred_at FROM audit_events WHERE batch_id=? ORDER BY id DESC LIMIT ?`, batchID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var event domain.AuditEvent
		var occurredAt string
		if err = rows.Scan(&event.ID, &event.BatchID, &event.Action, &event.BatchVersion, &occurredAt); err != nil {
			return nil, err
		}
		event.OccurredAt = parseTime(occurredAt)
		events = append(events, event)
	}
	return events, rows.Err()
}
