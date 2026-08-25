package store

import (
	"context"
	"errors"
	"stage-rigging-release/internal/domain"
	"testing"
	"time"
)

func TestVersionIdempotencyAndPersistence(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, "file:store-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	b, err := domain.NewBatch("batch", "剧场", "主舞台", now.Add(time.Hour), "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := s.Create(ctx, "create-key", b)
	if err != nil || replayed {
		t.Fatalf("创建 err=%v replay=%v", err, replayed)
	}
	retry, replayed, err := s.Create(ctx, "create-key", b)
	if err != nil || !replayed || retry.ID != created.ID {
		t.Fatalf("重放 err=%v replay=%v", err, replayed)
	}
	updated, replayed, err := s.Mutate(ctx, b.ID, 1, "point-key", "add_point", func(current *domain.InspectionBatch) error {
		return current.AddPoint(domain.RiggingPoint{ID: "point", PointCode: "P-01", HoistSerial: "H-01", RopeSpec: "6x19", RatedLoadKg: 1000, PlannedLoadKg: 600})
	})
	if err != nil || replayed {
		t.Fatalf("变更 err=%v", err)
	}
	if updated.Version != 2 || len(updated.Points) != 1 {
		t.Fatalf("持久化结果异常: %+v", updated)
	}
	_, _, err = s.Mutate(ctx, b.ID, 1, "stale-key", "lock_batch", func(current *domain.InspectionBatch) error { return current.Lock(now) })
	if !errors.Is(err, domain.ErrVersion) {
		t.Fatalf("期望版本冲突，得到 %v", err)
	}
	loaded, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 2 || loaded.Points[0].PointCode != "P-01" {
		t.Fatalf("读取异常: %+v", loaded)
	}
}

func TestImmutableSnapshotTrigger(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, "file:immutable-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	b, _ := domain.NewBatch("batch", "剧场", "主舞台", now.Add(time.Hour), "负责人", now)
	if _, _, err = s.Create(ctx, "create", b); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO frozen_snapshots(id,batch_id,digest,content,created_at) VALUES(?,?,?,?,?)`, "snap", b.ID, "digest", "{}", timeText(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE frozen_snapshots SET digest=? WHERE id=?`, "changed", "snap"); err == nil {
		t.Fatal("冻结快照被意外更新")
	}
}
