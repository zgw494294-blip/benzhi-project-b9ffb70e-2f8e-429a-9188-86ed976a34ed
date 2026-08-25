package application

import (
	"context"
	"stage-rigging-release/internal/domain"
)

func (s *Service) Approve(ctx context.Context, batchID string, c ApprovalCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, "approve_batch", func(b *domain.InspectionBatch) error {
		return b.Freeze(c.ApprovedBy, c.ApprovalNote, s.ids.New(), s.ids.New(), s.now())
	})
}

func (s *Service) Issue(ctx context.Context, batchID string, c IssueCommand) (*domain.InspectionBatch, bool, error) {
	if err := validateMeta(c.WriteMeta, false); err != nil {
		return nil, false, err
	}
	serial, err := s.repo.NextCredentialSerial(ctx)
	if err != nil {
		return nil, false, err
	}
	if lookup, ok := s.repo.(IdempotencyLookup); ok {
		if replay, found, err := lookup.LookupIdempotency(ctx, c.IdempotencyKey, "issue_credential", batchID); err != nil {
			return nil, false, err
		} else if found {
			return replay, true, nil
		}
	}
	return s.repo.Mutate(ctx, batchID, c.ExpectedVersion, c.IdempotencyKey, "issue_credential", func(b *domain.InspectionBatch) error {
		return b.IssueCredential(s.ids.New(), serial, c.ApprovedBy, c.ApprovalNote, s.now())
	})
}

type Verification struct {
	Credential       *domain.ReleaseCredential `json:"credential"`
	RecomputedDigest string                    `json:"recomputedDigest"`
	Status           string                    `json:"status"`
}

type ApprovalPreview struct {
	BatchID            string                       `json:"batchId"`
	Version            int64                        `json:"version"`
	Status             domain.BatchStatus           `json:"status"`
	Progress           domain.Progress              `json:"progress"`
	UntestedPointCodes []string                     `json:"untestedPointCodes"`
	LatestResults      map[string]domain.TestResult `json:"latestResults"`
	BlockingReasons    []string                     `json:"blockingReasons"`
}

func (s *Service) ApprovalPreview(ctx context.Context, batchID string) (ApprovalPreview, error) {
	b, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return ApprovalPreview{}, err
	}
	p := b.Progress()
	out := ApprovalPreview{BatchID: b.ID, Version: b.Version, Status: b.Status, Progress: p, UntestedPointCodes: p.UntestedPointCodes, LatestResults: map[string]domain.TestResult{}}
	for _, point := range b.Points {
		if t, ok := b.LatestTest(point.ID); ok {
			out.LatestResults[point.PointCode] = t.Result
		}
	}
	if len(p.UntestedPointCodes) > 0 {
		out.BlockingReasons = append(out.BlockingReasons, "存在未测吊点")
	}
	if p.PassedPoints != p.TotalPoints {
		out.BlockingReasons = append(out.BlockingReasons, "试验覆盖率不足或最新结果不合格")
	}
	if p.OpenDeviations > 0 {
		out.BlockingReasons = append(out.BlockingReasons, "存在未关闭偏差")
	}
	if p.CriticalDeviations > 0 {
		out.BlockingReasons = append(out.BlockingReasons, "存在 critical 高风险偏差")
	}
	for _, d := range b.Deviations {
		if d.Status != domain.DeviationClosed && !d.AssigneeConfirmed {
			out.BlockingReasons = append(out.BlockingReasons, "存在未确认责任人的偏差")
			break
		}
	}
	return out, nil
}

func (s *Service) VerifyBatch(ctx context.Context, batchID string) (Verification, error) {
	b, err := s.repo.Get(ctx, batchID)
	if err != nil {
		return Verification{}, err
	}
	v := Verification{Credential: b.Credential, Status: b.VerifyCredential()}
	if b.Snapshot != nil {
		v.RecomputedDigest = b.RecomputeSnapshotDigest()
	}
	return v, nil
}
