package application

import (
	"context"
	"stage-rigging-release/internal/domain"
	"strings"
)

func (s *Service) Remediate(ctx context.Context, batchID, deviationID string, c RemediateCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, "remediate_deviation", func(b *domain.InspectionBatch) error {
		for _, d := range b.Deviations {
			if d.ID == deviationID && d.Status == domain.DeviationOpen && (!d.AssigneeConfirmed || strings.TrimSpace(d.ConfirmedBy) == "") {
				return domain.ErrStateConflict
			}
		}
		return b.SubmitRemediation(deviationID, c.Evidence)
	})
}

func (s *Service) Retest(ctx context.Context, batchID, deviationID string, c RetestCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, "retest_deviation", func(b *domain.InspectionBatch) error {
		t := domain.LoadTest{ID: s.ids.New(), TargetLoadKg: c.TargetLoadKg, MeasuredLoadKg: c.MeasuredLoadKg, HoldSeconds: c.HoldSeconds, DisplacementMm: c.DisplacementMm, RecordedBy: c.RecordedBy, RecordedAt: s.now().UTC()}
		return b.CloseDeviation(deviationID, t, s.now())
	})
}

func (s *Service) UpdateDeviation(ctx context.Context, batchID, deviationID string, c DeviationUpdateCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	action := "update_deviation"
	if c.Severity != "" && c.ConfirmedBy == "" {
		action = "escalate_deviation"
	}
	if c.Severity == "" && c.ConfirmedBy != "" {
		action = "confirm_deviation"
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, action, func(b *domain.InspectionBatch) error {
		for i := range b.Deviations {
			if b.Deviations[i].ID != deviationID {
				continue
			}
			if c.Severity != "" {
				if err := b.Deviations[i].UpgradeSeverity(domain.DeviationSeverity(c.Severity)); err != nil {
					return err
				}
			}
			if c.ConfirmedBy != "" {
				if err := b.Deviations[i].ConfirmResponsibility(c.ConfirmedBy, s.now()); err != nil {
					return err
				}
			}
			b.Status = domain.StatusBlocked
			return nil
		}
		return domain.ErrNotFound
	})
}
