package application

import (
	"context"
	"stage-rigging-release/internal/domain"
)

type Repository interface {
	Create(context.Context, string, *domain.InspectionBatch) (*domain.InspectionBatch, bool, error)
	Mutate(context.Context, string, int64, string, string, func(*domain.InspectionBatch) error) (*domain.InspectionBatch, bool, error)
	Get(context.Context, string) (*domain.InspectionBatch, error)
	List(context.Context, int) ([]domain.InspectionBatch, error)
	GetCredential(context.Context, int64) (*domain.ReleaseCredential, error)
	NextCredentialSerial(context.Context) (int64, error)
	ListAudit(context.Context, string, int) ([]domain.AuditEvent, error)
}

type IdempotencyLookup interface {
	LookupIdempotency(context.Context, string, string, string) (*domain.InspectionBatch, bool, error)
}

type IDGenerator interface{ New() string }
