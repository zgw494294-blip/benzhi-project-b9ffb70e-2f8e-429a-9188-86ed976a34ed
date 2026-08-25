package httpapi

import (
	"net/http"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"strconv"
	"time"
)

func (a *API) ListBatches(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	stage := q.Get("stageZone")
	if stage == "" {
		stage = q.Get("stage")
	}
	owner := q.Get("ownerName")
	if owner == "" {
		owner = q.Get("owner")
	}
	f := domain.BatchFilter{Status: q.Get("status"), StageZone: stage, OwnerName: owner, Limit: 50, Cursor: q.Get("cursor")}
	var err error
	fromValue := q.Get("from")
	if fromValue == "" {
		fromValue = q.Get("start")
	}
	if v := fromValue; v != "" {
		t, e := time.Parse(time.RFC3339, v)
		if e != nil {
			writeError(w, domain.Invalid("from", "开始时间必须为 RFC3339"))
			return
		}
		f.From = &t
	}
	toValue := q.Get("to")
	if toValue == "" {
		toValue = q.Get("end")
	}
	if v := toValue; v != "" {
		t, e := time.Parse(time.RFC3339, v)
		if e != nil {
			writeError(w, domain.Invalid("to", "结束时间必须为 RFC3339"))
			return
		}
		f.To = &t
	}
	if v := q.Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 200 {
			writeError(w, domain.Invalid("limit", "分页上限必须为 1 至 200"))
			return
		}
		f.Limit = n
	}
	page, err := a.service.ListBatchesPage(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	type item struct {
		domain.InspectionBatch
		Progress  domain.Progress `json:"progress"`
		RiskLevel string          `json:"riskLevel"`
		RiskFlags []string        `json:"riskFlags,omitempty"`
	}
	result := make([]item, 0, len(page.Batches))
	for _, b := range page.Batches {
		p := b.ProgressAt(time.Now())
		result = append(result, item{InspectionBatch: b, Progress: p, RiskLevel: p.RiskLevel, RiskFlags: p.RiskFlags})
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": result, "nextCursor": page.NextCursor})
}
func (a *API) GetBatch(w http.ResponseWriter, r *http.Request) {
	b, err := a.service.GetBatch(r.Context(), r.PathValue("batchId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, responseFor(b, false))
}
func (a *API) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var c application.CreateBatchCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.CreateBatch(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, status, responseFor(b, replayed))
}

func (a *API) UpdateBatch(w http.ResponseWriter, r *http.Request) {
	var command application.UpdateBatchCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.UpdateBatch(r.Context(), r.PathValue("batchId"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, batch, replayed)
}

func (a *API) ListAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := application.AuditFilter{Action: q.Get("action")}
	var err error
	if v := q.Get("from"); v != "" {
		t, e := time.Parse(time.RFC3339, v)
		if e != nil {
			writeError(w, domain.Invalid("from", "开始时间必须为 RFC3339"))
			return
		}
		f.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, e := time.Parse(time.RFC3339, v)
		if e != nil {
			writeError(w, domain.Invalid("to", "结束时间必须为 RFC3339"))
			return
		}
		f.To = &t
	}
	if v := q.Get("version"); v != "" {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil || n < 1 {
			writeError(w, domain.Invalid("version", "版本必须为正整数"))
			return
		}
		f.Version = n
	}
	limit := 200
	if v := q.Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 500 {
			writeError(w, domain.Invalid("limit", "分页上限必须为 1 至 500"))
			return
		}
		limit = n
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		writeError(w, domain.Invalid("timeRange", "时间范围无效"))
		return
	}
	events, err := a.service.ListAuditFiltered(r.Context(), r.PathValue("batchId"), f)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(events) > limit {
		events = events[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (a *API) ListTests(w http.ResponseWriter, r *http.Request) {
	h, err := a.service.TestHistory(r.Context(), r.PathValue("batchId"), r.URL.Query().Get("pointCode"), r.URL.Query().Get("result"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}
func (a *API) AddPoint(w http.ResponseWriter, r *http.Request) {
	var c application.AddPointCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if c.Precheck || c.Preview || c.Preflight || c.DryRun {
		result, err := a.service.PrecheckPoints(r.Context(), r.PathValue("batchId"), c)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	b, replayed, err := a.service.AddPoint(r.Context(), r.PathValue("batchId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) LockBatch(w http.ResponseWriter, r *http.Request) {
	var c application.LockCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.LockBatch(r.Context(), r.PathValue("batchId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
