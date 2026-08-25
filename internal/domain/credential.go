package domain

import "time"

type BatchFilter struct {
	Status    string
	StageZone string
	OwnerName string
	From, To  *time.Time
	Limit     int
	Cursor    string
}

type FrozenSnapshot struct {
	ID        string    `json:"id"`
	BatchID   string    `json:"batchId"`
	Digest    string    `json:"digest"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type ReleaseCredential struct {
	ID                 string    `json:"id"`
	BatchID            string    `json:"batchId"`
	SerialNumber       int64     `json:"serialNumber"`
	SnapshotDigest     string    `json:"snapshotDigest"`
	ApprovedBy         string    `json:"approvedBy"`
	ApprovalNote       string    `json:"approvalNote"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationStatus string    `json:"verificationStatus"`
}

type Progress struct {
	TotalPoints          int      `json:"totalPoints"`
	TestedPoints         int      `json:"testedPoints"`
	PassedPoints         int      `json:"passedPoints"`
	OpenDeviations       int      `json:"openDeviations"`
	CoveragePercent      float64  `json:"coveragePercent"`
	FailedAttempts       int      `json:"failedAttempts"`
	CriticalDeviations   int      `json:"criticalDeviations"`
	UntestedPointCodes   []string `json:"untestedPointCodes,omitempty"`
	RiskLevel            string   `json:"riskLevel,omitempty"`
	RiskFlags            []string `json:"riskFlags,omitempty"`
	MinutesToPerformance int64    `json:"minutesToPerformance,omitempty"`
}
