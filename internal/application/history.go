package application

import (
	"context"
	"stage-rigging-release/internal/domain"
	"strings"
)

type TestHistory struct {
	BatchID             string                       `json:"batchId"`
	Tests               []domain.LoadTest            `json:"tests"`
	LatestAttempt       map[string]int               `json:"latestAttempt"`
	LatestResult        map[string]domain.TestResult `json:"latestResult"`
	LatestAttemptNo     map[string]int               `json:"latestAttemptNo"`
	LatestResultByPoint map[string]domain.TestResult `json:"latestResultByPoint"`
	FailedAttempts      int                          `json:"failedAttempts"`
	TestedPoints        int                          `json:"testedPoints"`
	PassedPoints        int                          `json:"passedPoints"`
	UntestedPointCodes  []string                     `json:"untestedPointCodes"`
	CoveragePercent     float64                      `json:"coveragePercent"`
}

type historyLoad struct {
	batch *domain.InspectionBatch
	err   error
}

// loadHistoryBatch dispatches the storage read on a background goroutine so
// the caller can abandon the wait as soon as the request context is cancelled.
// The request context is forwarded to the repository so the SQL driver can
// interrupt the in-progress query; the buffered channel lets the goroutine
// publish its result and exit even when the caller has already returned,
// preventing a leak when cancellation wins the select.
func loadHistoryBatch(ctx context.Context, repo Repository, batchID string) (*domain.InspectionBatch, error) {
	result := make(chan historyLoad, 1)
	go func() {
		batch, err := repo.Get(ctx, batchID)
		result <- historyLoad{batch: batch, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case loaded := <-result:
		return loaded.batch, loaded.err
	}
}

func (s *Service) TestHistory(ctx context.Context, batchID, pointCode, result string) (TestHistory, error) {
	b, err := loadHistoryBatch(ctx, s.repo, batchID)
	if err != nil {
		return TestHistory{}, err
	}
	if result != "" && result != string(domain.TestPassed) && result != string(domain.TestFailed) {
		return TestHistory{}, domain.Invalid("result", "结果必须为 passed 或 failed")
	}
	pointID := ""
	if pointCode != "" {
		for _, p := range b.Points {
			if strings.EqualFold(p.PointCode, pointCode) {
				pointID = p.ID
				break
			}
		}
		if pointID == "" {
			return TestHistory{}, domain.Invalid("pointCode", "吊点不属于该批次")
		}
	}
	h := TestHistory{BatchID: batchID, LatestAttempt: map[string]int{}, LatestResult: map[string]domain.TestResult{}, LatestAttemptNo: map[string]int{}, LatestResultByPoint: map[string]domain.TestResult{}}
	for _, t := range b.Tests {
		// Latest-attempt maps always use the full history, even when the rows below are filtered.
		if t.AttemptNo > h.LatestAttempt[t.RiggingPointID] {
			h.LatestAttempt[t.RiggingPointID] = t.AttemptNo
			h.LatestResult[t.RiggingPointID] = t.Result
			for _, p := range b.Points {
				if p.ID == t.RiggingPointID {
					h.LatestAttemptNo[p.PointCode] = t.AttemptNo
					h.LatestResultByPoint[p.PointCode] = t.Result
					break
				}
			}
		}
		if pointID != "" && t.RiggingPointID != pointID {
			continue
		}
		if t.Result == domain.TestFailed {
			h.FailedAttempts++
		}
		if result != "" && string(t.Result) != result {
			continue
		}
		h.Tests = append(h.Tests, t)
	}
	for _, p := range b.Points {
		if t, ok := b.LatestTest(p.ID); ok {
			h.TestedPoints++
			if t.Result == domain.TestPassed {
				h.PassedPoints++
			}
		} else {
			h.UntestedPointCodes = append(h.UntestedPointCodes, p.PointCode)
		}
	}
	if len(b.Points) > 0 {
		h.CoveragePercent = float64(h.TestedPoints) * 100 / float64(len(b.Points))
	}
	return h, nil
}
