package domain

import "errors"

var (
	ErrValidation    = errors.New("业务数据校验失败")
	ErrStateConflict = errors.New("当前状态不允许此操作")
	ErrNotFound      = errors.New("记录不存在")
	ErrVersion       = errors.New("版本冲突")
	ErrIdempotency   = errors.New("幂等键已用于其他请求")
	ErrCorrupt       = errors.New("持久化聚合完整性校验失败")
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Problems []FieldError `json:"problems"`
}

type ApprovalBlockedError struct {
	DeviationIDs []string
	Reasons      []string
}

func (e *ApprovalBlockedError) Error() string { return "批次存在阻断事项，不能批准" }
func (e *ApprovalBlockedError) Unwrap() error { return ErrStateConflict }

func (e *ValidationError) Error() string { return ErrValidation.Error() }

func Invalid(field, message string) error {
	return &ValidationError{Problems: []FieldError{{Field: field, Message: message}}}
}
