package credentialverificationcacherace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"sync"
	"testing"
)

type barrierRepository struct {
	started chan struct{}
	release chan struct{}
	batch   *domain.InspectionBatch
}

func (r *barrierRepository) Get(context.Context, string) (*domain.InspectionBatch, error) {
	r.started <- struct{}{}
	<-r.release
	b := *r.batch
	return &b, nil
}

func (*barrierRepository) Create(context.Context, string, *domain.InspectionBatch) (*domain.InspectionBatch, bool, error) {
	panic("unexpected Create call")
}

func (*barrierRepository) Mutate(context.Context, string, int64, string, string, func(*domain.InspectionBatch) error) (*domain.InspectionBatch, bool, error) {
	panic("unexpected Mutate call")
}

func (*barrierRepository) List(context.Context, int) ([]domain.InspectionBatch, error) {
	panic("unexpected List call")
}

func (*barrierRepository) GetCredential(context.Context, int64) (*domain.ReleaseCredential, error) {
	panic("unexpected GetCredential call")
}

func (*barrierRepository) NextCredentialSerial(context.Context) (int64, error) {
	panic("unexpected NextCredentialSerial call")
}

func (*barrierRepository) ListAudit(context.Context, string, int) ([]domain.AuditEvent, error) {
	panic("unexpected ListAudit call")
}

func TestConcurrentCredentialVerificationCache(t *testing.T) {
	content := "immutable frozen rigging configuration"
	sum := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(sum[:])
	batch := &domain.InspectionBatch{
		ID:     "released-batch",
		Status: domain.StatusReleased,
		Snapshot: &domain.FrozenSnapshot{
			BatchID: "released-batch",
			Content: content,
			Digest:  digest,
		},
		Credential: &domain.ReleaseCredential{
			BatchID:        "released-batch",
			SnapshotDigest: digest,
		},
	}
	repo := &barrierRepository{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
		batch:   batch,
	}
	service := application.NewService(repo)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			verification, err := service.VerifyBatch(context.Background(), batch.ID)
			if err == nil && verification.Status != "valid" {
				t.Errorf("核验状态 = %q，期望 valid", verification.Status)
			}
			errs <- err
		}()
	}

	<-repo.started
	<-repo.started
	close(repo.release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("VerifyBatch() 返回错误: %v", err)
		}
	}
}
