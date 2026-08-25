package application

import (
	"context"
	"stage-rigging-release/internal/domain"
	"time"
)

type AuditFilter struct {
	Action   string
	From, To *time.Time
	Version  int64
}

func (s *Service) ListAudit(ctx context.Context, batchID string) ([]domain.AuditEvent, error) {
	if _, err := s.repo.Get(ctx, batchID); err != nil {
		return nil, err
	}
	return s.repo.ListAudit(ctx, batchID, 200)
}

func (s *Service) ListAuditFiltered(ctx context.Context, batchID string, f AuditFilter) ([]domain.AuditEvent, error) {
	if _, err := s.repo.Get(ctx, batchID); err != nil {
		return nil, err
	}
	events, err := s.repo.ListAudit(ctx, batchID, 500)
	if err != nil {
		return nil, err
	}
	result := events[:0]
	for _, e := range events {
		if f.Action != "" && e.Action != f.Action {
			continue
		}
		if f.Version > 0 && e.BatchVersion != f.Version {
			continue
		}
		if f.From != nil && e.OccurredAt.Before(*f.From) {
			continue
		}
		if f.To != nil && e.OccurredAt.After(*f.To) {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}
