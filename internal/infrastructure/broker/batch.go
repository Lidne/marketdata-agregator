package broker

import (
	"context"
	"errors"
	"sync"
	"time"

	appmarketdata "main/internal/application/service/marketdata"
	domain "main/internal/domain/entity/marketdata"

	"github.com/sirupsen/logrus"
)

// BatchConfig controls batching thresholds for market data ingestion.
type BatchConfig struct {
	Size    int
	Timeout time.Duration
}

// BatchWriter buffers market data entities and flushes them via the service.
type BatchWriter struct {
	service *appmarketdata.Service

	trades     *batchBuffer[domain.Trade]
	candles    *batchBuffer[domain.Candle]
	orderBooks *batchBuffer[domain.OrderBookSnapshot]
}

// NewBatchWriter configures a batch writer for all market data entity types.
func NewBatchWriter(cfg BatchConfig, service *appmarketdata.Service, logger *logrus.Logger) *BatchWriter {
	componentLogger := logger.WithField("component", "batch_writer")
	return &BatchWriter{
		service: service,
		trades: newBatchBuffer(cfg, func(ctx context.Context, batch []domain.Trade) error {
			return service.AddTrades(ctx, batch)
		}, componentLogger.WithField("entity", "trade")),
		candles: newBatchBuffer(cfg, func(ctx context.Context, batch []domain.Candle) error {
			return service.AddCandles(ctx, batch)
		}, componentLogger.WithField("entity", "candle")),
		orderBooks: newBatchBuffer(cfg, func(ctx context.Context, batch []domain.OrderBookSnapshot) error {
			return service.AddOrderBookSnapshots(ctx, batch)
		}, componentLogger.WithField("entity", "orderbook")),
	}
}

// Run sets the base context for asynchronous flush operations.
func (b *BatchWriter) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	b.trades.setContext(ctx)
	b.candles.setContext(ctx)
	b.orderBooks.setContext(ctx)
}

// Stop flushes remaining buffers using the provided context.
func (b *BatchWriter) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	b.trades.setContext(ctx)
	b.candles.setContext(ctx)
	b.orderBooks.setContext(ctx)

	var errs []error
	if err := b.trades.drain(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := b.candles.drain(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := b.orderBooks.drain(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// AddTrade appends a trade to the trade buffer.
func (b *BatchWriter) AddTrade(trade *domain.Trade) error {
	if trade == nil {
		return errors.New("trade is nil")
	}
	copyTrade := *trade
	return b.trades.enqueue(copyTrade)
}

// AddCandle appends a candle to the candle buffer.
func (b *BatchWriter) AddCandle(candle *domain.Candle) error {
	if candle == nil {
		return errors.New("candle is nil")
	}
	copyCandle := *candle
	return b.candles.enqueue(copyCandle)
}

// AddOrderBook appends an order book snapshot to its buffer.
func (b *BatchWriter) AddOrderBook(snapshot *domain.OrderBookSnapshot) error {
	if snapshot == nil {
		return errors.New("order book snapshot is nil")
	}
	copySnapshot := *snapshot
	return b.orderBooks.enqueue(copySnapshot)
}

type batchBuffer[T any] struct {
	cfg     BatchConfig
	mu      sync.Mutex
	items   []T
	timer   *time.Timer
	flushFn func(context.Context, []T) error
	logger  *logrus.Entry
	ctx     context.Context
}

func newBatchBuffer[T any](cfg BatchConfig, flushFn func(context.Context, []T) error, logger *logrus.Entry) *batchBuffer[T] {
	return &batchBuffer[T]{
		cfg:     cfg,
		flushFn: flushFn,
		logger:  logger,
	}
}

func (bb *batchBuffer[T]) setContext(ctx context.Context) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	bb.ctx = ctx
}

func (bb *batchBuffer[T]) enqueue(item T) error {
	bb.mu.Lock()
	ctx := bb.ctx
	if ctx == nil {
		bb.mu.Unlock()
		return errors.New("batch buffer is not running")
	}
	if err := ctx.Err(); err != nil {
		bb.mu.Unlock()
		return err
	}
	bb.items = append(bb.items, item)
	var batch []T
	limit := bb.cfg.Size
	if limit <= 0 {
		limit = 1
	}
	if len(bb.items) >= limit {
		batch = bb.takeBatchLocked()
	} else if bb.timer == nil && bb.cfg.Timeout > 0 {
		bb.startTimerLocked()
	}
	bb.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}
	return bb.flushWithContext(ctx, batch)
}

func (bb *batchBuffer[T]) startTimerLocked() {
	timeout := bb.cfg.Timeout
	if timeout <= 0 {
		return
	}
	bb.timer = time.AfterFunc(timeout, func() {
		batch := bb.takeBatch()
		if len(batch) == 0 {
			return
		}
		if err := bb.flushWithCurrentContext(batch); err != nil && bb.logger != nil {
			bb.logger.WithError(err).Warn("batch flush failed")
		}
	})
}

func (bb *batchBuffer[T]) takeBatch() []T {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return bb.takeBatchLocked()
}

func (bb *batchBuffer[T]) takeBatchLocked() []T {
	if bb.timer != nil {
		bb.timer.Stop()
		bb.timer = nil
	}
	if len(bb.items) == 0 {
		return nil
	}
	batch := make([]T, len(bb.items))
	copy(batch, bb.items)
	bb.items = bb.items[:0]
	return batch
}

func (bb *batchBuffer[T]) flushWithCurrentContext(batch []T) error {
	bb.mu.Lock()
	ctx := bb.ctx
	bb.mu.Unlock()
	return bb.flushWithContext(ctx, batch)
}

func (bb *batchBuffer[T]) flushWithContext(ctx context.Context, batch []T) error {
	if len(batch) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	if err := bb.flushFn(ctx, batch); err != nil {
		return err
	}
	if bb.logger != nil {
		bb.logger.WithFields(logrus.Fields{
			"size":    len(batch),
			"took_ms": time.Since(start).Milliseconds(),
		}).Debug("flushed batch")
	}
	return nil
}

func (bb *batchBuffer[T]) drain(ctx context.Context) error {
	batch := bb.takeBatch()
	if len(batch) == 0 {
		return nil
	}
	return bb.flushWithContext(ctx, batch)
}
