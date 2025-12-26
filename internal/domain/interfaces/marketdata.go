package interfaces

import (
	"context"
	"time"

	marketdata "main/internal/domain/entity/marketdata"

	"github.com/google/uuid"
)

type MarketDataRepository interface {
	AddTrade(ctx context.Context, trade *marketdata.Trade) error
	AddTrades(ctx context.Context, trades []marketdata.Trade) error
	GetTradesBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time) ([]marketdata.Trade, error)
	GetLastTrades(ctx context.Context, instrumentUID uuid.UUID, limit int) ([]marketdata.Trade, error)

	AddCandle(ctx context.Context, candle *marketdata.Candle) error
	AddCandles(ctx context.Context, candles []marketdata.Candle) error
	GetCandlesBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time, intervalSeconds int64) ([]marketdata.Candle, error)
	GetLastCandles(ctx context.Context, instrumentUID uuid.UUID, intervalSeconds int64, limit int) ([]marketdata.Candle, error)

	AddOrderBookSnapshot(ctx context.Context, snapshot *marketdata.OrderBookSnapshot) error
	AddOrderBookSnapshots(ctx context.Context, snapshots []marketdata.OrderBookSnapshot) error
	GetOrderBookSnapshotsBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time, depth int32) ([]marketdata.OrderBookSnapshot, error)
	GetLastOrderBookSnapshots(ctx context.Context, instrumentUID uuid.UUID, depth int32, limit int) ([]marketdata.OrderBookSnapshot, error)

	Close()
}
