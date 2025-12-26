package interfaces

import (
	"context"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
)

type InstrumentsRepository interface {
	CreateInstrument(ctx context.Context, instrument *domain.Instrument) error
	GetInstrument(ctx context.Context, uid uuid.UUID) (*domain.Instrument, error)
	UpdateInstrument(ctx context.Context, instrument *domain.Instrument) error
	DeleteInstrument(ctx context.Context, uid uuid.UUID) error
	GetShare(ctx context.Context, uid uuid.UUID) (*domain.Share, error)
	GetBond(ctx context.Context, uid uuid.UUID) (*domain.Bond, error)
	GetFuture(ctx context.Context, uid uuid.UUID) (*domain.Future, error)
	GetCurrency(ctx context.Context, uid uuid.UUID) (*domain.Currency, error)
	GetEtf(ctx context.Context, uid uuid.UUID) (*domain.Etf, error)
	Close()
}
