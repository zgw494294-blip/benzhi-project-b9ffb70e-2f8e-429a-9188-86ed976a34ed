package batchcache_test

import (
	"context"
	"testing"
	"time"

	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/store"
)

func TestBatchGetCacheInvalidation(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(ctx, "file:batch-get-cache-invalidation?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	service := application.NewService(repo)
	created, replayed, err := service.CreateBatch(ctx, application.CreateBatchCommand{
		WriteMeta: application.WriteMeta{ExpectedVersion: 0, IdempotencyKey: "create-cache-test"},
		VenueName: "星海剧场", StageZone: "主舞台", PerformanceAt: time.Now().UTC().Add(24 * time.Hour), OwnerName: "负责人甲",
	})
	if err != nil || replayed {
		t.Fatalf("创建批次失败 err=%v replayed=%v", err, replayed)
	}
	if _, err = service.GetBatch(ctx, created.ID); err != nil {
		t.Fatal(err)
	}

	updated, replayed, err := service.AddPoint(ctx, created.ID, application.AddPointCommand{
		WriteMeta: application.WriteMeta{ExpectedVersion: created.Version, IdempotencyKey: "add-cache-test"},
		PointCode: "P-01", HoistSerial: "H-01", RopeSpec: "6x19-12mm", RatedLoadKg: 1000, PlannedLoadKg: 600, PositionNote: "台口中线",
	})
	if err != nil || replayed || len(updated.Points) != 1 {
		t.Fatalf("登记吊点失败 err=%v replayed=%v points=%d", err, replayed, len(updated.Points))
	}

	fresh, err := service.GetBatch(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Points) != 1 || fresh.Points[0].PointCode != "P-01" {
		got := ""
		if len(fresh.Points) > 0 {
			got = fresh.Points[0].PointCode
		}
		t.Fatalf("写入后详情仍是旧缓存: points=%d, got=%q", len(fresh.Points), got)
	}
}
