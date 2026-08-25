package application

import (
	"context"
	"errors"
	"stage-rigging-release/internal/domain"
	"strings"
)

type PointLineError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}
type PointPrecheckResult struct {
	Precheck              bool             `json:"precheck"`
	Errors                []PointLineError `json:"errors,omitempty"`
	RatedLoadTotalKg      float64          `json:"ratedLoadTotalKg"`
	PlannedLoadTotalKg    float64          `json:"plannedLoadTotalKg"`
	MaxUtilizationPercent float64          `json:"maxUtilizationPercent"`
	ItemCount             int              `json:"itemCount"`
}

func (s *Service) PrecheckPoints(ctx context.Context, batchID string, c AddPointCommand) (PointPrecheckResult, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return PointPrecheckResult{}, err
	}
	b, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return PointPrecheckResult{}, err
	}
	if b.Version != c.ExpectedVersion {
		return PointPrecheckResult{}, domain.ErrVersion
	}
	inputs := c.Points
	if len(inputs) == 0 {
		inputs = c.Items
	}
	if len(inputs) == 0 {
		inputs = []PointInput{{PointCode: c.PointCode, HoistSerial: c.HoistSerial, RopeSpec: c.RopeSpec, RatedLoadKg: c.RatedLoadKg, PlannedLoadKg: c.PlannedLoadKg, PositionNote: c.PositionNote}}
	}
	out := PointPrecheckResult{Precheck: true, ItemCount: len(inputs)}
	if len(inputs) == 0 || len(inputs) > 200 {
		out.Errors = append(out.Errors, PointLineError{Row: 0, Field: "items", Message: "一次最多登记 200 个吊点，且不能为空"})
		return out, nil
	}
	codes, hoists := map[string]int{}, map[string]int{}
	for _, p := range b.Points {
		codes[strings.ToLower(strings.TrimSpace(p.PointCode))] = 0
		hoists[strings.ToLower(strings.TrimSpace(p.HoistSerial))] = 0
	}
	for i, in := range inputs {
		row := i + 1
		p := domain.RiggingPoint{PointCode: in.PointCode, HoistSerial: in.HoistSerial, RopeSpec: in.RopeSpec, RatedLoadKg: in.RatedLoadKg, PlannedLoadKg: in.PlannedLoadKg, PositionNote: in.PositionNote}
		if err := p.Validate(); err != nil {
			var ve *domain.ValidationError
			if errors.As(err, &ve) {
				for _, problem := range ve.Problems {
					out.Errors = append(out.Errors, PointLineError{Row: row, Field: problem.Field, Message: problem.Message})
				}
			}
		}
		code := strings.ToLower(strings.TrimSpace(in.PointCode))
		hoist := strings.ToLower(strings.TrimSpace(in.HoistSerial))
		if code != "" {
			if prev, ok := codes[code]; ok {
				field := "pointCode"
				msg := "吊点编号与已有或本批次其他行重复"
				if prev > 0 {
					msg = "输入行吊点编号重复"
				}
				out.Errors = append(out.Errors, PointLineError{Row: row, Field: field, Message: msg})
			}
			codes[code] = row
		}
		if hoist != "" {
			if prev, ok := hoists[hoist]; ok {
				msg := "葫芦序列号与已有或本批次其他行重复"
				if prev > 0 {
					msg = "输入行葫芦序列号重复"
				}
				out.Errors = append(out.Errors, PointLineError{Row: row, Field: "hoistSerial", Message: msg})
			}
			hoists[hoist] = row
		}
		if p.RatedLoadKg > 0 {
			out.RatedLoadTotalKg += p.RatedLoadKg
			out.PlannedLoadTotalKg += p.PlannedLoadKg
			util := p.PlannedLoadKg / p.RatedLoadKg * 100
			if util > out.MaxUtilizationPercent {
				out.MaxUtilizationPercent = util
			}
		}
	}
	return out, nil
}

func (s *Service) AddPoint(ctx context.Context, batchID string, c AddPointCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, "add_point", func(b *domain.InspectionBatch) error {
		inputs := c.Points
		if len(inputs) == 0 {
			inputs = c.Items
		}
		if len(inputs) > 0 {
			pts := make([]domain.RiggingPoint, 0, len(inputs))
			for _, in := range inputs {
				pts = append(pts, domain.RiggingPoint{ID: s.ids.New(), PointCode: in.PointCode, HoistSerial: in.HoistSerial, RopeSpec: in.RopeSpec, RatedLoadKg: in.RatedLoadKg, PlannedLoadKg: in.PlannedLoadKg, PositionNote: in.PositionNote})
			}
			return b.AddPoints(pts)
		}
		return b.AddPoint(domain.RiggingPoint{ID: s.ids.New(), PointCode: c.PointCode, HoistSerial: c.HoistSerial, RopeSpec: c.RopeSpec, RatedLoadKg: c.RatedLoadKg, PlannedLoadKg: c.PlannedLoadKg, PositionNote: c.PositionNote})
	})
}
