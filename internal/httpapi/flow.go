package httpapi

import (
	"net/http"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"strconv"
)

func writeMutation(w http.ResponseWriter, b *domain.InspectionBatch, replayed bool) {
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, http.StatusOK, responseFor(b, replayed))
}
func (a *API) RecordTest(w http.ResponseWriter, r *http.Request) {
	var c application.RecordTestCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.RecordTest(r.Context(), r.PathValue("batchId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) RemediateDeviation(w http.ResponseWriter, r *http.Request) {
	var c application.RemediateCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.Remediate(r.Context(), r.PathValue("batchId"), r.PathValue("deviationId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) RetestDeviation(w http.ResponseWriter, r *http.Request) {
	var c application.RetestCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.Retest(r.Context(), r.PathValue("batchId"), r.PathValue("deviationId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) UpdateDeviation(w http.ResponseWriter, r *http.Request) {
	var c application.DeviationUpdateCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.UpdateDeviation(r.Context(), r.PathValue("batchId"), r.PathValue("deviationId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) ApproveBatch(w http.ResponseWriter, r *http.Request) {
	var c application.ApprovalCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.Approve(r.Context(), r.PathValue("batchId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) ApprovalPreview(w http.ResponseWriter, r *http.Request) {
	v, err := a.service.ApprovalPreview(r.Context(), r.PathValue("batchId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (a *API) IssueCredential(w http.ResponseWriter, r *http.Request) {
	var c application.IssueCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	b, replayed, err := a.service.Issue(r.Context(), r.PathValue("batchId"), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeMutation(w, b, replayed)
}
func (a *API) VerifyBatch(w http.ResponseWriter, r *http.Request) {
	v, err := a.service.VerifyBatch(r.Context(), r.PathValue("batchId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (a *API) GetCredential(w http.ResponseWriter, r *http.Request) {
	serial, err := strconv.ParseInt(r.PathValue("serial"), 10, 64)
	if err != nil || serial < 1 {
		writeError(w, domain.Invalid("serial", "凭据序号必须为正整数"))
		return
	}
	c, err := a.service.GetCredential(r.Context(), serial)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credential": c})
}
