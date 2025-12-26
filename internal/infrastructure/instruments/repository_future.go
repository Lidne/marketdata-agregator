package instruments

import (
	"context"
	"errors"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateFuture(ctx context.Context, future *domain.Future) error {
	if future == nil {
		return errors.New("future is nil")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := r.createInstrumentWith(ctx, tx, &future.Instrument); err != nil {
			return err
		}
		const query = `
			INSERT INTO futures (uid, min_price_increment, min_price_increment_amount, asset_type)
			VALUES ($1,$2,$3,$4)`
		if _, err := tx.Exec(ctx, query,
			future.UID,
			future.MinPriceIncrement,
			future.MinPriceIncrementAmount,
			future.AssetType.String(),
		); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) UpdateFuture(ctx context.Context, future *domain.Future) error {
	if future == nil {
		return errors.New("future is nil")
	}
	if future.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "futures", future.UID); err != nil {
			return err
		}
		if err := r.updateInstrumentWith(ctx, tx, &future.Instrument); err != nil {
			return err
		}
		const query = `
			UPDATE futures
			SET min_price_increment=$2,
				min_price_increment_amount=$3,
				asset_type=$4
			WHERE uid=$1`
		if cmdTag, err := tx.Exec(ctx, query,
			future.UID,
			future.MinPriceIncrement,
			future.MinPriceIncrementAmount,
			future.AssetType.String(),
		); err != nil {
			return err
		} else if cmdTag.RowsAffected() == 0 {
			return ErrInstrumentNotFound
		}
		return nil
	})
}

func (r *Repository) DeleteFuture(ctx context.Context, uid uuid.UUID) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "futures", uid); err != nil {
			return err
		}
		return r.deleteInstrumentWith(ctx, tx, uid)
	})
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
