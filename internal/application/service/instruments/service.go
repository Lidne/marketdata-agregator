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

func (s *Service) GetShare(ctx context.Context, uid uuid.UUID) (*domain.Share, error) {
	return s.repo.GetShare(ctx, uid)
}

func (s *Service) GetBond(ctx context.Context, uid uuid.UUID) (*domain.Bond, error) {
	return s.repo.GetBond(ctx, uid)
}

func (s *Service) GetFuture(ctx context.Context, uid uuid.UUID) (*domain.Future, error) {
	return s.repo.GetFuture(ctx, uid)
}

func (s *Service) GetCurrency(ctx context.Context, uid uuid.UUID) (*domain.Currency, error) {
	return s.repo.GetCurrency(ctx, uid)
}

func (s *Service) GetEtf(ctx context.Context, uid uuid.UUID) (*domain.Etf, error) {
	return s.repo.GetEtf(ctx, uid)
}

func (s *Service) Close() {
	s.repo.Close()
}
