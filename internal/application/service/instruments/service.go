package instruments

import (
	"context"
	"errors"

	domain "main/internal/domain/entity/instruments"
	interfaces "main/internal/domain/interfaces"

	"github.com/google/uuid"
)

var ErrNilInstrument = errors.New("instrument is nil")

type Service struct {
	repo interfaces.InstrumentsRepository
}

func NewService(repo interfaces.InstrumentsRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateInstrument(ctx context.Context, instrument *domain.Instrument) error {
	if instrument == nil {
		return ErrNilInstrument
	}
	return s.repo.CreateInstrument(ctx, instrument)
}

func (s *Service) GetInstrument(ctx context.Context, uid uuid.UUID) (*domain.Instrument, error) {
	return s.repo.GetInstrument(ctx, uid)
}

func (s *Service) UpdateInstrument(ctx context.Context, instrument *domain.Instrument) error {
	if instrument == nil {
		return ErrNilInstrument
	}
	return s.repo.UpdateInstrument(ctx, instrument)
}

func (s *Service) DeleteInstrument(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeleteInstrument(ctx, uid)
}

func (s *Service) CreateShare(ctx context.Context, share *domain.Share) error {
	if share == nil {
		return ErrNilInstrument
	}
	return s.repo.CreateShare(ctx, share)
}

func (s *Service) UpdateShare(ctx context.Context, share *domain.Share) error {
	if share == nil {
		return ErrNilInstrument
	}
	return s.repo.UpdateShare(ctx, share)
}

func (s *Service) DeleteShare(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeleteShare(ctx, uid)
}

func (s *Service) GetShare(ctx context.Context, uid uuid.UUID) (*domain.Share, error) {
	return s.repo.GetShare(ctx, uid)
}

func (s *Service) CreateBond(ctx context.Context, bond *domain.Bond) error {
	if bond == nil {
		return ErrNilInstrument
	}
	return s.repo.CreateBond(ctx, bond)
}

func (s *Service) UpdateBond(ctx context.Context, bond *domain.Bond) error {
	if bond == nil {
		return ErrNilInstrument
	}
	return s.repo.UpdateBond(ctx, bond)
}

func (s *Service) DeleteBond(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeleteBond(ctx, uid)
}

func (s *Service) GetBond(ctx context.Context, uid uuid.UUID) (*domain.Bond, error) {
	return s.repo.GetBond(ctx, uid)
}

func (s *Service) CreateFuture(ctx context.Context, future *domain.Future) error {
	if future == nil {
		return ErrNilInstrument
	}
	return s.repo.CreateFuture(ctx, future)
}

func (s *Service) UpdateFuture(ctx context.Context, future *domain.Future) error {
	if future == nil {
		return ErrNilInstrument
	}
	return s.repo.UpdateFuture(ctx, future)
}

func (s *Service) DeleteFuture(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeleteFuture(ctx, uid)
}

func (s *Service) GetFuture(ctx context.Context, uid uuid.UUID) (*domain.Future, error) {
	return s.repo.GetFuture(ctx, uid)
}

func (s *Service) CreateCurrency(ctx context.Context, currency *domain.Currency) error {
	if currency == nil {
		return ErrNilInstrument
	}
	return s.repo.CreateCurrency(ctx, currency)
}

func (s *Service) UpdateCurrency(ctx context.Context, currency *domain.Currency) error {
	if currency == nil {
		return ErrNilInstrument
	}
	return s.repo.UpdateCurrency(ctx, currency)
}

func (s *Service) DeleteCurrency(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeleteCurrency(ctx, uid)
}

func (s *Service) GetCurrency(ctx context.Context, uid uuid.UUID) (*domain.Currency, error) {
	return s.repo.GetCurrency(ctx, uid)
}

func (s *Service) CreateEtf(ctx context.Context, etf *domain.Etf) error {
	if etf == nil {
		return ErrNilInstrument
	}
	return s.repo.CreateEtf(ctx, etf)
}

func (s *Service) UpdateEtf(ctx context.Context, etf *domain.Etf) error {
	if etf == nil {
		return ErrNilInstrument
	}
	return s.repo.UpdateEtf(ctx, etf)
}

func (s *Service) DeleteEtf(ctx context.Context, uid uuid.UUID) error {
	return s.repo.DeleteEtf(ctx, uid)
}

func (s *Service) GetEtf(ctx context.Context, uid uuid.UUID) (*domain.Etf, error) {
	return s.repo.GetEtf(ctx, uid)
}

func (s *Service) Close() {
	s.repo.Close()
}
