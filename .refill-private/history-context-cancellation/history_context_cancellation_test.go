package historycontextcancellation

import (
	"context"
	"errors"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"testing"
)

type controlledRepository struct {
	seen    chan context.Context
	release chan struct{}
}

func (r *controlledRepository) Create(context.Context, string, *domain.InspectionBatch) (*domain.InspectionBatch, bool, error) {
	return nil, false, errors.New("未使用")
}

func (r *controlledRepository) Mutate(context.Context, string, int64, string, string, func(*domain.InspectionBatch) error) (*domain.InspectionBatch, bool, error) {
	return nil, false, errors.New("未使用")
}

func (r *controlledRepository) Get(ctx context.Context, id string) (*domain.InspectionBatch, error) {
	r.seen <- ctx
	<-r.release
	return &domain.InspectionBatch{ID: id}, nil
}

func (r *controlledRepository) List(context.Context, int) ([]domain.InspectionBatch, error) {
	return nil, errors.New("未使用")
}

func (r *controlledRepository) GetCredential(context.Context, int64) (*domain.ReleaseCredential, error) {
	return nil, errors.New("未使用")
}

func (r *controlledRepository) NextCredentialSerial(context.Context) (int64, error) {
	return 0, errors.New("未使用")
}

func (r *controlledRepository) ListAudit(context.Context, string, int) ([]domain.AuditEvent, error) {
	return nil, errors.New("未使用")
}

func TestHistoryContextCancellation(t *testing.T) {
	repo := &controlledRepository{seen: make(chan context.Context, 1), release: make(chan struct{})}
	service := application.NewService(repo)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.TestHistory(ctx, "batch-1", "", "")
		done <- err
	}()

	storageContext := <-repo.seen
	cancel()
	canceled := false
	select {
	case <-storageContext.Done():
		canceled = true
	default:
	}

	close(repo.release)
	if !canceled {
		t.Fatal("repository context 未在请求取消时结束")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("请求取消后应返回 context.Canceled，得到 %v", err)
	}
}
