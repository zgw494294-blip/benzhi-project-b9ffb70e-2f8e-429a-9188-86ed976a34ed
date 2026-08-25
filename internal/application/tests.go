package application

import (
	"context"
	"stage-rigging-release/internal/domain"
)

func (s *Service) RecordTest(ctx context.Context, batchID string, c RecordTestCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, "record_test", func(b *domain.InspectionBatch) error {
		t := domain.LoadTest{ID: s.ids.New(), RiggingPointID: c.RiggingPointID, TargetLoadKg: c.TargetLoadKg, MeasuredLoadKg: c.MeasuredLoadKg, HoldSeconds: c.HoldSeconds, DisplacementMm: c.DisplacementMm, RecordedBy: c.RecordedBy, RecordedAt: s.now().UTC()}
		var d *domain.Deviation
		if c.Deviation != nil {
			d = &domain.Deviation{ID: s.ids.New(), Severity: domain.DeviationSeverity(c.Deviation.Severity), Symptom: c.Deviation.Symptom, RequiredAction: c.Deviation.RequiredAction, Assignee: c.Deviation.Assignee, AssigneeConfirmed: c.Deviation.AssigneeConfirmed, ConfirmedBy: c.Deviation.ConfirmedBy}
			if d.AssigneeConfirmed && d.ConfirmedBy != "" {
				now := s.now().UTC()
				d.ConfirmedAt = &now
			}
		}
		return b.AddTest(t, d)
	})
}
