package audit_transaction_atomicity_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"stage-rigging-release/internal/domain"
	"stage-rigging-release/internal/store"
	"testing"
	"time"
)

func TestAuditFailureRollsBackMutation(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "audit-atomicity.db")
	repo, err := store.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	batch, err := domain.NewBatch("batch-audit", "测试剧场", "主舞台", now.Add(24*time.Hour), "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.Create(ctx, "create-audit-batch", batch); err != nil {
		t.Fatal(err)
	}

	control, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	_, err = control.ExecContext(ctx, `CREATE TRIGGER reject_add_point_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.action = 'add_point'
		BEGIN SELECT RAISE(ABORT, 'forced audit failure'); END`)
	if err != nil {
		t.Fatal(err)
	}

	_, _, mutationErr := repo.Mutate(ctx, batch.ID, batch.Version, "add-point-audit-failure", "add_point", func(current *domain.InspectionBatch) error {
		return current.AddPoint(domain.RiggingPoint{
			ID: "point-audit", PointCode: "P-AUDIT", HoistSerial: "H-AUDIT",
			RopeSpec: "6x19-12mm", RatedLoadKg: 1000, PlannedLoadKg: 600,
		})
	})
	if mutationErr == nil {
		t.Fatal("审计写入被拒绝时变更应返回错误")
	}

	loaded, err := repo.Get(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, replayed, retryErr := repo.Mutate(ctx, batch.ID, batch.Version, "add-point-audit-failure", "add_point", func(current *domain.InspectionBatch) error {
		return current.AddPoint(domain.RiggingPoint{
			ID: "point-audit", PointCode: "P-AUDIT", HoistSerial: "H-AUDIT",
			RopeSpec: "6x19-12mm", RatedLoadKg: 1000, PlannedLoadKg: 600,
		})
	})
	if loaded.Version != batch.Version || len(loaded.Points) != 0 || replayed {
		t.Fatalf("审计失败后业务变更仍被持久化并污染重试: version=%d points=%d replayed=%t", loaded.Version, len(loaded.Points), replayed)
	}
	if retryErr == nil {
		t.Fatal("审计仍被拒绝时，未提交的同键重试应再次返回错误")
	}
}
