package domain

import "time"

type TechnicalReview struct {
	ID                   string    `json:"id"`
	BatchID              string    `json:"batchId"`
	ApprovedBy           string    `json:"approvedBy"`
	ApprovalNote         string    `json:"approvalNote"`
	PointCount           int       `json:"pointCount"`
	TestedPointCount     int       `json:"testedPointCount"`
	PassedPointCount     int       `json:"passedPointCount"`
	ClosedDeviationCount int       `json:"closedDeviationCount"`
	ReviewedAt           time.Time `json:"reviewedAt"`
}

type AuditEvent struct {
	ID           int64     `json:"id"`
	BatchID      string    `json:"batchId"`
	Action       string    `json:"action"`
	BatchVersion int64     `json:"batchVersion"`
	OccurredAt   time.Time `json:"occurredAt"`
}
