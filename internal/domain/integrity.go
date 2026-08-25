package domain

import (
	"fmt"
	"sort"
)

func (b *InspectionBatch) ValidateIntegrity() error {
	if b.ID == "" || b.Version < 1 || b.CreatedAt.IsZero() || b.UpdatedAt.IsZero() {
		return corrupt("批次标识、版本或时间戳无效")
	}
	pointIDs := make(map[string]RiggingPoint, len(b.Points))
	pointCodes := make(map[string]struct{}, len(b.Points))
	hoists := make(map[string]struct{}, len(b.Points))
	for _, p := range b.Points {
		if p.ID == "" || p.BatchID != b.ID {
			return corrupt("吊点标识或批次引用无效")
		}
		if _, exists := pointIDs[p.ID]; exists {
			return corrupt("吊点标识重复")
		}
		if _, exists := pointCodes[p.PointCode]; exists {
			return corrupt("吊点编号重复")
		}
		if _, exists := hoists[p.HoistSerial]; exists {
			return corrupt("葫芦序列号重复")
		}
		if err := p.Validate(); err != nil {
			return corrupt("吊点安全参数不再满足规则")
		}
		pointIDs[p.ID] = p
		pointCodes[p.PointCode] = struct{}{}
		hoists[p.HoistSerial] = struct{}{}
	}
	if b.Status != StatusDraft {
		for _, point := range b.Points {
			if point.LockedAt == nil {
				return corrupt("已锁定流程中的吊点缺少锁定时间")
			}
		}
	}
	testIDs := make(map[string]LoadTest, len(b.Tests))
	attempts := make(map[string][]int, len(b.Points))
	for _, test := range b.Tests {
		point, exists := pointIDs[test.RiggingPointID]
		if !exists || test.BatchID != b.ID || test.ID == "" {
			return corrupt("试验引用了不存在的吊点或批次")
		}
		if _, exists = testIDs[test.ID]; exists {
			return corrupt("试验标识重复")
		}
		result, err := EvaluateTest(point, test.TargetLoadKg, test.MeasuredLoadKg, test.HoldSeconds, test.DisplacementMm)
		if err != nil || result != test.Result {
			return corrupt("试验判定与测量数据不一致")
		}
		if test.AttemptNo < 1 || test.RecordedAt.IsZero() || test.RecordedBy == "" {
			return corrupt("试验次数或记录信息无效")
		}
		testIDs[test.ID] = test
		attempts[test.RiggingPointID] = append(attempts[test.RiggingPointID], test.AttemptNo)
	}
	for _, numbers := range attempts {
		sort.Ints(numbers)
		for i, number := range numbers {
			if number != i+1 {
				return corrupt("吊点试验次数不连续")
			}
		}
	}
	deviationIDs := make(map[string]struct{}, len(b.Deviations))
	for _, deviation := range b.Deviations {
		failed, exists := testIDs[deviation.LoadTestID]
		if !exists || failed.Result != TestFailed || deviation.RiggingPointID != failed.RiggingPointID {
			return corrupt("偏差未关联不合格试验")
		}
		if _, exists = deviationIDs[deviation.ID]; exists || deviation.BatchID != b.ID {
			return corrupt("偏差标识重复或批次引用无效")
		}
		deviationIDs[deviation.ID] = struct{}{}
		if deviation.Status == DeviationClosed {
			closing, found := testIDs[deviation.ClosedByTestID]
			if !found || closing.Result != TestPassed || deviation.ClosedAt == nil || deviation.RemediationEvidence == "" {
				return corrupt("已关闭偏差缺少整改证据或合格复测")
			}
		}
		if deviation.AssigneeConfirmed && (deviation.ConfirmedBy == "" || deviation.ConfirmedAt == nil) {
			return corrupt("偏差责任确认缺少确认人或时间")
		}
	}
	progress := b.Progress()
	switch b.Status {
	case StatusDraft:
	case StatusAwaitingTests, StatusTesting:
		if len(b.Points) == 0 {
			return corrupt("待试验批次没有吊点")
		}
	case StatusBlocked:
		if progress.OpenDeviations == 0 {
			return corrupt("阻断批次没有未关闭偏差")
		}
	case StatusPendingReview:
		if progress.TotalPoints == 0 || progress.PassedPoints != progress.TotalPoints || progress.OpenDeviations != 0 || progress.CriticalDeviations != 0 || hasUnconfirmedDeviation(b.Deviations) {
			return corrupt("待复核批次不满足完整覆盖条件")
		}
	case StatusFrozen:
		if b.Snapshot == nil || b.Review == nil || b.Credential != nil {
			return corrupt("冻结批次缺少快照或复核记录")
		}
	case StatusReleased:
		if b.Snapshot == nil || b.Review == nil || b.Credential == nil {
			return corrupt("放行批次缺少冻结或凭据记录")
		}
	default:
		return corrupt("批次状态值未知")
	}
	if b.Snapshot != nil && b.RecomputeSnapshotDigest() != b.Snapshot.Digest {
		return corrupt("冻结快照摘要与内容不一致")
	}
	// A mismatch is surfaced by the verification endpoint; loading the aggregate must
	// remain possible so operators can investigate a tampered snapshot.
	if b.Review != nil {
		if b.Review.BatchID != b.ID || b.Review.ID == "" || b.Review.ReviewedAt.IsZero() {
			return corrupt("技术复核记录标识或时间无效")
		}
		if b.Review.PointCount != progress.TotalPoints || b.Review.PassedPointCount != progress.TotalPoints || b.Review.TestedPointCount != progress.TotalPoints {
			return corrupt("技术复核记录与当前冻结清单不一致")
		}
	}
	return nil
}

func corrupt(reason string) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, reason)
}
