package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"stage-rigging-release/internal/domain"
	"strings"
	"sync"
	"time"
)

type Service struct {
	repo       Repository
	ids        IDGenerator
	now        func() time.Time
	cacheMu    sync.RWMutex
	batchCache map[string]*domain.InspectionBatch
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, ids: randomIDs{}, now: time.Now, batchCache: make(map[string]*domain.InspectionBatch)}
}

type randomIDs struct{}

func (randomIDs) New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

func validateMeta(meta WriteMeta, creation bool) error {
	if strings.TrimSpace(meta.IdempotencyKey) == "" || len(meta.IdempotencyKey) > 128 {
		return domain.Invalid("idempotencyKey", "幂等键不能为空且不能超过 128 字符")
	}
	if creation && meta.ExpectedVersion != 0 {
		return domain.Invalid("expectedVersion", "创建批次时 expectedVersion 必须为 0")
	}
	if !creation && meta.ExpectedVersion < 1 {
		return domain.Invalid("expectedVersion", "expectedVersion 必须大于 0")
	}
	return nil
}

func (s *Service) GetBatch(ctx context.Context, id string) (*domain.InspectionBatch, error) {
	s.cacheMu.RLock()
	cached := s.batchCache[id]
	s.cacheMu.RUnlock()
	if cached != nil {
		return cloneBatch(cached), nil
	}
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	s.batchCache[id] = cloneBatch(b)
	s.cacheMu.Unlock()
	return cloneBatch(b), nil
}
func (s *Service) ListBatches(ctx context.Context) ([]domain.InspectionBatch, error) {
	return s.repo.List(ctx, 100)
}
func (s *Service) GetCredential(ctx context.Context, serial int64) (*domain.ReleaseCredential, error) {
	return s.repo.GetCredential(ctx, serial)
}

func cloneBatch(b *domain.InspectionBatch) *domain.InspectionBatch {
	if b == nil {
		return nil
	}
	out := *b
	out.Points = append([]domain.RiggingPoint(nil), b.Points...)
	for i := range b.Points {
		if b.Points[i].CapacityReview != nil {
			review := *b.Points[i].CapacityReview
			out.Points[i].CapacityReview = &review
		}
	}
	out.Tests = append([]domain.LoadTest(nil), b.Tests...)
	out.Deviations = append([]domain.Deviation(nil), b.Deviations...)
	for i := range b.Deviations {
		if b.Deviations[i].ConfirmedAt != nil {
			confirmedAt := *b.Deviations[i].ConfirmedAt
			out.Deviations[i].ConfirmedAt = &confirmedAt
		}
		if b.Deviations[i].ClosedAt != nil {
			closedAt := *b.Deviations[i].ClosedAt
			out.Deviations[i].ClosedAt = &closedAt
		}
	}
	if b.Snapshot != nil {
		snapshot := *b.Snapshot
		out.Snapshot = &snapshot
	}
	if b.Review != nil {
		review := *b.Review
		out.Review = &review
	}
	if b.Credential != nil {
		credential := *b.Credential
		out.Credential = &credential
	}
	return &out
}
