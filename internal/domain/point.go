package domain

import "time"

type RiggingPoint struct {
	ID             string          `json:"id"`
	BatchID        string          `json:"batchId"`
	PointCode      string          `json:"pointCode"`
	HoistSerial    string          `json:"hoistSerial"`
	RopeSpec       string          `json:"ropeSpec"`
	RatedLoadKg    float64         `json:"ratedLoadKg"`
	PlannedLoadKg  float64         `json:"plannedLoadKg"`
	PositionNote   string          `json:"positionNote"`
	LockedAt       *time.Time      `json:"lockedAt,omitempty"`
	CapacityReview *CapacityReview `json:"capacityReview,omitempty"`
}

type CapacityReview struct {
	RatedLoadKg        float64 `json:"ratedLoadKg"`
	PlannedLoadKg      float64 `json:"plannedLoadKg"`
	UtilizationPercent float64 `json:"utilizationPercent"`
	WithinLimit        bool    `json:"withinLimit"`
}

func (p RiggingPoint) Validate() error {
	if err := validateRequiredText("pointCode", p.PointCode, 40); err != nil {
		return err
	}
	if err := validateRequiredText("hoistSerial", p.HoistSerial, 60); err != nil {
		return err
	}
	if err := validateRequiredText("ropeSpec", p.RopeSpec, 80); err != nil {
		return err
	}
	if !validPositiveNumber(p.RatedLoadKg) {
		return Invalid("ratedLoadKg", "额定载荷必须大于 0")
	}
	if !validPositiveNumber(p.PlannedLoadKg) || p.PlannedLoadKg > p.RatedLoadKg*0.8 {
		return Invalid("plannedLoadKg", "计划载荷必须大于 0 且不超过额定载荷的 80%")
	}
	if err := validateOptionalText("positionNote", p.PositionNote, 120); err != nil {
		return err
	}
	return nil
}

func (p RiggingPoint) ReviewCapacity() CapacityReview {
	return CapacityReview{RatedLoadKg: p.RatedLoadKg, PlannedLoadKg: p.PlannedLoadKg, UtilizationPercent: p.PlannedLoadKg / p.RatedLoadKg * 100, WithinLimit: p.PlannedLoadKg <= p.RatedLoadKg*0.8}
}
