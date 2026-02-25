package interfaces

import (
	"context"

	marketdata "main/internal/domain/entity/marketdata"
)

// MarketDataPublisher publishes market data entities to an external message broker.
type MarketDataPublisher interface {
	PublishTrade(ctx context.Context, trade *marketdata.Trade) error
	PublishCandle(ctx context.Context, candle *marketdata.Candle) error
	PublishOrderBook(ctx context.Context, snapshot *marketdata.OrderBookSnapshot) error
	Close() error
}
