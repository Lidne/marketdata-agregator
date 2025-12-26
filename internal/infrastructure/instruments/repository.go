package instruments

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInstrumentNotFound = errors.New("instrument not found")

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

func (r *Repository) CreateInstrument(ctx context.Context, instrument *domain.Instrument) error {
	if instrument == nil {
		return errors.New("instrument is nil")
	}
	if instrument.UID == uuid.Nil {
		instrument.UID = uuid.New()
	}
	now := time.Now().UTC()
	if instrument.CreatedAt.IsZero() {
		instrument.CreatedAt = now
	}
	instrument.UpdatedAt = now

	const query = `
		INSERT INTO instruments (uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, query,
		instrument.UID,
		instrument.Figi,
		instrument.Ticker,
		instrument.Lot,
		instrument.ClassCode,
		instrument.LogoURL,
		instrument.CreatedAt,
		instrument.UpdatedAt,
		instrument.DeletedAt,
	)

	return scanInstrumentInto(row, instrument)
}

func (r *Repository) GetInstrument(ctx context.Context, uid uuid.UUID) (*domain.Instrument, error) {
	const query = `
		SELECT uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at
		FROM instruments
		WHERE uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	instrument := &domain.Instrument{}
	if err := scanInstrumentInto(row, instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	return instrument, nil
}

func (r *Repository) UpdateInstrument(ctx context.Context, instrument *domain.Instrument) error {
	if instrument == nil {
		return errors.New("instrument is nil")
	}
	if instrument.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	instrument.UpdatedAt = time.Now().UTC()

	const query = `
		UPDATE instruments
		SET figi=$2,
			ticker=$3,
			lot=$4,
			class_code=$5,
			logo_url=$6,
			updated_at=$7,
			deleted_at=$8
		WHERE uid=$1
		RETURNING uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, query,
		instrument.UID,
		instrument.Figi,
		instrument.Ticker,
		instrument.Lot,
		instrument.ClassCode,
		instrument.LogoURL,
		instrument.UpdatedAt,
		instrument.DeletedAt,
	)

	if err := scanInstrumentInto(row, instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInstrumentNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) DeleteInstrument(ctx context.Context, uid uuid.UUID) error {
	const query = `DELETE FROM instruments WHERE uid=$1`
	cmdTag, err := r.pool.Exec(ctx, query, uid)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrInstrumentNotFound
	}
	return nil
}

func (r *Repository) GetShare(ctx context.Context, uid uuid.UUID) (*domain.Share, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at
		FROM instruments i
		INNER JOIN shares s ON s.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	share := &domain.Share{}
	if err := scanInstrumentInto(row, &share.Instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	return share, nil
}

func (r *Repository) GetBond(ctx context.Context, uid uuid.UUID) (*domain.Bond, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at,
		       b.nominal, b.aci_value
		FROM instruments i
		INNER JOIN bonds b ON b.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	bond := &domain.Bond{}
	var nominal, aciValue float64
	if err := scanInstrumentInto(row, &bond.Instrument, &nominal, &aciValue); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	bond.Nominal = nominal
	bond.AciValue = aciValue
	return bond, nil
}

func (r *Repository) GetFuture(ctx context.Context, uid uuid.UUID) (*domain.Future, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at,
		       f.min_price_increment, f.min_price_increment_amount, f.asset_type
		FROM instruments i
		INNER JOIN futures f ON f.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	future := &domain.Future{}
	var minIncrement, minIncrementAmount float64
	var assetTypeStr string
	if err := scanInstrumentInto(row, &future.Instrument, &minIncrement, &minIncrementAmount, &assetTypeStr); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	assetType, err := domain.NewAssetType(assetTypeStr)
	if err != nil {
		return nil, err
	}
	future.MinPriceIncrement = minIncrement
	future.MinPriceIncrementAmount = minIncrementAmount
	future.AssetType = assetType
	return future, nil
}

func (r *Repository) GetCurrency(ctx context.Context, uid uuid.UUID) (*domain.Currency, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at
		FROM instruments i
		INNER JOIN currencies c ON c.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	currency := &domain.Currency{}
	if err := scanInstrumentInto(row, &currency.Instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	return currency, nil
}

func (r *Repository) GetEtf(ctx context.Context, uid uuid.UUID) (*domain.Etf, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at,
		       e.min_price_increment
		FROM instruments i
		INNER JOIN etfs e ON e.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	etf := &domain.Etf{}
	var minIncrement float64
	if err := scanInstrumentInto(row, &etf.Instrument, &minIncrement); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	etf.MinPriceIncrement = minIncrement
	return etf, nil
}

func scanInstrumentInto(row pgx.Row, instrument *domain.Instrument, extras ...interface{}) error {
	var deletedAt *time.Time
	args := []interface{}{
		&instrument.UID,
		&instrument.Figi,
		&instrument.Ticker,
		&instrument.Lot,
		&instrument.ClassCode,
		&instrument.LogoURL,
		&instrument.CreatedAt,
		&instrument.UpdatedAt,
		&deletedAt,
	}
	args = append(args, extras...)

	if err := row.Scan(args...); err != nil {
		return err
	}
	instrument.DeletedAt = deletedAt
	return nil
}
