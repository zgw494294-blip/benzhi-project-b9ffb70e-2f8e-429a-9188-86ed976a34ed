package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type InspectionBatch struct {
	ID            string             `json:"id"`
	VenueName     string             `json:"venueName"`
	StageZone     string             `json:"stageZone"`
	PerformanceAt time.Time          `json:"performanceAt"`
	OwnerName     string             `json:"ownerName"`
	Status        BatchStatus        `json:"status"`
	Version       int64              `json:"version"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
	Points        []RiggingPoint     `json:"points"`
	Tests         []LoadTest         `json:"tests"`
	Deviations    []Deviation        `json:"deviations"`
	Snapshot      *FrozenSnapshot    `json:"snapshot,omitempty"`
	Review        *TechnicalReview   `json:"review,omitempty"`
	Credential    *ReleaseCredential `json:"credential,omitempty"`
}

func NewBatch(id, venue, zone string, performance time.Time, owner string, now time.Time) (*InspectionBatch, error) {
	if err := validateBatchDetails(venue, zone, performance, owner); err != nil {
		return nil, err
	}
	if id == "" || now.IsZero() {
		return nil, Invalid("batch", "批次标识和创建时间不能为空")
	}
	return &InspectionBatch{ID: id, VenueName: strings.TrimSpace(venue), StageZone: strings.TrimSpace(zone), PerformanceAt: performance.UTC(), OwnerName: strings.TrimSpace(owner), Status: StatusDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (b *InspectionBatch) UpdateDetails(venue, zone string, performance time.Time, owner string) error {
	if b.Status != StatusDraft {
		return ErrStateConflict
	}
	if err := validateBatchDetails(venue, zone, performance, owner); err != nil {
		return err
	}
	b.VenueName = strings.TrimSpace(venue)
	b.StageZone = strings.TrimSpace(zone)
	b.PerformanceAt = performance.UTC()
	b.OwnerName = strings.TrimSpace(owner)
	return nil
}

func validateBatchDetails(venue, zone string, performance time.Time, owner string) error {
	if err := validateRequiredText("venueName", venue, 100); err != nil {
		return err
	}
	if err := validateRequiredText("stageZone", zone, 100); err != nil {
		return err
	}
	if err := validateRequiredText("ownerName", owner, 60); err != nil {
		return err
	}
	if performance.IsZero() {
		return Invalid("performanceAt", "计划演出时间不能为空")
	}
	return nil
}

func (b *InspectionBatch) AddPoint(p RiggingPoint) error {
	if b.Status != StatusDraft {
		return ErrStateConflict
	}
	if err := p.Validate(); err != nil {
		return err
	}
	for _, old := range b.Points {
		if strings.EqualFold(old.PointCode, p.PointCode) {
			return Invalid("pointCode", "同一批次的吊点编号必须唯一")
		}
		if strings.EqualFold(old.HoistSerial, p.HoistSerial) {
			return Invalid("hoistSerial", "同一批次的葫芦序列号必须唯一")
		}
	}
	p.BatchID = b.ID
	review := p.ReviewCapacity()
	p.CapacityReview = &review
	b.Points = append(b.Points, p)
	return nil
}

func (b *InspectionBatch) AddPoints(points []RiggingPoint) error {
	if len(points) == 0 || len(points) > 200 {
		return Invalid("points", "一次最多登记 200 个吊点，且不能为空")
	}
	copyBatch := *b
	copyBatch.Points = append([]RiggingPoint(nil), b.Points...)
	for _, p := range points {
		if err := copyBatch.AddPoint(p); err != nil {
			return err
		}
	}
	b.Points = copyBatch.Points
	return nil
}

func (b *InspectionBatch) Lock(now time.Time) error {
	if b.Status != StatusDraft {
		return ErrStateConflict
	}
	if len(b.Points) == 0 {
		return Invalid("points", "至少登记一个吊点后才能锁定")
	}
	for i := range b.Points {
		t := now.UTC()
		b.Points[i].LockedAt = &t
	}
	b.Status = StatusAwaitingTests
	return nil
}

func (b *InspectionBatch) Point(id string) (RiggingPoint, bool) {
	for _, p := range b.Points {
		if p.ID == id {
			return p, true
		}
	}
	return RiggingPoint{}, false
}

func (b *InspectionBatch) LatestTest(pointID string) (LoadTest, bool) {
	var latest LoadTest
	found := false
	for _, t := range b.Tests {
		if t.RiggingPointID == pointID && (!found || t.AttemptNo > latest.AttemptNo) {
			latest, found = t, true
		}
	}
	return latest, found
}

func (b *InspectionBatch) AddTest(t LoadTest, dev *Deviation) error {
	if b.Status != StatusAwaitingTests && b.Status != StatusTesting && b.Status != StatusBlocked {
		return ErrStateConflict
	}
	p, ok := b.Point(t.RiggingPointID)
	if !ok {
		return Invalid("riggingPointId", "吊点不属于该批次")
	}
	result, err := EvaluateTest(p, t.TargetLoadKg, t.MeasuredLoadKg, t.HoldSeconds, t.DisplacementMm)
	if err != nil {
		return err
	}
	if err := validateRequiredText("recordedBy", t.RecordedBy, 60); err != nil {
		return err
	}
	if prev, exists := b.LatestTest(p.ID); exists {
		t.AttemptNo = prev.AttemptNo + 1
	} else {
		t.AttemptNo = 1
	}
	t.Result, t.BatchID = result, b.ID
	b.Tests = append(b.Tests, t)
	if result == TestFailed {
		if dev == nil {
			return Invalid("deviation", "不合格试验必须登记偏差信息")
		}
		dev.BatchID, dev.LoadTestID, dev.RiggingPointID, dev.Status = b.ID, t.ID, p.ID, DeviationOpen
		if err := dev.ValidateNew(); err != nil {
			b.Tests = b.Tests[:len(b.Tests)-1]
			return err
		}
		b.Deviations = append(b.Deviations, *dev)
		b.Status = StatusBlocked
	} else {
		b.refreshStatus()
	}
	return nil
}

func (b *InspectionBatch) SubmitRemediation(id, evidence string) error {
	if b.Status != StatusBlocked {
		return ErrStateConflict
	}
	if strings.TrimSpace(evidence) == "" {
		return Invalid("remediationEvidence", "整改证据不能为空")
	}
	if err := validateRequiredText("remediationEvidence", evidence, 1000); err != nil {
		return err
	}
	for i := range b.Deviations {
		if b.Deviations[i].ID == id && b.Deviations[i].Status == DeviationOpen {
			b.Deviations[i].RemediationEvidence = strings.TrimSpace(evidence)
			b.Deviations[i].Status = DeviationRemediated
			return nil
		}
	}
	return ErrNotFound
}

func (b *InspectionBatch) CloseDeviation(id string, t LoadTest, now time.Time) error {
	idx := -1
	for i := range b.Deviations {
		if b.Deviations[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	d := &b.Deviations[idx]
	if d.Status != DeviationRemediated {
		return Invalid("deviation", "必须先提交整改证据")
	}
	point, found := b.Point(d.RiggingPointID)
	if !found {
		return ErrCorrupt
	}
	original, found := b.testByID(d.LoadTestID)
	if !found {
		return ErrCorrupt
	}
	if original.TargetLoadKg <= 0 || abs(t.TargetLoadKg-original.TargetLoadKg) > original.TargetLoadKg*0.05 {
		return Invalid("targetLoadKg", "复测目标载荷必须与原试验目标保持 5% 以内误差")
	}
	result, err := EvaluateTest(point, t.TargetLoadKg, t.MeasuredLoadKg, t.HoldSeconds, t.DisplacementMm)
	if err != nil {
		return err
	}
	if err = validateRequiredText("recordedBy", t.RecordedBy, 60); err != nil {
		return err
	}
	t.RiggingPointID, t.BatchID, t.Result = point.ID, b.ID, result
	if previous, exists := b.LatestTest(point.ID); exists {
		t.AttemptNo = previous.AttemptNo + 1
	} else {
		t.AttemptNo = 1
	}
	b.Tests = append(b.Tests, t)
	if result != TestPassed {
		b.Status = StatusBlocked
		d.Status = DeviationOpen
		d.RemediationEvidence = ""
		d.ClosedByTestID = ""
		d.ClosedAt = nil
		return nil
	}
	last := b.Tests[len(b.Tests)-1]
	d.Status, d.ClosedByTestID = DeviationClosed, last.ID
	closed := now.UTC()
	d.ClosedAt = &closed
	b.refreshStatus()
	return nil
}

func (b *InspectionBatch) testByID(id string) (LoadTest, bool) {
	for _, t := range b.Tests {
		if t.ID == id {
			return t, true
		}
	}
	return LoadTest{}, false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (b *InspectionBatch) refreshStatus() {
	progress := b.Progress()
	if progress.OpenDeviations > 0 || progress.CriticalDeviations > 0 || hasUnconfirmedDeviation(b.Deviations) {
		b.Status = StatusBlocked
		return
	}
	if progress.TotalPoints > 0 && progress.PassedPoints == progress.TotalPoints {
		b.Status = StatusPendingReview
		return
	}
	b.Status = StatusTesting
}

func hasUnconfirmedDeviation(ds []Deviation) bool {
	for _, d := range ds {
		if d.Status != DeviationClosed && !d.AssigneeConfirmed {
			return true
		}
	}
	return false
}

func (b *InspectionBatch) Progress() Progress {
	p := Progress{TotalPoints: len(b.Points)}
	for _, point := range b.Points {
		if t, ok := b.LatestTest(point.ID); ok {
			p.TestedPoints++
			if t.Result == TestPassed {
				p.PassedPoints++
			}
		}
	}
	for _, d := range b.Deviations {
		if d.Status != DeviationClosed {
			p.OpenDeviations++
		}
		if d.Severity == SeverityCritical {
			p.CriticalDeviations++
		}
	}
	for _, point := range b.Points {
		if _, ok := b.LatestTest(point.ID); !ok {
			p.UntestedPointCodes = append(p.UntestedPointCodes, point.PointCode)
		}
	}
	for _, t := range b.Tests {
		if t.Result == TestFailed {
			p.FailedAttempts++
		}
	}
	if p.TotalPoints > 0 {
		p.CoveragePercent = float64(p.TestedPoints) * 100 / float64(p.TotalPoints)
	}
	p.RiskFlags = make([]string, 0, 3)
	if len(p.UntestedPointCodes) > 0 {
		p.RiskFlags = append(p.RiskFlags, "untested_points")
	}
	if p.OpenDeviations > 0 {
		p.RiskFlags = append(p.RiskFlags, "open_deviations")
	}
	if p.FailedAttempts > 0 {
		p.RiskFlags = append(p.RiskFlags, "failed_attempts")
	}
	if p.CriticalDeviations > 0 {
		p.RiskFlags = append(p.RiskFlags, "critical_deviation")
	}
	switch {
	case p.CriticalDeviations > 0:
		p.RiskLevel = "critical"
	case p.OpenDeviations > 0 || len(p.UntestedPointCodes) > 0:
		p.RiskLevel = "high"
	case p.FailedAttempts > 0:
		p.RiskLevel = "medium"
	default:
		p.RiskLevel = "low"
	}
	return p
}

// ProgressAt adds schedule-sensitive risk markers to the aggregate progress.
func (b *InspectionBatch) ProgressAt(now time.Time) Progress {
	p := b.Progress()
	minutes := int64(now.UTC().Sub(b.PerformanceAt.UTC()).Minutes())
	p.MinutesToPerformance = -minutes
	if minutes >= 0 {
		p.RiskFlags = append(p.RiskFlags, "performance_overdue")
		if p.RiskLevel == "low" {
			p.RiskLevel = "medium"
		}
	} else if minutes >= -24*60 {
		p.RiskFlags = append(p.RiskFlags, "performance_soon")
		if p.RiskLevel == "low" {
			p.RiskLevel = "medium"
		}
	}
	return p
}

func (b *InspectionBatch) Freeze(approvedBy, note, snapshotID, reviewID string, now time.Time) error {
	if b.Status != StatusPendingReview {
		ids := make([]string, 0)
		reasons := make([]string, 0)
		for _, d := range b.Deviations {
			if d.Status != DeviationClosed || d.Severity == SeverityCritical || !d.AssigneeConfirmed {
				ids = append(ids, d.ID)
				if d.Status != DeviationClosed {
					reasons = append(reasons, "存在未关闭偏差")
				}
				if d.Severity == SeverityCritical {
					reasons = append(reasons, "存在 critical 高风险偏差")
				}
				if !d.AssigneeConfirmed {
					reasons = append(reasons, "存在未确认责任人的偏差")
				}
			}
		}
		p := b.Progress()
		if len(p.UntestedPointCodes) > 0 {
			reasons = append(reasons, "存在未测吊点")
		}
		if p.PassedPoints != p.TotalPoints {
			reasons = append(reasons, "存在不合格最新试验或覆盖率不足")
		}
		if len(ids) > 0 {
			return &ApprovalBlockedError{DeviationIDs: ids, Reasons: reasons}
		}
		if len(reasons) > 0 {
			return &ApprovalBlockedError{Reasons: reasons}
		}
		return ErrStateConflict
	}
	if err := validateRequiredText("approvedBy", approvedBy, 60); err != nil {
		return err
	}
	if err := validateRequiredText("approvalNote", note, 400); err != nil {
		return err
	}
	p := b.Progress()
	if p.PassedPoints != p.TotalPoints || p.OpenDeviations != 0 || p.CriticalDeviations != 0 || hasUnconfirmedDeviation(b.Deviations) {
		return Invalid("review", "清单覆盖率或偏差关闭情况不满足冻结要求")
	}
	content, digest, err := b.SnapshotData()
	if err != nil {
		return err
	}
	b.Snapshot = &FrozenSnapshot{ID: snapshotID, BatchID: b.ID, Digest: digest, Content: content, CreatedAt: now.UTC()}
	closed := 0
	for _, d := range b.Deviations {
		if d.Status == DeviationClosed {
			closed++
		}
	}
	b.Review = &TechnicalReview{ID: reviewID, BatchID: b.ID, ApprovedBy: strings.TrimSpace(approvedBy), ApprovalNote: strings.TrimSpace(note), PointCount: p.TotalPoints, TestedPointCount: p.TestedPoints, PassedPointCount: p.PassedPoints, ClosedDeviationCount: closed, ReviewedAt: now.UTC()}
	b.Status = StatusFrozen
	return nil
}

func (b *InspectionBatch) SnapshotData() (string, string, error) {
	type item struct {
		PointCode, HoistSerial, RopeSpec string
		RatedLoadKg, PlannedLoadKg       float64
		TestID                           string
		MeasuredLoadKg, DisplacementMm   float64
		HoldSeconds                      int
	}
	items := make([]item, 0, len(b.Points))
	for _, p := range b.Points {
		t, ok := b.LatestTest(p.ID)
		if !ok || t.Result != TestPassed {
			return "", "", fmt.Errorf("%w: 吊点 %s 无合格试验", ErrValidation, p.PointCode)
		}
		items = append(items, item{p.PointCode, p.HoistSerial, p.RopeSpec, p.RatedLoadKg, p.PlannedLoadKg, t.ID, t.MeasuredLoadKg, t.DisplacementMm, t.HoldSeconds})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].PointCode < items[j].PointCode })
	payload := struct {
		BatchID, VenueName, StageZone, PerformanceAt string
		Points                                       []item
	}{b.ID, b.VenueName, b.StageZone, b.PerformanceAt.UTC().Format(time.RFC3339Nano), items}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(raw)
	return string(raw), hex.EncodeToString(sum[:]), nil
}

func (b *InspectionBatch) IssueCredential(id string, serial int64, approvedBy, note string, now time.Time) error {
	if b.Status != StatusFrozen || b.Snapshot == nil || b.Review == nil {
		return ErrStateConflict
	}
	if b.Credential != nil {
		return Invalid("credential", "该批次已经签发放行凭据")
	}
	if id == "" || serial < 1 || now.IsZero() {
		return Invalid("credential", "凭据标识、序号和签发时间无效")
	}
	if err := validateRequiredText("approvedBy", approvedBy, 60); err != nil {
		return err
	}
	if err := validateRequiredText("approvalNote", note, 400); err != nil {
		return err
	}
	if strings.TrimSpace(approvedBy) != b.Review.ApprovedBy {
		return Invalid("approvedBy", "签发人必须与技术复核批准人一致")
	}
	b.Credential = &ReleaseCredential{ID: id, BatchID: b.ID, SerialNumber: serial, SnapshotDigest: b.Snapshot.Digest, ApprovedBy: approvedBy, ApprovalNote: note, IssuedAt: now.UTC(), VerificationStatus: "valid"}
	b.Status = StatusReleased
	return nil
}

func (b *InspectionBatch) VerifyCredential() string {
	if b.Credential == nil || b.Snapshot == nil {
		return "missing"
	}
	sum := sha256.Sum256([]byte(b.Snapshot.Content))
	if hex.EncodeToString(sum[:]) != b.Credential.SnapshotDigest || b.Snapshot.Digest != b.Credential.SnapshotDigest {
		return "mismatch"
	}
	return "valid"
}

func (b *InspectionBatch) RecomputeSnapshotDigest() string {
	if b.Snapshot == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(b.Snapshot.Content))
	return hex.EncodeToString(sum[:])
}
