package credential_replay_sequence_test

import (
	"context"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/domain"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func freezeBatch(t *testing.T, service *application.Service, prefix string) *domain.InspectionBatch {
	t.Helper()
	ctx := context.Background()
	b, replayed, err := service.CreateBatch(ctx, application.CreateBatchCommand{
		WriteMeta:     application.WriteMeta{IdempotencyKey: prefix + "-create"},
		VenueName:     "复现剧场",
		StageZone:     "主舞台",
		PerformanceAt: time.Now().UTC().Add(24 * time.Hour),
		OwnerName:     "装台负责人",
	})
	if err != nil || replayed {
		t.Fatalf("创建批次失败: replay=%v err=%v", replayed, err)
	}
	b, replayed, err = service.AddPoint(ctx, b.ID, application.AddPointCommand{
		WriteMeta:     application.WriteMeta{ExpectedVersion: b.Version, IdempotencyKey: prefix + "-point"},
		PointCode:     prefix + "-P01",
		HoistSerial:   prefix + "-H01",
		RopeSpec:      "6x19-12mm",
		RatedLoadKg:   1000,
		PlannedLoadKg: 600,
		PositionNote:  "台口中线",
	})
	if err != nil || replayed {
		t.Fatalf("登记吊点失败: replay=%v err=%v", replayed, err)
	}
	pointID := b.Points[0].ID
	b, replayed, err = service.LockBatch(ctx, b.ID, application.LockCommand{WriteMeta: application.WriteMeta{
		ExpectedVersion: b.Version,
		IdempotencyKey:  prefix + "-lock",
	}})
	if err != nil || replayed {
		t.Fatalf("锁定批次失败: replay=%v err=%v", replayed, err)
	}
	b, replayed, err = service.RecordTest(ctx, b.ID, application.RecordTestCommand{
		WriteMeta:      application.WriteMeta{ExpectedVersion: b.Version, IdempotencyKey: prefix + "-test"},
		RiggingPointID: pointID,
		TargetLoadKg:   750,
		MeasuredLoadKg: 750,
		HoldSeconds:    60,
		DisplacementMm: 1,
		RecordedBy:     "试验记录员",
	})
	if err != nil || replayed {
		t.Fatalf("记录试验失败: replay=%v err=%v", replayed, err)
	}
	b, replayed, err = service.Approve(ctx, b.ID, application.ApprovalCommand{
		WriteMeta:    application.WriteMeta{ExpectedVersion: b.Version, IdempotencyKey: prefix + "-approval"},
		ApprovedBy:   "技术负责人",
		ApprovalNote: "清单完整，批准冻结",
	})
	if err != nil || replayed {
		t.Fatalf("冻结批次失败: replay=%v err=%v", replayed, err)
	}
	return b
}

func TestCredentialReplayDoesNotConsumeSerial(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(ctx, "file:credential-replay-sequence?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := application.NewService(repo)

	issuedBatch := freezeBatch(t, service, "issued")
	issued, replayed, err := service.Issue(ctx, issuedBatch.ID, application.IssueCommand{
		WriteMeta:    application.WriteMeta{ExpectedVersion: issuedBatch.Version, IdempotencyKey: "issued-key"},
		ApprovedBy:   "技术负责人",
		ApprovalNote: "批准放行",
	})
	if err != nil || replayed || issued.Credential.SerialNumber != 1 {
		t.Fatalf("首次签发结果异常: replay=%v err=%v batch=%+v", replayed, err, issued)
	}

	retry, replayed, err := service.Issue(ctx, issuedBatch.ID, application.IssueCommand{
		WriteMeta:    application.WriteMeta{ExpectedVersion: issuedBatch.Version, IdempotencyKey: "issued-key"},
		ApprovedBy:   "技术负责人",
		ApprovalNote: "批准放行",
	})
	if err != nil || !replayed || retry.Credential.SerialNumber != 1 {
		t.Fatalf("幂等重放结果异常: replay=%v err=%v batch=%+v", replayed, err, retry)
	}

	freshBatch := freezeBatch(t, service, "fresh")
	fresh, replayed, err := service.Issue(ctx, freshBatch.ID, application.IssueCommand{
		WriteMeta:    application.WriteMeta{ExpectedVersion: freshBatch.Version, IdempotencyKey: "fresh-key"},
		ApprovedBy:   "技术负责人",
		ApprovalNote: "批准放行",
	})
	if err != nil || replayed {
		t.Fatalf("第二批次签发失败: replay=%v err=%v", replayed, err)
	}
	if fresh.Credential.SerialNumber != 2 {
		t.Fatalf("幂等重放不应消耗序号，下一张凭据序号=%d，期望 2", fresh.Credential.SerialNumber)
	}
}
