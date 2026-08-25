package httpapi

import (
	"context"
	"net/http"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"strings"
	"time"
)

type API struct {
	service *application.Service
	ui      http.Handler
}

func New(service *application.Service, ui http.Handler) http.Handler {
	a := &API{service: service, ui: ui}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/batches", a.ListBatches)
	mux.HandleFunc("POST /api/v1/batches", a.CreateBatch)
	mux.HandleFunc("GET /api/v1/batches/{batchId}", a.GetBatch)
	mux.HandleFunc("PATCH /api/v1/batches/{batchId}", a.UpdateBatch)
	mux.HandleFunc("GET /api/v1/batches/{batchId}/audit", a.ListAudit)
	mux.HandleFunc("GET /api/v1/batches/{batchId}/tests", a.ListTests)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/points", a.AddPoint)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/points/batch", a.AddPoint)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/lock", a.LockBatch)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/tests", a.RecordTest)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/deviations/{deviationId}/remediation", a.RemediateDeviation)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/deviations/{deviationId}/retest", a.RetestDeviation)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/deviations/{deviationId}/update", a.UpdateDeviation)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/deviations/{deviationId}/escalate", a.UpdateDeviation)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/deviations/{deviationId}/confirm", a.UpdateDeviation)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/approval", a.ApproveBatch)
	mux.HandleFunc("GET /api/v1/batches/{batchId}/approval-preview", a.ApprovalPreview)
	mux.HandleFunc("POST /api/v1/batches/{batchId}/credential", a.IssueCredential)
	mux.HandleFunc("GET /api/v1/batches/{batchId}/verification", a.VerifyBatch)
	mux.HandleFunc("GET /api/v1/credentials/{serial}", a.GetCredential)
	mux.Handle("/", ui)
	return securityHeaders(requestDeadline(negotiateResponses(mux)))
}

func negotiateResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			accept := r.Header.Get("Accept")
			if accept != "" && accept != "*/*" && !strings.Contains(accept, "application/json") {
				writeJSON(w, http.StatusNotAcceptable, errorBody{apiError{Code: "not_acceptable", Message: "API 仅提供 application/json 响应"}})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requestDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextWithTimeout(r, 15*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

type batchResponse struct {
	Batch    *domain.InspectionBatch `json:"batch"`
	Progress domain.Progress         `json:"progress"`
	Replayed bool                    `json:"replayed"`
}

func responseFor(b *domain.InspectionBatch, replayed bool) batchResponse {
	return batchResponse{Batch: b, Progress: b.ProgressAt(time.Now()), Replayed: replayed}
}
