package historyfiltercachealias_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"stage-rigging-release/internal/httpapi"
	"testing"
	"time"
)

type historyRepository struct {
	batch *domain.InspectionBatch
}

func (r *historyRepository) Create(context.Context, string, *domain.InspectionBatch) (*domain.InspectionBatch, bool, error) {
	return nil, false, nil
}

func (r *historyRepository) Mutate(context.Context, string, int64, string, string, func(*domain.InspectionBatch) error) (*domain.InspectionBatch, bool, error) {
	return nil, false, nil
}

func (r *historyRepository) Get(context.Context, string) (*domain.InspectionBatch, error) {
	return r.batch, nil
}

func (r *historyRepository) List(context.Context, int) ([]domain.InspectionBatch, error) {
	return nil, nil
}

func (r *historyRepository) GetCredential(context.Context, int64) (*domain.ReleaseCredential, error) {
	return nil, domain.ErrNotFound
}

func (r *historyRepository) NextCredentialSerial(context.Context) (int64, error) {
	return 1, nil
}

func (r *historyRepository) ListAudit(context.Context, string, int) ([]domain.AuditEvent, error) {
	return nil, nil
}

func TestHistoryFilterCacheIsolation(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	batch := &domain.InspectionBatch{
		ID: "batch-1", Version: 7, Status: domain.StatusPendingReview,
		Points: []domain.RiggingPoint{
			{ID: "point-a", PointCode: "P-A"},
			{ID: "point-b", PointCode: "P-B"},
		},
		Tests: []domain.LoadTest{
			{ID: "test-b", RiggingPointID: "point-b", AttemptNo: 1, Result: domain.TestPassed, RecordedAt: now},
			{ID: "test-a", RiggingPointID: "point-a", AttemptNo: 1, Result: domain.TestPassed, RecordedAt: now},
		},
	}
	handler := httpapi.New(application.NewService(&historyRepository{batch: batch}), http.NotFoundHandler())

	first := performHistoryRequest(t, handler, "/api/v1/batches/batch-1/tests?pointCode=P-A")
	if len(first.Tests) != 1 || first.Tests[0].RiggingPointID != "point-a" {
		t.Fatalf("第一次筛选结果异常: %+v", first.Tests)
	}
	second := performHistoryRequest(t, handler, "/api/v1/batches/batch-1/tests?pointCode=P-B")
	if len(second.Tests) != 1 || second.Tests[0].RiggingPointID != "point-b" {
		t.Fatalf("第二次筛选应返回 point-b 的试验，实际为: %+v", second.Tests)
	}
	if second.LatestAttemptNo["P-B"] != 1 || second.LatestResultByPoint["P-B"] != domain.TestPassed {
		t.Fatalf("第二次筛选丢失 point-b 的最新试验摘要: attempt=%d result=%q", second.LatestAttemptNo["P-B"], second.LatestResultByPoint["P-B"])
	}
}

func performHistoryRequest(t *testing.T, handler http.Handler, target string) application.TestHistory {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("历史查询返回状态 %d: %s", recorder.Code, recorder.Body.String())
	}
	var history application.TestHistory
	if err := json.Unmarshal(recorder.Body.Bytes(), &history); err != nil {
		t.Fatalf("解析历史响应: %v", err)
	}
	return history
}
