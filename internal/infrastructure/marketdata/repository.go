package marketdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "main/internal/domain/entity/marketdata"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	if r == nil || r.pool == nil {
		return
	}
	r.pool.Close()
}

// Trades

const insertTradeQuery = `
	INSERT INTO trades (trade_id, instrument_uid, side, price, quantity_lots, traded_at, metadata)
	VALUES ($1,$2,$3,$4,$5,$6,$7)`

func (r *Repository) AddTrade(ctx context.Context, trade *domain.Trade) error {
	if trade == nil {
		return errors.New("nil trade")
	}
	if trade.ID == uuid.Nil {
		trade.ID = uuid.New()
	}
	meta, err := marshalJSON(trade.Metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, insertTradeQuery,
		trade.ID,
		trade.InstrumentUID,
		trade.Side,
		trade.Price,
		trade.QuantityLots,
		trade.TradedAt,
		meta,
	)
	return err
}

func (r *Repository) AddTrades(ctx context.Context, trades []domain.Trade) error {
	if len(trades) == 0 {
		return nil
	}
	rows := make([][]interface{}, 0, len(trades))
	for i := range trades {
		if trades[i].ID == uuid.Nil {
			trades[i].ID = uuid.New()
		}
		meta, err := marshalJSON(trades[i].Metadata)
		if err != nil {
			return err
		}
		rows = append(rows, []interface{}{
			trades[i].ID,
			trades[i].InstrumentUID,
			trades[i].Side,
			trades[i].Price,
			trades[i].QuantityLots,
			trades[i].TradedAt,
			meta,
		})
	}
	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"trades"},
		[]string{"trade_id", "instrument_uid", "side", "price", "quantity_lots", "traded_at", "metadata"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func (r *Repository) GetTradesBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time) ([]domain.Trade, error) {
	const query = `
		SELECT trade_id, instrument_uid, side, price, quantity_lots, traded_at, metadata
		FROM trades
		WHERE instrument_uid=$1 AND traded_at >= $2 AND traded_at <= $3
		ORDER BY traded_at ASC`
	rows, err := r.pool.Query(ctx, query, instrumentUID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		trade, err := scanTrade(rows)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, rows.Err()
}

func (r *Repository) GetLastTrades(ctx context.Context, instrumentUID uuid.UUID, limit int) ([]domain.Trade, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}
	const query = `
		SELECT trade_id, instrument_uid, side, price, quantity_lots, traded_at, metadata
		FROM trades
		WHERE instrument_uid=$1
		ORDER BY traded_at DESC
		LIMIT $2`
	rows, err := r.pool.Query(ctx, query, instrumentUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		trade, err := scanTrade(rows)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, rows.Err()
}

func scanTrade(row pgx.Row) (domain.Trade, error) {
	var metadataBytes []byte
	trade := domain.Trade{}
	err := row.Scan(
		&trade.ID,
		&trade.InstrumentUID,
		&trade.Side,
		&trade.Price,
		&trade.QuantityLots,
		&trade.TradedAt,
		&metadataBytes,
	)
	if err != nil {
		return domain.Trade{}, err
	}
	meta, err := unmarshalMetadata(metadataBytes)
	if err != nil {
		return domain.Trade{}, err
	}
	trade.Metadata = meta
	return trade, nil
}

// Candles

const insertCandleQuery = `
	INSERT INTO candles (
		candle_id, instrument_uid, interval_seconds, period_start,
		open, high, low, close,
		volume_lots, volume_buy_lots, volume_sell_lots,
		last_trade_at, metadata
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

func (r *Repository) AddCandle(ctx context.Context, candle *domain.Candle) error {
	if candle == nil {
		return errors.New("nil candle")
	}
	if candle.ID == uuid.Nil {
		candle.ID = uuid.New()
	}
	meta, err := marshalJSON(candle.Metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, insertCandleQuery,
		candle.ID,
		candle.InstrumentUID,
		candle.IntervalSeconds,
		candle.PeriodStart,
		candle.Open,
		candle.High,
		candle.Low,
		candle.Close,
		candle.VolumeLots,
		nullableInt64(candle.VolumeBuyLots),
		nullableInt64(candle.VolumeSellLots),
		candle.LastTradeAt,
		meta,
	)
	return err
}

func (r *Repository) AddCandles(ctx context.Context, candles []domain.Candle) error {
	if len(candles) == 0 {
		return nil
	}
	rows := make([][]interface{}, 0, len(candles))
	for i := range candles {
		if candles[i].ID == uuid.Nil {
			candles[i].ID = uuid.New()
		}
		meta, err := marshalJSON(candles[i].Metadata)
		if err != nil {
			return err
		}
		rows = append(rows, []interface{}{
			candles[i].ID,
			candles[i].InstrumentUID,
			candles[i].IntervalSeconds,
			candles[i].PeriodStart,
			candles[i].Open,
			candles[i].High,
			candles[i].Low,
			candles[i].Close,
			candles[i].VolumeLots,
			nullableInt64(candles[i].VolumeBuyLots),
			nullableInt64(candles[i].VolumeSellLots),
			candles[i].LastTradeAt,
			meta,
		})
	}
	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"candles"},
		[]string{
			"candle_id",
			"instrument_uid",
			"interval_seconds",
			"period_start",
			"open",
			"high",
			"low",
			"close",
			"volume_lots",
			"volume_buy_lots",
			"volume_sell_lots",
			"last_trade_at",
			"metadata",
		},
		pgx.CopyFromRows(rows),
	)
	return err
}

func (r *Repository) GetCandlesBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time, intervalSeconds int64) ([]domain.Candle, error) {
	const query = `
		SELECT candle_id, instrument_uid, interval_seconds, period_start,
		       open, high, low, close,
		       volume_lots, volume_buy_lots, volume_sell_lots,
		       last_trade_at, metadata
		FROM candles
		WHERE instrument_uid=$1
		  AND interval_seconds=$2
		  AND period_start >= $3
		  AND period_start <= $4
		ORDER BY period_start ASC`
	rows, err := r.pool.Query(ctx, query, instrumentUID, intervalSeconds, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candles []domain.Candle
	for rows.Next() {
		candle, err := scanCandle(rows)
		if err != nil {
			return nil, err
		}
		candles = append(candles, candle)
	}
	return candles, rows.Err()
}

func (r *Repository) GetLastCandles(ctx context.Context, instrumentUID uuid.UUID, intervalSeconds int64, limit int) ([]domain.Candle, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}
	const query = `
		SELECT candle_id, instrument_uid, interval_seconds, period_start,
		       open, high, low, close,
		       volume_lots, volume_buy_lots, volume_sell_lots,
		       last_trade_at, metadata
		FROM candles
		WHERE instrument_uid=$1 AND interval_seconds=$2
		ORDER BY period_start DESC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, query, instrumentUID, intervalSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candles []domain.Candle
	for rows.Next() {
		candle, err := scanCandle(rows)
		if err != nil {
			return nil, err
		}
		candles = append(candles, candle)
	}
	return candles, rows.Err()
}

func scanCandle(row pgx.Row) (domain.Candle, error) {
	var (
		volumeBuy  sql.NullInt64
		volumeSell sql.NullInt64
		lastTrade  sql.NullTime
		metadata   []byte
	)
	candle := domain.Candle{}
	err := row.Scan(
		&candle.ID,
		&candle.InstrumentUID,
		&candle.IntervalSeconds,
		&candle.PeriodStart,
		&candle.Open,
		&candle.High,
		&candle.Low,
		&candle.Close,
		&candle.VolumeLots,
		&volumeBuy,
		&volumeSell,
		&lastTrade,
		&metadata,
	)
	if err != nil {
		return domain.Candle{}, err
	}
	if volumeBuy.Valid {
		val := volumeBuy.Int64
		candle.VolumeBuyLots = &val
	}
	if volumeSell.Valid {
		val := volumeSell.Int64
		candle.VolumeSellLots = &val
	}
	if lastTrade.Valid {
		t := lastTrade.Time
		candle.LastTradeAt = &t
	}
	meta, err := unmarshalMetadata(metadata)
	if err != nil {
		return domain.Candle{}, err
	}
	candle.Metadata = meta
	return candle, nil
}

// Order book snapshots

const insertOrderBookQuery = `
	INSERT INTO order_book_snapshots (
		snapshot_id, instrument_uid, snapshot_at, depth, bids, asks, metadata
	) VALUES ($1,$2,$3,$4,$5,$6,$7)`

func (r *Repository) AddOrderBookSnapshot(ctx context.Context, snapshot *domain.OrderBookSnapshot) error {
	if snapshot == nil {
		return errors.New("nil order book snapshot")
	}
	if snapshot.ID == uuid.Nil {
		snapshot.ID = uuid.New()
	}
	bidsJSON, err := marshalJSON(snapshot.Bids)
	if err != nil {
		return err
	}
	asksJSON, err := marshalJSON(snapshot.Asks)
	if err != nil {
		return err
	}
	meta, err := marshalJSON(snapshot.Metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, insertOrderBookQuery,
		snapshot.ID,
		snapshot.InstrumentUID,
		snapshot.SnapshotAt,
		snapshot.Depth,
		bidsJSON,
		asksJSON,
		meta,
	)
	return err
}

func (r *Repository) AddOrderBookSnapshots(ctx context.Context, snapshots []domain.OrderBookSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	rows := make([][]interface{}, 0, len(snapshots))
	for i := range snapshots {
		if snapshots[i].ID == uuid.Nil {
			snapshots[i].ID = uuid.New()
		}
		bidsJSON, err := marshalJSON(snapshots[i].Bids)
		if err != nil {
			return err
		}
		asksJSON, err := marshalJSON(snapshots[i].Asks)
		if err != nil {
			return err
		}
		meta, err := marshalJSON(snapshots[i].Metadata)
		if err != nil {
			return err
		}
		rows = append(rows, []interface{}{
			snapshots[i].ID,
			snapshots[i].InstrumentUID,
			snapshots[i].SnapshotAt,
			snapshots[i].Depth,
			bidsJSON,
			asksJSON,
			meta,
		})
	}
	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"order_book_snapshots"},
		[]string{
			"snapshot_id",
			"instrument_uid",
			"snapshot_at",
			"depth",
			"bids",
			"asks",
			"metadata",
		},
		pgx.CopyFromRows(rows),
	)
	return err
}

func (r *Repository) GetOrderBookSnapshotsBetween(ctx context.Context, instrumentUID uuid.UUID, from, to time.Time, depth int32) ([]domain.OrderBookSnapshot, error) {
	const query = `
		SELECT snapshot_id, instrument_uid, snapshot_at, depth, bids, asks, metadata
		FROM order_book_snapshots
		WHERE instrument_uid=$1
		  AND depth=$2
		  AND snapshot_at >= $3
		  AND snapshot_at <= $4
		ORDER BY snapshot_at ASC`
	rows, err := r.pool.Query(ctx, query, instrumentUID, depth, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []domain.OrderBookSnapshot
	for rows.Next() {
		snapshot, err := scanOrderBook(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (r *Repository) GetLastOrderBookSnapshots(ctx context.Context, instrumentUID uuid.UUID, depth int32, limit int) ([]domain.OrderBookSnapshot, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}
	const query = `
		SELECT snapshot_id, instrument_uid, snapshot_at, depth, bids, asks, metadata
		FROM order_book_snapshots
		WHERE instrument_uid=$1 AND depth=$2
		ORDER BY snapshot_at DESC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, query, instrumentUID, depth, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []domain.OrderBookSnapshot
	for rows.Next() {
		snapshot, err := scanOrderBook(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func scanOrderBook(row pgx.Row) (domain.OrderBookSnapshot, error) {
	var (
		bidsJSON []byte
		asksJSON []byte
		metaJSON []byte
	)
	snapshot := domain.OrderBookSnapshot{}
	err := row.Scan(
		&snapshot.ID,
		&snapshot.InstrumentUID,
		&snapshot.SnapshotAt,
		&snapshot.Depth,
		&bidsJSON,
		&asksJSON,
		&metaJSON,
	)
	if err != nil {
		return domain.OrderBookSnapshot{}, err
	}
	if err := json.Unmarshal(bidsJSON, &snapshot.Bids); err != nil {
		return domain.OrderBookSnapshot{}, err
	}
	if err := json.Unmarshal(asksJSON, &snapshot.Asks); err != nil {
		return domain.OrderBookSnapshot{}, err
	}
	meta, err := unmarshalMetadata(metaJSON)
	if err != nil {
		return domain.OrderBookSnapshot{}, err
	}
	snapshot.Metadata = meta
	return snapshot, nil
}

// Helpers

func marshalJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func unmarshalMetadata(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func nullableInt64(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
