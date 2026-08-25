package domain

import (
	"errors"
	"testing"
	"time"
)

func TestBatchDeviationRetestAndRelease(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	b, err := NewBatch("batch-1", "星海剧场", "主舞台", now.Add(24*time.Hour), "负责人甲", now)
	if err != nil {
		t.Fatal(err)
	}
	p := RiggingPoint{ID: "point-1", PointCode: "P-01", HoistSerial: "H-01", RopeSpec: "6x19-12mm", RatedLoadKg: 1000, PlannedLoadKg: 600}
	if err = b.AddPoint(p); err != nil {
		t.Fatal(err)
	}
	if err = b.Lock(now); err != nil {
		t.Fatal(err)
	}
	failed := LoadTest{ID: "test-1", RiggingPointID: p.ID, TargetLoadKg: 750, MeasuredLoadKg: 700, HoldSeconds: 45, DisplacementMm: 6, RecordedBy: "记录员", RecordedAt: now}
	deviation := Deviation{ID: "dev-1", Severity: SeverityMajor, Symptom: "位移超限", RequiredAction: "重新张紧", Assignee: "技师甲"}
	if err = b.AddTest(failed, &deviation); err != nil {
		t.Fatal(err)
	}
	if b.Status != StatusBlocked {
		t.Fatalf("状态=%s", b.Status)
	}
	if err = b.SubmitRemediation("dev-1", "复紧记录与照片编号 E-01"); err != nil {
		t.Fatal(err)
	}
	retest := LoadTest{ID: "test-2", TargetLoadKg: 750, MeasuredLoadKg: 752, HoldSeconds: 90, DisplacementMm: 1.2, RecordedBy: "记录员", RecordedAt: now.Add(time.Hour)}
	if err = b.CloseDeviation("dev-1", retest, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if b.Status != StatusPendingReview {
		t.Fatalf("复测后状态=%s", b.Status)
	}
	if err = b.Freeze("负责人乙", "复核通过", "snapshot-1", "review-1", now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	first := b.Snapshot.Digest
	_, second, err := b.SnapshotData()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("规范化摘要不稳定")
	}
	if err = b.IssueCredential("credential-1", 1, "负责人乙", "准予放行", now.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := b.VerifyCredential(); got != "valid" {
		t.Fatalf("核验=%s", got)
	}
}

func TestPointSafetyLimit(t *testing.T) {
	p := RiggingPoint{PointCode: "P", HoistSerial: "H", RopeSpec: "R", RatedLoadKg: 1000, PlannedLoadKg: 801}
	var validation *ValidationError
	if err := p.Validate(); !errors.As(err, &validation) {
		t.Fatalf("期望 ValidationError，得到 %v", err)
	}
}

func TestCannotFreezeIncompleteBatch(t *testing.T) {
	now := time.Now().UTC()
	b, _ := NewBatch("b", "剧场", "区域", now.Add(time.Hour), "负责人", now)
	_ = b.AddPoint(RiggingPoint{ID: "p", PointCode: "P", HoistSerial: "H", RopeSpec: "R", RatedLoadKg: 1000, PlannedLoadKg: 500})
	_ = b.Lock(now)
	if err := b.Freeze("批准人", "意见", "s", "r", now); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("期望状态冲突，得到 %v", err)
	}
}
