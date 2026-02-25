package streamer

import (
	"context"
	"fmt"

	"main/internal/config"
	"main/internal/domain/interfaces"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Service fetches saved market data from the repository and replays it
// by publishing each record to the message broker.
type Service struct {
	marketRepo  interfaces.MarketDataRepository
	instruments interfaces.InstrumentsRepository
	publisher   interfaces.MarketDataPublisher
	logger      *logrus.Logger
}

func NewService(
	marketRepo interfaces.MarketDataRepository,
	instruments interfaces.InstrumentsRepository,
	publisher interfaces.MarketDataPublisher,
	logger *logrus.Logger,
) *Service {
	return &Service{
		marketRepo:  marketRepo,
		instruments: instruments,
		publisher:   publisher,
		logger:      logger,
	}
}

// Stream replays historical market data for each configured instrument
// within the time window defined by cfg.
func (s *Service) Stream(ctx context.Context, cfg config.StreamerConfig) error {
	uids, err := s.resolveInstruments(ctx, cfg.Instruments)
	if err != nil {
		return fmt.Errorf("resolve instruments: %w", err)
	}
	if len(uids) == 0 {
		s.logger.Warn("no instruments resolved, nothing to stream")
		return nil
	}

	for _, uid := range uids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.streamInstrument(ctx, uid, cfg); err != nil {
			return fmt.Errorf("stream instrument %s: %w", uid, err)
		}
	}
	return nil
}

func (s *Service) streamInstrument(ctx context.Context, uid uuid.UUID, cfg config.StreamerConfig) error {
	if cfg.Trades {
		if err := s.streamTrades(ctx, uid, cfg); err != nil {
			return fmt.Errorf("trades: %w", err)
		}
	}
	if cfg.Candles {
		if err := s.streamCandles(ctx, uid, cfg); err != nil {
			return fmt.Errorf("candles: %w", err)
		}
	}
	if cfg.Orders {
		if err := s.streamOrderBooks(ctx, uid, cfg); err != nil {
			return fmt.Errorf("order books: %w", err)
		}
	}
	return nil
}

func (s *Service) resolveInstruments(ctx context.Context, figis []string) ([]uuid.UUID, error) {
	uids := make([]uuid.UUID, 0, len(figis))
	for _, figi := range figis {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		instrument, err := s.instruments.GetInstrumentByFigi(ctx, figi)
		if err != nil {
			s.logger.WithError(err).Warnf("instrument not found for figi %q, skipping", figi)
			continue
		}
		uids = append(uids, instrument.UID)
	}
	return uids, nil
}

func (s *Service) streamTrades(ctx context.Context, uid uuid.UUID, cfg config.StreamerConfig) error {
	trades, err := s.marketRepo.GetTradesBetween(ctx, uid, cfg.From, cfg.To)
	if err != nil {
		return fmt.Errorf("fetch trades: %w", err)
	}

	for i := range trades {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.publisher.PublishTrade(ctx, &trades[i]); err != nil {
			return fmt.Errorf("publish trade %s: %w", trades[i].ID, err)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"instrument_uid": uid,
		"count":          len(trades),
	}).Info("streamed trades")
	return nil
}

func (s *Service) streamCandles(ctx context.Context, uid uuid.UUID, cfg config.StreamerConfig) error {
	candles, err := s.marketRepo.GetCandlesBetween(ctx, uid, cfg.From, cfg.To, cfg.CandleInterval)
	if err != nil {
		return fmt.Errorf("fetch candles: %w", err)
	}

	for i := range candles {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.publisher.PublishCandle(ctx, &candles[i]); err != nil {
			return fmt.Errorf("publish candle %s: %w", candles[i].ID, err)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"instrument_uid":   uid,
		"count":            len(candles),
		"interval_seconds": cfg.CandleInterval,
	}).Info("streamed candles")
	return nil
}

func (s *Service) streamOrderBooks(ctx context.Context, uid uuid.UUID, cfg config.StreamerConfig) error {
	snapshots, err := s.marketRepo.GetOrderBookSnapshotsBetween(ctx, uid, cfg.From, cfg.To, cfg.OrderDepth)
	if err != nil {
		return fmt.Errorf("fetch order books: %w", err)
	}

	for i := range snapshots {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.publisher.PublishOrderBook(ctx, &snapshots[i]); err != nil {
			return fmt.Errorf("publish order book %s: %w", snapshots[i].ID, err)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"instrument_uid": uid,
		"count":          len(snapshots),
		"depth":          cfg.OrderDepth,
	}).Info("streamed order books")
	return nil
}
