package domain

type BatchStatus string

const (
	StatusDraft         BatchStatus = "draft"
	StatusAwaitingTests BatchStatus = "awaiting_tests"
	StatusTesting       BatchStatus = "testing"
	StatusBlocked       BatchStatus = "blocked"
	StatusPendingReview BatchStatus = "pending_review"
	StatusFrozen        BatchStatus = "frozen"
	StatusReleased      BatchStatus = "released"
)

type TestResult string

const (
	TestPassed TestResult = "passed"
	TestFailed TestResult = "failed"
)

type DeviationStatus string

const (
	DeviationOpen       DeviationStatus = "open"
	DeviationRemediated DeviationStatus = "remediated"
	DeviationClosed     DeviationStatus = "closed"
)

type DeviationSeverity string

const (
	SeverityMinor    DeviationSeverity = "minor"
	SeverityMajor    DeviationSeverity = "major"
	SeverityCritical DeviationSeverity = "critical"
)
