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
	batchMu    sync.Mutex
	batchCalls map[string]*batchCall
}

type batchCall struct {
	done  chan struct{}
	batch *domain.InspectionBatch
	err   error
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, ids: randomIDs{}, now: time.Now, batchCalls: make(map[string]*batchCall)}
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
	s.batchMu.Lock()
	if call, ok := s.batchCalls[id]; ok {
		s.batchMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			return call.batch, call.err
		}
	}
	call := &batchCall{done: make(chan struct{})}
	s.batchCalls[id] = call
	s.batchMu.Unlock()

	call.batch, call.err = s.repo.Get(ctx, id)
	close(call.done)
	s.batchMu.Lock()
	delete(s.batchCalls, id)
	s.batchMu.Unlock()
	return call.batch, call.err
}
func (s *Service) ListBatches(ctx context.Context) ([]domain.InspectionBatch, error) {
	return s.repo.List(ctx, 100)
}
func (s *Service) GetCredential(ctx context.Context, serial int64) (*domain.ReleaseCredential, error) {
	return s.repo.GetCredential(ctx, serial)
}
