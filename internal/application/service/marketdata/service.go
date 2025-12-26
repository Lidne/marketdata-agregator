package marketdata

import (
	"context"
	"errors"
	"time"

	marketdata "main/internal/domain/entity/marketdata"
	interfaces "main/internal/domain/interfaces"

	"github.com/google/uuid"
)

var (
	ErrNilTrade        = errors.New("trade is nil")
	ErrNilCandle       = errors.New("candle is nil")
	ErrNilOrderBook    = errors.New("order book snapshot is nil")
	ErrInvalidLimit    = errors.New("limit must be positive")
	ErrInvalidInterval = errors.New("interval seconds must be positive")
)

type Service struct {
	repo interfaces.MarketDataRepository
}

func NewService(repo interfaces.MarketDataRepository) *Service {
	return &Service{repo: repo}
}

// Trades

func (s *Service) AddTrade(ctx context.Context, trade *marketdata.Trade) error {
	if trade == nil {
		return ErrNilTrade
	}
	return s.repo.AddTrade(ctx, trade)
}

func (s *Service) AddTrades(ctx context.Context, trades []marketdata.Trade) error {
	if len(trades) == 0 {
		return nil
	}
	return s.repo.AddTrades(ctx, trades)
}

func (s *Service) GetTradesBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time) ([]marketdata.Trade, error) {
	if from.After(to) {
		from, to = to, from
	}
	return s.repo.GetTradesBetween(ctx, instrumentUID, from, to)
}

func (s *Service) GetLastTrades(ctx context.Context, instrumentUID uuid.UUID, limit int) ([]marketdata.Trade, error) {
	if limit <= 0 {
		return nil, ErrInvalidLimit
	}
	return s.repo.GetLastTrades(ctx, instrumentUID, limit)
}

// Candles

func (s *Service) AddCandle(ctx context.Context, candle *marketdata.Candle) error {
	if candle == nil {
		return ErrNilCandle
	}
	return s.repo.AddCandle(ctx, candle)
}

func (s *Service) AddCandles(ctx context.Context, candles []marketdata.Candle) error {
	if len(candles) == 0 {
		return nil
	}
	return s.repo.AddCandles(ctx, candles)
}

func (s *Service) GetCandlesBetween(ctx context.Context, instrumentUID uuid.UUID, intervalSeconds int64, from, to time.Time) ([]marketdata.Candle, error) {
	if intervalSeconds <= 0 {
		return nil, ErrInvalidInterval
	}
	if from.After(to) {
		from, to = to, from
	}
	return s.repo.GetCandlesBetween(ctx, instrumentUID, from, to, intervalSeconds)
}

func (s *Service) GetLastCandles(ctx context.Context, instrumentUID uuid.UUID, intervalSeconds int64, limit int) ([]marketdata.Candle, error) {
	if intervalSeconds <= 0 {
		return nil, ErrInvalidInterval
	}
	if limit <= 0 {
		return nil, ErrInvalidLimit
	}
	return s.repo.GetLastCandles(ctx, instrumentUID, intervalSeconds, limit)
}

// Order book snapshots

func (s *Service) AddOrderBookSnapshot(ctx context.Context, snapshot *marketdata.OrderBookSnapshot) error {
	if snapshot == nil {
		return ErrNilOrderBook
	}
	return s.repo.AddOrderBookSnapshot(ctx, snapshot)
}

func (s *Service) AddOrderBookSnapshots(ctx context.Context, snapshots []marketdata.OrderBookSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	return s.repo.AddOrderBookSnapshots(ctx, snapshots)
}

func (s *Service) GetOrderBookSnapshotsBetween(ctx context.Context, instrumentUID uuid.UUID, depth int32, from, to time.Time) ([]marketdata.OrderBookSnapshot, error) {
	if depth <= 0 {
		return nil, errors.New("depth must be positive")
	}
	if from.After(to) {
		from, to = to, from
	}
	return s.repo.GetOrderBookSnapshotsBetween(ctx, instrumentUID, from, to, depth)
}

func (s *Service) GetLastOrderBookSnapshots(ctx context.Context, instrumentUID uuid.UUID, depth int32, limit int) ([]marketdata.OrderBookSnapshot, error) {
	if depth <= 0 {
		return nil, errors.New("depth must be positive")
	}
	if limit <= 0 {
		return nil, ErrInvalidLimit
	}
	return s.repo.GetLastOrderBookSnapshots(ctx, instrumentUID, depth, limit)
}

func (s *Service) Close() {
	s.repo.Close()
}
