package coalesced_get_cancellation_test

import (
	"context"
	"errors"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"sync"
	"testing"
	"time"
)

type controlledRepository struct {
	batch   *domain.InspectionBatch
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *controlledRepository) Get(ctx context.Context, _ string) (*domain.InspectionBatch, error) {
	r.once.Do(func() { close(r.entered) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.release:
		return r.batch, nil
	}
}

func (r *controlledRepository) Create(context.Context, string, *domain.InspectionBatch) (*domain.InspectionBatch, bool, error) {
	panic("unexpected Create")
}

func (r *controlledRepository) Mutate(context.Context, string, int64, string, string, func(*domain.InspectionBatch) error) (*domain.InspectionBatch, bool, error) {
	panic("unexpected Mutate")
}

func (r *controlledRepository) List(context.Context, int) ([]domain.InspectionBatch, error) {
	panic("unexpected List")
}

func (r *controlledRepository) GetCredential(context.Context, int64) (*domain.ReleaseCredential, error) {
	panic("unexpected GetCredential")
}

func (r *controlledRepository) NextCredentialSerial(context.Context) (int64, error) {
	panic("unexpected NextCredentialSerial")
}

func (r *controlledRepository) ListAudit(context.Context, string, int) ([]domain.AuditEvent, error) {
	panic("unexpected ListAudit")
}

type observedContext struct {
	context.Context
	observed chan struct{}
	done     chan struct{}
	once     sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.done
}

type getResult struct {
	batch *domain.InspectionBatch
	err   error
}

func TestCoalescedGetCancellationIsolation(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	batch, err := domain.NewBatch("batch-1", "城市剧场", "主舞台", now.Add(24*time.Hour), "技术负责人", now)
	if err != nil {
		t.Fatalf("create fixture batch: %v", err)
	}
	repo := &controlledRepository{batch: batch, entered: make(chan struct{}), release: make(chan struct{})}
	service := application.NewService(repo)

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	firstResult := make(chan getResult, 1)
	go func() {
		got, getErr := service.GetBatch(firstCtx, batch.ID)
		firstResult <- getResult{batch: got, err: getErr}
	}()
	<-repo.entered

	secondCtx := &observedContext{Context: context.Background(), observed: make(chan struct{}), done: make(chan struct{})}
	secondResult := make(chan getResult, 1)
	go func() {
		got, getErr := service.GetBatch(secondCtx, batch.ID)
		secondResult <- getResult{batch: got, err: getErr}
	}()
	<-secondCtx.observed

	cancelFirst()
	first := <-firstResult
	if !errors.Is(first.err, context.Canceled) {
		t.Fatalf("first caller should observe its cancellation, got %v", first.err)
	}
	close(repo.release)

	second := <-secondResult
	if second.err != nil {
		t.Fatalf("second caller was poisoned by first cancellation: %v", second.err)
	}
	if second.batch == nil || second.batch.ID != batch.ID {
		t.Fatalf("second caller received wrong batch: %#v", second.batch)
	}
}
