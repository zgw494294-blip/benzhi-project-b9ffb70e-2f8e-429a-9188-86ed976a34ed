package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"stage-rigging-release/internal/domain"
)

const maxBodyBytes = 1 << 20

type errorBody struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return domain.Invalid("Content-Type", "请求必须使用 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err = dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.Invalid("body", "请求体不能为空")
		}
		return domain.Invalid("body", "JSON 格式无效或包含未知字段")
	}
	if err = dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.Invalid("body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "服务处理请求时发生错误"
	var validation *domain.ValidationError
	var blocked *domain.ApprovalBlockedError
	switch {
	case errors.As(err, &validation):
		status, code, message = http.StatusUnprocessableEntity, "validation_failed", err.Error()
		writeJSON(w, status, errorBody{apiError{Code: code, Message: message, Details: validation.Problems}})
		return
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", err.Error()
	case errors.Is(err, domain.ErrVersion):
		status, code, message = http.StatusConflict, "version_conflict", "批次已被其他操作更新，请刷新后重试"
	case errors.As(err, &blocked):
		status, code, message = http.StatusConflict, "state_conflict", err.Error()
		writeJSON(w, status, errorBody{apiError{Code: code, Message: message, Details: map[string]any{"blockingDeviationIds": blocked.DeviationIDs, "reasons": blocked.Reasons}}})
		return
	case errors.Is(err, domain.ErrStateConflict):
		status, code, message = http.StatusConflict, "state_conflict", err.Error()
	case errors.Is(err, domain.ErrIdempotency):
		status, code, message = http.StatusConflict, "idempotency_conflict", err.Error()
	case errors.Is(err, domain.ErrCorrupt):
		status, code, message = http.StatusInternalServerError, "corrupt_data", "批次数据完整性校验失败"
	}
	writeJSON(w, status, errorBody{apiError{Code: code, Message: message}})
}
