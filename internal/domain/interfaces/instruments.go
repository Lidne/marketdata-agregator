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
	CreateShare(ctx context.Context, share *domain.Share) error
	UpdateShare(ctx context.Context, share *domain.Share) error
	DeleteShare(ctx context.Context, uid uuid.UUID) error
	GetShare(ctx context.Context, uid uuid.UUID) (*domain.Share, error)
	CreateBond(ctx context.Context, bond *domain.Bond) error
	UpdateBond(ctx context.Context, bond *domain.Bond) error
	DeleteBond(ctx context.Context, uid uuid.UUID) error
	GetBond(ctx context.Context, uid uuid.UUID) (*domain.Bond, error)
	CreateFuture(ctx context.Context, future *domain.Future) error
	UpdateFuture(ctx context.Context, future *domain.Future) error
	DeleteFuture(ctx context.Context, uid uuid.UUID) error
	GetFuture(ctx context.Context, uid uuid.UUID) (*domain.Future, error)
	CreateCurrency(ctx context.Context, currency *domain.Currency) error
	UpdateCurrency(ctx context.Context, currency *domain.Currency) error
	DeleteCurrency(ctx context.Context, uid uuid.UUID) error
	GetCurrency(ctx context.Context, uid uuid.UUID) (*domain.Currency, error)
	CreateEtf(ctx context.Context, etf *domain.Etf) error
	UpdateEtf(ctx context.Context, etf *domain.Etf) error
	DeleteEtf(ctx context.Context, uid uuid.UUID) error
	GetEtf(ctx context.Context, uid uuid.UUID) (*domain.Etf, error)
	Close()
}
