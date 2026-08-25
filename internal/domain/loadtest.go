package domain

import "time"

type LoadTest struct {
	ID             string     `json:"id"`
	BatchID        string     `json:"batchId"`
	RiggingPointID string     `json:"riggingPointId"`
	AttemptNo      int        `json:"attemptNo"`
	TargetLoadKg   float64    `json:"targetLoadKg"`
	MeasuredLoadKg float64    `json:"measuredLoadKg"`
	HoldSeconds    int        `json:"holdSeconds"`
	DisplacementMm float64    `json:"displacementMm"`
	Result         TestResult `json:"result"`
	RecordedBy     string     `json:"recordedBy"`
	RecordedAt     time.Time  `json:"recordedAt"`
}

func EvaluateTest(point RiggingPoint, target, measured float64, hold int, displacement float64) (TestResult, error) {
	if !validPositiveNumber(target) || target > point.RatedLoadKg {
		return "", Invalid("targetLoadKg", "目标载荷必须大于 0 且不超过额定载荷")
	}
	if !validPositiveNumber(measured) {
		return "", Invalid("measuredLoadKg", "实测载荷必须大于 0")
	}
	if hold < 0 || !validNonNegativeNumber(displacement) {
		return "", Invalid("measurement", "保持时长和位移不能为负数")
	}
	if measured < target*0.95 || measured > target*1.10 || hold < 60 || displacement > 5 {
		return TestFailed, nil
	}
	return TestPassed, nil
}
