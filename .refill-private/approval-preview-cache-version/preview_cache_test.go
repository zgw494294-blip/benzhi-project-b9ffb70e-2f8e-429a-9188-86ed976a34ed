package approvalpreviewcache_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"stage-rigging-release/internal/httpapi"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestApprovalPreviewCacheVersionIsolation(t *testing.T) {
	repo, err := store.Open(context.Background(), "file:approval-preview-cache-version?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	handler := httpapi.New(application.NewService(repo), http.NotFoundHandler())
	created := requestJSON(t, handler, http.MethodPost, "/api/v1/batches", map[string]any{
		"expectedVersion": 0,
		"idempotencyKey":  "create-preview-cache-batch",
		"venueName":       "缓存版本测试剧场",
		"stageZone":       "主舞台",
		"performanceAt":   time.Now().UTC().Add(time.Hour),
		"ownerName":       "测试负责人",
	})
	var createResponse struct {
		Batch domain.InspectionBatch `json:"batch"`
	}
	decodeResponse(t, created, &createResponse)

	previewPath := "/api/v1/batches/" + createResponse.Batch.ID + "/approval-preview"
	first := requestJSON(t, handler, http.MethodGet, previewPath, nil)
	var firstPreview application.ApprovalPreview
	decodeResponse(t, first, &firstPreview)
	if firstPreview.Version != 1 {
		t.Fatalf("首次预览版本应为 1，得到 %d", firstPreview.Version)
	}

	pointPath := "/api/v1/batches/" + createResponse.Batch.ID + "/points"
	added := requestJSON(t, handler, http.MethodPost, pointPath, map[string]any{
		"expectedVersion": 1,
		"idempotencyKey":  "add-point-after-preview",
		"pointCode":       "P-CACHE-01",
		"hoistSerial":     "H-CACHE-01",
		"ropeSpec":        "6x19-20mm",
		"ratedLoadKg":     1000,
		"plannedLoadKg":   600,
		"positionNote":    "舞台中央",
	})
	if added.Code != http.StatusOK {
		t.Fatalf("登记吊点失败: status=%d body=%s", added.Code, added.Body.String())
	}

	second := requestJSON(t, handler, http.MethodGet, previewPath, nil)
	var secondPreview application.ApprovalPreview
	decodeResponse(t, second, &secondPreview)
	if secondPreview.Version != 2 || secondPreview.Progress.TotalPoints != 1 || len(secondPreview.UntestedPointCodes) != 1 {
		t.Fatalf("写事务后预览未反映批次版本和待测吊点: version=%d total=%d untested=%v", secondPreview.Version, secondPreview.Progress.TotalPoints, secondPreview.UntestedPointCodes)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if recorder.Code < 200 || recorder.Code >= 300 {
		t.Fatalf("请求失败: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if err := json.NewDecoder(recorder.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
