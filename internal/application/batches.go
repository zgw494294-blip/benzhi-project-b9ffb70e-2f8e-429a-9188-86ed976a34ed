package application

import (
	"context"
	"encoding/base64"
	"sort"
	"stage-rigging-release/internal/domain"
	"strings"
	"time"
)

type BatchPage struct {
	Batches    []domain.InspectionBatch
	NextCursor string
}

func (s *Service) CreateBatch(ctx context.Context, c CreateBatchCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, true); err != nil {
		return nil, false, err
	}
	b, err := domain.NewBatch(s.ids.New(), c.VenueName, c.StageZone, c.PerformanceAt, c.OwnerName, s.now())
	if err != nil {
		return nil, false, err
	}
	return s.repo.Create(ctx, c.IdempotencyKey, b)
}

func (s *Service) UpdateBatch(ctx context.Context, id string, c UpdateBatchCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, id, c.ExpectedVersion, c.IdempotencyKey, "update_batch", func(b *domain.InspectionBatch) error {
		return b.UpdateDetails(c.VenueName, c.StageZone, c.PerformanceAt, c.OwnerName)
	})
}

func (s *Service) LockBatch(ctx context.Context, id string, c LockCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, id, c.ExpectedVersion, c.IdempotencyKey, "lock_batch", func(b *domain.InspectionBatch) error { return b.Lock(s.now()) })
}

func (s *Service) ListBatchesFiltered(ctx context.Context, f domain.BatchFilter) ([]domain.InspectionBatch, error) {
	p, err := s.ListBatchesPage(ctx, f)
	return p.Batches, err
}

func (s *Service) ListBatchesPage(ctx context.Context, f domain.BatchFilter) (BatchPage, error) {
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		return BatchPage{}, domain.Invalid("to", "结束时间不能早于开始时间")
	}
	if f.Status != "" {
		valid := map[domain.BatchStatus]bool{domain.StatusDraft: true, domain.StatusAwaitingTests: true, domain.StatusTesting: true, domain.StatusBlocked: true, domain.StatusPendingReview: true, domain.StatusFrozen: true, domain.StatusReleased: true}
		if !valid[domain.BatchStatus(f.Status)] {
			return BatchPage{}, domain.Invalid("status", "批次状态无效")
		}
	}
	if f.Limit < 1 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Cursor != "" {
		if _, _, err := decodeBatchCursor(f.Cursor); err != nil {
			return BatchPage{}, domain.Invalid("cursor", "分页游标无效")
		}
	}
	if r, ok := s.repo.(interface {
		ListFiltered(context.Context, domain.BatchFilter) ([]domain.InspectionBatch, error)
	}); ok {
		items, err := r.ListFiltered(ctx, f)
		if err != nil {
			return BatchPage{}, err
		}
		// The store supplies the keyset order; risk ranking is deterministic within it.
		now := s.now().UTC()
		riskRank := func(b domain.InspectionBatch) int {
			switch b.ProgressAt(now).RiskLevel {
			case "critical":
				return 4
			case "high":
				return 3
			case "medium":
				return 2
			default:
				return 1
			}
		}
		sort.SliceStable(items, func(i, j int) bool {
			ri, rj := riskRank(items[i]), riskRank(items[j])
			if ri != rj {
				return ri > rj
			}
			if !items[i].PerformanceAt.Equal(items[j].PerformanceAt) {
				return items[i].PerformanceAt.Before(items[j].PerformanceAt)
			}
			return items[i].ID < items[j].ID
		})
		hasMore := len(items) > f.Limit
		if hasMore {
			items = items[:f.Limit]
		}
		page := BatchPage{Batches: items}
		if hasMore {
			last := items[len(items)-1]
			page.NextCursor = encodeBatchCursor(last.PerformanceAt, last.ID)
		}
		return page, nil
	}
	items, err := s.repo.List(ctx, f.Limit)
	return BatchPage{Batches: items}, err
}

func encodeBatchCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeBatchCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return time.Time{}, "", domain.ErrNotFound
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return at, parts[1], nil
}
