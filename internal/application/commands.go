package application

import "time"

type WriteMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}
type CreateBatchCommand struct {
	WriteMeta
	VenueName     string    `json:"venueName"`
	StageZone     string    `json:"stageZone"`
	PerformanceAt time.Time `json:"performanceAt"`
	OwnerName     string    `json:"ownerName"`
}
type UpdateBatchCommand struct {
	WriteMeta
	VenueName     string    `json:"venueName"`
	StageZone     string    `json:"stageZone"`
	PerformanceAt time.Time `json:"performanceAt"`
	OwnerName     string    `json:"ownerName"`
}
type AddPointCommand struct {
	WriteMeta
	Precheck      bool         `json:"precheck"`
	Preview       bool         `json:"preview"`
	Preflight     bool         `json:"preflight"`
	DryRun        bool         `json:"dryRun"`
	PointCode     string       `json:"pointCode"`
	HoistSerial   string       `json:"hoistSerial"`
	RopeSpec      string       `json:"ropeSpec"`
	RatedLoadKg   float64      `json:"ratedLoadKg"`
	PlannedLoadKg float64      `json:"plannedLoadKg"`
	PositionNote  string       `json:"positionNote"`
	Points        []PointInput `json:"points,omitempty"`
	Items         []PointInput `json:"items,omitempty"`
}
type PointInput struct {
	PointCode     string  `json:"pointCode"`
	HoistSerial   string  `json:"hoistSerial"`
	RopeSpec      string  `json:"ropeSpec"`
	RatedLoadKg   float64 `json:"ratedLoadKg"`
	PlannedLoadKg float64 `json:"plannedLoadKg"`
	PositionNote  string  `json:"positionNote"`
}
type LockCommand struct{ WriteMeta }
type DeviationInput struct {
	Severity          string `json:"severity"`
	Symptom           string `json:"symptom"`
	RequiredAction    string `json:"requiredAction"`
	Assignee          string `json:"assignee"`
	AssigneeConfirmed bool   `json:"assigneeConfirmed"`
	ConfirmedBy       string `json:"confirmedBy"`
}
type RecordTestCommand struct {
	WriteMeta
	RiggingPointID string          `json:"riggingPointId"`
	TargetLoadKg   float64         `json:"targetLoadKg"`
	MeasuredLoadKg float64         `json:"measuredLoadKg"`
	HoldSeconds    int             `json:"holdSeconds"`
	DisplacementMm float64         `json:"displacementMm"`
	RecordedBy     string          `json:"recordedBy"`
	Deviation      *DeviationInput `json:"deviation,omitempty"`
}
type RemediateCommand struct {
	WriteMeta
	Evidence string `json:"evidence"`
}
type RetestCommand struct {
	WriteMeta
	TargetLoadKg   float64 `json:"targetLoadKg"`
	MeasuredLoadKg float64 `json:"measuredLoadKg"`
	HoldSeconds    int     `json:"holdSeconds"`
	DisplacementMm float64 `json:"displacementMm"`
	RecordedBy     string  `json:"recordedBy"`
}
type DeviationUpdateCommand struct {
	WriteMeta
	Severity    string `json:"severity"`
	ConfirmedBy string `json:"confirmedBy"`
}
type ApprovalCommand struct {
	WriteMeta
	ApprovedBy   string `json:"approvedBy"`
	ApprovalNote string `json:"approvalNote"`
}
type IssueCommand struct {
	WriteMeta
	ApprovedBy   string `json:"approvedBy"`
	ApprovalNote string `json:"approvalNote"`
}
